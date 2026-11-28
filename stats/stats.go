// Package stats is goblog's Site Statistics: it sits around the handler
// serving the site's files, classifies each request it sees, and emits what
// it learns as JSON lines and Prometheus metrics. It never changes a
// response, and never delays one: serving a request only takes a snapshot of
// it and queues it, and a recorder in the background does the rest, dropping
// requests and lines (and counting them) rather than waiting. See ADR-0002
// and the README's Site Statistics section, which documents the lines and
// metrics as a contract.
package stats

//go:generate go tool mimir-generate

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
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
	// nothing is emitted: neither lines nor metrics.
	Enabled() bool
	// ExcludedNetworks are CIDRs, such as the author's own networks, whose
	// requests emit nothing at all. Entries that aren't CIDRs are skipped,
	// with a warning.
	ExcludedNetworks() []string
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
	// DatabaseDir is the directory the location and network databases are
	// downloaded into.
	DatabaseDir string
	// CityDatabaseURL is the City database's URL template, such as
	// DefaultCityDatabaseURL, with {YYYY} and {MM} for the release's year
	// and month. Empty means no location.
	CityDatabaseURL string
	// ASNDatabaseURL is the ASN database's URL template, such as
	// DefaultASNDatabaseURL, with {YYYY} and {MM} as in CityDatabaseURL.
	// Empty means no network.
	ASNDatabaseURL string
}

// Stats is Site Statistics, built by New.
type Stats struct {
	settings Settings
	excluded *excludedNetworks
	now      func() time.Time
	siteURL  func() string
	sources  sourceList
	lines    *emitter
	salt     *salt
	// city is the City database, or nil for no location.
	city *database
	// asn is the ASN database, or nil for no network.
	asn *database
	// hosting are the hosting networks' AS numbers.
	hosting hostingNetworks
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
	// subscribersAt is when the fetch arrived whose count feedSubscribers
	// holds, by feed reader. Only the recorder uses it.
	subscribersAt map[string]time.Time
	// visitorNotFound counts Visitor 404s.
	visitorNotFound prometheus.Counter
	// others counts the requests that are none of the other classes, by
	// kind of page.
	others *prometheus.CounterVec
	// dropped counts the requests and lines dropped because the recorder
	// or the output couldn't keep up.
	dropped prometheus.Counter

	// queue holds the requests waiting for the recorder.
	queue chan request
	// closed is whether Close has been called: nothing more is queued.
	closed atomic.Bool
	// closeOnce makes Close idempotent.
	closeOnce sync.Once
	// stop is closed to stop the recorder, once it has recorded what's
	// queued.
	stop chan struct{}
	// stopped is closed once the recorder has stopped.
	stopped chan struct{}
	// deadline is how long the recorder may take over what's queued once
	// stop is closed. It's set before stop is closed.
	deadline time.Time
}

// requestQueueLen is how many requests can wait for the recorder before new
// ones are dropped.
const requestQueueLen = 1024

// drainTimeout bounds how long Close waits for the queued requests and
// lines.
const drainTimeout = 2 * time.Second

// New builds Site Statistics from cfg.
func New(cfg Config) (*Stats, error) {
	sources, err := loadSourceList()
	if err != nil {
		return nil, fmt.Errorf("loading the search-engine and social list: %w", err)
	}
	s := &Stats{
		settings:      cfg.Settings,
		now:           cfg.Now,
		siteURL:       cfg.SiteURL,
		sources:       sources,
		subscribersAt: map[string]time.Time{},
	}
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
	if s.hosting, err = parsedHostingNetworks(); err != nil {
		return nil, fmt.Errorf("loading the hosting-network list: %w", err)
	}
	s.readers = newFeedReaders(crawlers)
	s.dropped = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "goblog_stats_dropped_total",
		Help: "Requests and lines dropped, uncounted or unwritten, because recording or the output couldn't keep up.",
	})
	s.lines = newEmitter(output, s.dropped)
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
	s.excluded = newExcludedNetworks(logger)
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
		s.visitorNotFound, s.others, s.dropped, saltLoaded)
	if cfg.CityDatabaseURL != "" {
		s.city = newDatabase(databaseConfig{
			Name:        cityDatabase,
			URLTemplate: cfg.CityDatabaseURL,
			Dir:         cfg.DatabaseDir,
			Now:         s.now,
			Logger:      logger,
		})
		s.registry.MustRegister(newDatabaseCollector(s.city))
	}
	if cfg.ASNDatabaseURL != "" {
		s.asn = newDatabase(databaseConfig{
			Name:        asnDatabase,
			URLTemplate: cfg.ASNDatabaseURL,
			Dir:         cfg.DatabaseDir,
			Now:         s.now,
			Logger:      logger,
		})
		s.registry.MustRegister(newDatabaseCollector(s.asn))
	}
	s.queue = make(chan request, requestQueueLen)
	s.stop, s.stopped = make(chan struct{}), make(chan struct{})
	go s.recordQueued()
	return s, nil
}

// MetricsHandler serves Site Statistics' Prometheus metrics. It belongs on
// its own port, never on the site's router.
func (s *Stats) MetricsHandler() http.Handler {
	return promhttp.HandlerFor(s.registry, promhttp.HandlerOpts{})
}

// Close records the queued requests and writes the queued lines, waiting
// up to drainTimeout for them, then stops Site Statistics' background work
// (the recorder, the output and the downloads) and waits for it. Requests
// served after Close aren't recorded.
func (s *Stats) Close() {
	s.closeOnce.Do(func() {
		s.closed.Store(true)
		deadline := time.Now().Add(drainTimeout)
		s.deadline = deadline
		close(s.stop)
		waitUntil(s.stopped, deadline)
		s.lines.close(deadline)
		s.eachDatabase((*database).close)
	})
}

