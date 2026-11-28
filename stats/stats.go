// Package stats is goblog's Site Statistics: it sits around the handler
// serving the site's files, classifies each request it sees, and emits what
// it learns as JSON lines and Prometheus metrics. It never changes a
// response, and never delays one. See ADR-0002 and the README's Site
// Statistics section, which documents the lines and metrics as a contract.
package stats

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/dfryer1193/mjolnir/middleware"
	useragent "github.com/medama-io/go-useragent"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// The metrics' labels.
const (
	// labelKind is the kind of page.
	labelKind = "kind"
	// labelBot is the Bot name.
	labelBot = "bot"
	// labelReader is the feed reader's name.
	labelReader = "reader"
	// labelStatusClass is the class of the response's status, such as 2xx.
	labelStatusClass = "status_class"
)

// Settings is the StatsConfig: the settings Site Statistics read on every
// request, so changes apply live.
type Settings interface {
	// Enabled reports whether Site Statistics are on. While they're off,
	// nothing is emitted.
	Enabled() bool
}

// Config is what Site Statistics are built from.
type Config struct {
	// Settings are the StatsConfig. Nil means the defaults.
	Settings Settings
	// Now is the clock. Nil means time.Now.
	Now func() time.Time
	// Output receives the JSON lines. Nil means os.Stdout.
	Output io.Writer
	// SaltFile is the path of the daily salt for Visitor IDs. Empty means
	// there's no salt, and so no Visitor IDs.
	SaltFile string
	// Logger receives Site Statistics' warnings. Nil means slog's default.
	Logger *slog.Logger
	// SiteURL is the current site's base URL, from site.yaml's url, or ""
	// if it has none. A referrer from its host is internal navigation.
	// Without a url, or with a nil SiteURL, the host a request was sent to
	// is the site's host.
	SiteURL func() string
}

// Stats is Site Statistics, built by New.
type Stats struct {
	settings Settings
	now      func() time.Time
	siteURL  func() string
	sources  sourceList
	lines    *emitter
	salt     *salt
	// userAgents parses User-Agents into families.
	userAgents *useragent.Parser
	// crawlers recognises Bots by the crawler list.
	crawlers *crawlers
	// readers recognises feed readers.
	readers  *feedReaders
	registry *prometheus.Registry
	// pageViews counts Page Views by kind of page.
	pageViews *prometheus.CounterVec
	// botRequests counts Bot requests by Bot name, status class and kind
	// of page.
	botRequests *prometheus.CounterVec
	// feedFetches counts feed fetches by feed reader.
	feedFetches *prometheus.CounterVec
	// feedSubscribers is the latest subscriber count each feed reader
	// reported.
	feedSubscribers *prometheus.GaugeVec
	// visitorNotFound counts Visitor 404s.
	visitorNotFound prometheus.Counter
	// others counts the requests that are none of the other classes, by
	// kind of page.
	others *prometheus.CounterVec
}

// New builds Site Statistics from cfg.
func New(cfg Config) (*Stats, error) {
	sources, err := loadSourceList()
	if err != nil {
		return nil, fmt.Errorf("loading the search-engine and social list: %w", err)
	}
	s := &Stats{settings: cfg.Settings, now: cfg.Now, siteURL: cfg.SiteURL, sources: sources}
	if s.settings == nil {
		s.settings = Defaults()
	}
	if s.now == nil {
		s.now = time.Now
	}
	output := cfg.Output
	if output == nil {
		output = os.Stdout
	}
	crawlers, err := parsedCrawlers()
	if err != nil {
		return nil, fmt.Errorf("loading the crawler list: %w", err)
	}
	s.crawlers = crawlers
	s.readers = newFeedReaders(crawlers)
	s.lines = newEmitter(output)
	// Built here, in serve mode only: it takes a few milliseconds and MiB.
	s.userAgents = useragent.NewParser()
	s.registry = prometheus.NewRegistry()
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	saltLoaded := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "goblog_stats_salt_loaded",
		Help: "Whether a salt for Visitor IDs is loaded: 1 if so, 0 if Page Views have no Visitor ID.",
	})
	s.salt = newSalt(cfg.SaltFile, s.now, logger, saltLoaded)
	s.pageViews = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "goblog_page_views_total",
		Help: "Page Views, by kind of page.",
	}, []string{labelKind})
	s.visitorNotFound = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "goblog_visitor_not_found_total",
		Help: "Visitor 404s: Visitors' GETs, not prefetches, answered with the 404 page.",
	})
	s.others = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "goblog_other_requests_total",
		Help: "Requests that are no other class (such as images, assets, the feed and redirects), by kind of page.",
	}, []string{labelKind})
	s.botRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "goblog_bot_requests_total",
		Help: "Bot requests, by Bot name (from the crawler list, or unknown), status class and kind of page.",
	}, []string{labelBot, labelStatusClass, labelKind})
	s.feedFetches = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "goblog_feed_fetches_total",
		Help: "Feed fetches, by feed reader.",
	}, []string{labelReader})
	s.feedSubscribers = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "goblog_feed_subscribers",
		Help: "The subscriber count each feed reader last reported in its User-Agent.",
	}, []string{labelReader})
	s.registry.MustRegister(s.pageViews, s.botRequests, s.feedFetches, s.feedSubscribers,
		s.visitorNotFound, s.others, saltLoaded)
	return s, nil
}

