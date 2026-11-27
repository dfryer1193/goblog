// Package stats is goblog's Site Statistics: it sits around the handler
// serving the site's files, classifies each request it sees, and emits what
// it learns as JSON lines and Prometheus metrics. It never changes a
// response, and never delays one. See ADR-0002 and the README's Site
// Statistics section, which documents the lines and metrics as a contract.
package stats

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/dfryer1193/mjolnir/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// The metrics' labels.
const (
	// labelKind is the kind of page.
	labelKind = "kind"
	// labelBot is the Bot name.
	labelBot = "bot"
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
}

// Stats is Site Statistics, built by New.
type Stats struct {
	settings Settings
	now      func() time.Time
	lines    *emitter
	crawlers *crawlers
	registry *prometheus.Registry
	// pageViews counts Page Views by kind of page.
	pageViews *prometheus.CounterVec
	// botRequests counts Bot requests by Bot name, status class and kind
	// of page.
	botRequests *prometheus.CounterVec
	// others counts the requests that are none of the other classes, by
	// kind of page.
	others *prometheus.CounterVec
}

// New builds Site Statistics from cfg.
func New(cfg Config) *Stats {
	s := &Stats{settings: cfg.Settings, now: cfg.Now}
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
		// The list is embedded, so this is goblog's bug, caught by any test.
		panic(err)
	}
	s.crawlers = crawlers
	s.lines = newEmitter(output)
	s.registry = prometheus.NewRegistry()
	s.pageViews = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "goblog_page_views_total",
		Help: "Page Views, by kind of page.",
	}, []string{labelKind})
	s.others = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "goblog_other_requests_total",
		Help: "Requests that are no other class (such as images, assets, the feed and redirects), by kind of page.",
	}, []string{labelKind})
	s.botRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "goblog_bot_requests_total",
		Help: "Bot requests, by Bot name (from the crawler list, or unknown), status class and kind of page.",
	}, []string{labelBot, labelStatusClass, labelKind})
	s.registry.MustRegister(s.pageViews, s.botRequests, s.others)
	return s
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
	if bot, ok := s.bot(r.UserAgent()); ok {
		s.botRequests.WithLabelValues(bot, statusClass(status), kind).Inc()
		s.emit(line{Time: s.now().UTC(), Class: classBot, Path: r.URL.Path, Status: status, Kind: kind, Bot: bot})
		return
	}
	if !isPageView(r, kind, status) {
		s.others.WithLabelValues(kind).Inc()
		return
	}
	s.pageViews.WithLabelValues(kind).Inc()
	s.emit(line{Time: s.now().UTC(), Class: classPageView, Path: r.URL.Path, Status: status, Kind: kind})
}

// bot is the Bot name of a request with userAgent, if it's a Bot's.
func (s *Stats) bot(userAgent string) (string, bool) {
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

// Defaults are the StatsConfig's defaults: Site Statistics are on.
func Defaults() Settings {
	return defaults{}
}

// Enabled is true by default.
func (defaults) Enabled() bool {
	return true
}