// eachDatabase calls f with each database there is: the City database and
// the ASN database, unless they're off.
func (s *Stats) eachDatabase(f func(*database)) {
	for _, db := range []*database{s.city, s.asn} {
		if db != nil {
			f(db)
		}
	}
}

// Wrap returns next with Site Statistics around it: every request next
// serves is queued for the recorder, which classifies and emits it.
func (s *Stats) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.settings.Enabled() {
			next.ServeHTTP(w, r)
			return
		}
		req := snapshot(r, s.now().UTC())
		req.excludedNetworks = s.settings.ExcludedNetworks()
		ww := middleware.NewResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		req.status = ww.Status()
		req.kind = kindOf(req.path, req.status)
		s.enqueue(req)
	})
}

// enqueue queues req for the recorder, or drops it if the queue is full. It
// never waits.
func (s *Stats) enqueue(req request) {
	if s.closed.Load() {
		return
	}
	select {
	case s.queue <- req:
	default:
		s.dropped.Inc()
	}
}

// recordQueued is the recorder: it records the queued requests until stop
// is closed, then what's still queued, until the queue is empty or the
// deadline has passed.
func (s *Stats) recordQueued() {
	defer close(s.stopped)
	for {
		select {
		case req := <-s.queue:
			s.record(req)
		case <-s.stop:
			drain(s.queue, s.deadline, s.record)
			return
		}
	}
}

// record emits what Site Statistics learn from req. Each request is in one
// class, checked in order: Excluded first, ahead of every other class.
func (s *Stats) record(req request) {
	s.eachDatabase((*database).refresh)
	ip := req.clientIP()
	// Excluded: a request from an excluded network emits nothing at all,
	// neither a line nor a metric.
	if s.excluded.contains(req.excludedNetworks, ip) {
		return
	}
	if req.kind == kindFeed && s.recordFeedFetch(req) {
		return
	}
	if bot, ok := s.botName(req.userAgent); ok {
		s.botRequests.WithLabelValues(bot, statusClass(req.status), string(req.kind)).Inc()
		botNetwork := networkOf(s.asn, ip)
		l := req.baseLine(classBot)
		l.Bot, l.ASN, l.ASOrg = bot, botNetwork.asn, botNetwork.org
		s.emit(l)
		return
	}
	switch {
	case req.isPageView():
		s.pageViews.WithLabelValues(string(req.kind)).Inc()
		s.emit(s.visitorLine(req, ip, classPageView))
	case req.isVisitorNotFound():
		s.visitorNotFound.Inc()
		s.emit(s.visitorLine(req, ip, classNotFound))
	default:
		s.others.WithLabelValues(string(req.kind)).Inc()
	}
}

// visitorLine is req's line, of class c, for a Visitor's request from ip:
// it carries the Visitor fields, which Page Views and Visitor 404s share.
func (s *Stats) visitorLine(req request, ip netip.Addr, c class) line {
	loc := locationOf(s.city, ip)
	visitorNetwork := networkOf(s.asn, ip)
	f := familiesOf(s.userAgents, req.userAgent)
	l := req.baseLine(c)
	l.Browser, l.BrowserMajor, l.OS, l.Device = f.browser, f.browserMajor, f.os, f.device
	l.Country, l.Region = loc.country, loc.region
	// The network itself is never on a Visitor's line: only whether it's a
	// hosting network.
	if visitorNetwork.asn != 0 {
		hosting := s.hosting.contains(visitorNetwork.asn)
		l.Hosting = &hosting
	}
	if key := s.salt.current(); key != nil {
		l.VisitorID = visitorIDFor(key, ip, req.userAgent)
	}
	return l.withSource(s.sources.sourceOf(req, s.siteHost()))
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

// recordFeedFetch emits req, a request for the feed, as a feed fetch if a
// known feed reader sent it, and reports whether it did.
func (s *Stats) recordFeedFetch(req request) bool {
	reader, ok := s.readers.match(req.userAgent)
	if !ok {
		return false
	}
	s.feedFetches.WithLabelValues(reader.name).Inc()
	if reader.subscribers != nil {
		s.setSubscribers(reader.name, req.at, *reader.subscribers)
	}
	l := req.baseLine(classFeedFetch)
	l.Reader, l.Subscribers = reader.name, reader.subscribers
	s.emit(l)
	return true
}

// setSubscribers sets reader's subscriber count to n, reported by a fetch
// that arrived at at, unless a later fetch's count is already set: a file's
// response can be sent before its handler returns, so fetches can be queued
// out of order.
func (s *Stats) setSubscribers(reader string, at time.Time, n int) {
	if last, ok := s.subscribersAt[reader]; ok && !at.After(last) {
		return
	}
	s.subscribersAt[reader] = at
	s.feedSubscribers.WithLabelValues(reader).Set(float64(n))
}

// botName is the Bot name of a request with userAgent, if it's a Bot's.
func (s *Stats) botName(userAgent string) (string, bool) {
	if cr, ok := s.crawlers.match(userAgent); ok {
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

// Defaults are the StatsConfig's defaults: Site Statistics are on, and no
// networks are excluded.
func Defaults() Settings {
	return defaults{}
}

// Enabled is true by default.
func (defaults) Enabled() bool {
	return true
}

// ExcludedNetworks are empty by default.
func (defaults) ExcludedNetworks() []string {
	return nil
}