// MetricsHandler serves Site Statistics' Prometheus metrics. It belongs on
// its own port, never on the site's router.
func (s *Stats) MetricsHandler() http.Handler {
	return promhttp.HandlerFor(s.registry, promhttp.HandlerOpts{})
}

// Wrap returns next with Site Statistics around it: every request next
// serves is classified and emitted.
func (s *Stats) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.settings.Enabled() {
			next.ServeHTTP(w, r)
			return
		}
		ww := middleware.NewResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		s.record(r, ww.Status())
	})
}

// record emits what Site Statistics learn from a request answered with
// status.
func (s *Stats) record(r *http.Request, status int) {
	kind := kindOf(r.URL.Path, status)
	// Excluded requests (from excluded_networks) are to be checked here,
	// before every other class.
	if kind == kindFeed && s.recordFeedFetch(r, status) {
		return
	}
	if bot, ok := s.bot(r.UserAgent()); ok {
		s.botRequests.WithLabelValues(bot, statusClass(status), kind).Inc()
		s.emit(line{Time: s.now().UTC(), Class: classBot, Path: r.URL.Path, Status: status, Kind: kind, Bot: bot})
		return
	}
	switch {
	case isPageView(r, kind, status):
		s.pageViews.WithLabelValues(kind).Inc()
		s.emit(s.visitorLine(r, classPageView, kind, status))
	case isVisitorNotFound(r, kind):
		s.visitorNotFound.Inc()
		s.emit(s.visitorLine(r, classNotFound, kind, status))
	default:
		s.others.WithLabelValues(kind).Inc()
	}
}

// visitorLine is the line of class for a Visitor's request r, for a page of
// kind answered with status: it carries the Visitor fields, which Page Views
// and Visitor 404s share.
func (s *Stats) visitorLine(r *http.Request, class, kind string, status int) line {
	f := familiesOf(s.userAgents, r.UserAgent())
	l := line{
		Time:         s.now().UTC(),
		Class:        class,
		Path:         r.URL.Path,
		Status:       status,
		Kind:         kind,
		Browser:      f.browser,
		BrowserMajor: f.browserMajor,
		OS:           f.os,
		Device:       f.device,
	}
	if key := s.salt.current(); key != nil {
		l.VisitorID = visitorIDFor(key, clientIP(r), r.UserAgent())
	}
	return l.withSource(s.sources.sourceOf(r, s.siteHost()))
}

// siteHost is the current site's host, from its url, or "" if it has none.
func (s *Stats) siteHost() string {
	if s.siteURL == nil {
		return ""
	}
	u, err := url.Parse(s.siteURL())
	if err != nil {
		return ""
	}
	return hostOf(u.Host)
}

// recordFeedFetch emits r, a request for the feed answered with status, as
// a feed fetch if a known feed reader sent it, and reports whether it did.
func (s *Stats) recordFeedFetch(r *http.Request, status int) bool {
	reader, ok := s.readers.match(r.UserAgent())
	if !ok {
		return false
	}
	s.feedFetches.WithLabelValues(reader.name).Inc()
	if reader.subscribers != nil {
		s.feedSubscribers.WithLabelValues(reader.name).Set(float64(*reader.subscribers))
	}
	s.emit(line{
		Time: s.now().UTC(), Class: classFeedFetch, Path: r.URL.Path, Status: status, Kind: kindFeed,
		Reader: reader.name, Subscribers: reader.subscribers,
	})
	return true
}

// bot is the Bot name of a request with userAgent, if it's a Bot's.
func (s *Stats) bot(userAgent string) (string, bool) {
	if cr, ok := s.crawlers.match(userAgent, false); ok {
		return cr.name, true
	}
	if botByHeuristics(userAgent) {
		return botUnknown, true
	}
	return "", false
}

// statusClass is the class of status, such as "2xx" for 200.
func statusClass(status int) string {
	const perClass = 100
	return strconv.Itoa(status/perClass) + "xx"
}

// emit writes l as one JSON line.
func (s *Stats) emit(l line) {
	l.Stream = streamMarker
	b, err := json.Marshal(l)
	if err != nil {
		return
	}
	s.lines.emit(append(b, '\n'))
}

// defaults are the StatsConfig's defaults.
type defaults struct{}

// Defaults are the StatsConfig's defaults: Site Statistics are on.
func Defaults() Settings {
	return defaults{}
}

// Enabled is true by default.
func (defaults) Enabled() bool {
	return true
}
