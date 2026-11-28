package stats

import (
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	maxminddb "github.com/oschwald/maxminddb-golang/v2"
	"github.com/prometheus/client_golang/prometheus"
)

// The placeholders in a database's URL template, replaced with the release's
// year and month.
const (
	placeholderYear  = "{YYYY}"
	placeholderMonth = "{MM}"
)

// databaseRetryInterval is how long after an attempt to get the current
// month's release goblog waits before trying again. Once it has the current
// month's release, it fetches nothing until the month changes.
const databaseRetryInterval = time.Hour

// databaseDownloadTimeout bounds one download.
const databaseDownloadTimeout = 10 * time.Minute

// maxDatabaseBytes is the largest database goblog writes, decompressed, so
// a bad download can't fill the work directory.
const maxDatabaseBytes = 1 << 30

// gzipMagic is how a gzip stream starts.
const gzipMagic = "\x1f\x8b"

// monthLayout formats a release's month, as in its file name.
const monthLayout = "2006-01"

// errNotReleased is a download's error when its month has no release (yet).
var errNotReleased = errors.New("not released")

// databaseConfig is what a database is built from.
type databaseConfig struct {
	// Name names the database, such as "city": in its files, its metrics'
	// database label and its logs.
	Name string
	// URLTemplate is the monthly release's URL, with {YYYY} and {MM} for
	// its year and month. It may be gzipped (as DB-IP's are) or not.
	URLTemplate string
	// Dir is the directory its copies are kept in.
	Dir string
	// Now is the clock that says which month's release is current.
	Now func() time.Time
	// Logger receives its warnings.
	Logger *slog.Logger
}

// database is a monthly-released MMDB, such as DB-IP's City Lite, kept up to
// date in a directory. It starts downloading the current month's release
// (or else last month's) in the background, and until the current month's
// has loaded, refresh tries again once databaseRetryInterval has passed,
// swapping the new copy in without a restart. It keeps
// the copy in use until a new one has loaded, then deletes the old one, so
// the directory never holds more than the copy in use and the one being
// fetched. Lookups are from the memory-mapped file.
type database struct {
	cfg    databaseConfig
	client *http.Client
	ctx    context.Context
	cancel context.CancelFunc
	// running counts the background downloads.
	running sync.WaitGroup

	// mu guards the loaded copy. Lookups hold it for reading while they
	// decode, so the copy isn't closed under them.
	mu sync.RWMutex
	// reader is the copy in use, or nil before one loads.
	reader *maxminddb.Reader
	// month is the loaded copy's release month, such as "2026-03".
	month string
	// file is the loaded copy's path.
	file string

	// attemptMu guards the attempts' schedule.
	attemptMu sync.Mutex
	// attempting is whether a download is running.
	attempting bool
	// attempted is when the last download started, or zero for never.
	attempted time.Time
	// closed is whether close has been called: nothing more is downloaded.
	closed bool
}

// newDatabase builds a database from cfg and starts its first download.
func newDatabase(cfg databaseConfig) *database {
	ctx, cancel := context.WithCancel(context.Background())
	d := &database{
		cfg:    cfg,
		client: &http.Client{Timeout: databaseDownloadTimeout},
		ctx:    ctx,
		cancel: cancel,
	}
	d.refresh()
	return d
}

// refresh starts fetching the current month's release in the background,
// unless it's loaded, a download is running or one was tried within
// databaseRetryInterval. It never waits.
func (d *database) refresh() {
	now := d.cfg.Now()
	if d.loadedMonth() == now.UTC().Format(monthLayout) {
		return
	}
	d.attemptMu.Lock()
	defer d.attemptMu.Unlock()
	if d.closed || d.attempting || (!d.attempted.IsZero() && now.Sub(d.attempted) < databaseRetryInterval &&
		!now.Before(d.attempted)) {
		return
	}
	d.attempting, d.attempted = true, now
	d.running.Go(func() {
		d.update(now)
		d.attemptMu.Lock()
		d.attempting = false
		d.attemptMu.Unlock()
	})
}

// update loads now's month's release, or, if that isn't out and the copy in
// use isn't last month's, last month's. On failure it keeps the copy in use
// and logs why.
func (d *database) update(now time.Time) {
	month := now.UTC().Format(monthLayout)
	firstOfMonth := time.Date(now.UTC().Year(), now.UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
	previous := firstOfMonth.AddDate(0, -1, 0).Format(monthLayout)
	loaded := d.loadedMonth()

	err := d.load(month)
	if errors.Is(err, errNotReleased) && loaded != previous {
		err = d.load(previous)
	}
	switch {
	case err == nil:
	case d.ctx.Err() != nil:
		// Closing.
	case loaded == "":
		d.cfg.Logger.Warn("Lines have no "+d.cfg.Name+" data: its database can't be downloaded",
			"database", d.cfg.Name, "error", err)
	case errors.Is(err, errNotReleased):
		d.cfg.Logger.Info("This month's "+d.cfg.Name+" database isn't out yet; still using "+loaded,
			"database", d.cfg.Name)
	default:
		d.cfg.Logger.Warn("This month's "+d.cfg.Name+" database can't be downloaded; still using "+loaded,
			"database", d.cfg.Name, "error", err)
	}
}

// load downloads month's release, opens and checks it, and swaps it in.
func (d *database) load(month string) error {
	file := filepath.Join(d.cfg.Dir, d.cfg.Name+"-"+month+".mmdb")
	if err := d.download(month, file); err != nil {
		return err
	}
	reader, err := maxminddb.Open(file)
	if err != nil {
		_ = os.Remove(file)
		return fmt.Errorf("opening the %s release: %w", month, err)
	}
	if err = reader.Verify(); err != nil {
		_ = reader.Close()
		_ = os.Remove(file)
		return fmt.Errorf("checking the %s release: %w", month, err)
	}
	d.mu.Lock()
	old := d.reader
	d.reader, d.month, d.file = reader, month, file
	if old != nil {
		_ = old.Close()
	}
	d.mu.Unlock()
	d.removeCopiesBut(file)
	d.cfg.Logger.Info("Loaded the "+d.cfg.Name+" database", "database", d.cfg.Name, "release", month)
	return nil
}

// download fetches month's release into file, decompressing it as it
// arrives if it's gzipped. It writes to a temporary file first, so file is
// only ever complete.
func (d *database) download(month, file string) error {
	year, mm, _ := strings.Cut(month, "-")
	url := strings.NewReplacer(placeholderYear, year, placeholderMonth, mm).Replace(d.cfg.URLTemplate)
	req, err := http.NewRequestWithContext(d.ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("building the request: %w", err)
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("downloading the %s release: %w", month, err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return fmt.Errorf("the %s release: %w", month, errNotReleased)
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("downloading the %s release: %s", month, resp.Status)
	}
	if err = os.MkdirAll(d.cfg.Dir, 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", d.cfg.Dir, err)
	}
	tmp, err := os.CreateTemp(d.cfg.Dir, d.cfg.Name+"-"+month+"-*.tmp")
	if err != nil {
		return fmt.Errorf("creating the %s release's file: %w", month, err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // gone already once renamed
	if err = decompress(tmp, resp.Body); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("downloading the %s release: %w", month, err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("writing the %s release: %w", month, err)
	}
	if err = os.Rename(tmp.Name(), file); err != nil {
		return fmt.Errorf("writing the %s release: %w", month, err)
	}
	return nil
}

// decompress copies body to w, gunzipping it if it's gzipped, and fails if
// it's longer than maxDatabaseBytes.
func decompress(w io.Writer, body io.Reader) error {
	br := bufio.NewReader(body)
	var r io.Reader = br
	if magic, _ := br.Peek(len(gzipMagic)); string(magic) == gzipMagic {
		zr, err := gzip.NewReader(br)
		if err != nil {
			return fmt.Errorf("gunzipping: %w", err)
		}
		r = zr
	}
	n, err := io.Copy(w, io.LimitReader(r, maxDatabaseBytes+1))
	if err != nil {
		return fmt.Errorf("copying: %w", err)
	}
	if n > maxDatabaseBytes {
		return fmt.Errorf("it's over %d bytes", maxDatabaseBytes)
	}
	return nil
}

// removeCopiesBut deletes the database's files in its directory, other than
// keep: old copies, and temporary files a crash left behind.
func (d *database) removeCopiesBut(keep string) {
	files, err := filepath.Glob(filepath.Join(d.cfg.Dir, d.cfg.Name+"-*"))
	if err != nil {
		return
	}
	for _, f := range files {
		if f != keep {
			if err = os.Remove(f); err != nil {
				d.cfg.Logger.Warn("Can't delete an old "+d.cfg.Name+" database", "file", f, "error", err)
			}
		}
	}
}

// loadedMonth is the loaded copy's release month, such as "2026-03", or ""
// if none is loaded.
func (d *database) loadedMonth() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.month
}

// name names the database, as in its metrics' database label.
func (d *database) name() string {
	return d.cfg.Name
}

// now is the time by the database's clock.
func (d *database) now() time.Time {
	return d.cfg.Now()
}

// lookup decodes ip's record into v, and reports whether there is one.
// Without a loaded copy, there isn't.
func (d *database) lookup(ip netip.Addr, v any) bool {
	if !ip.IsValid() {
		return false
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.reader == nil {
		return false
	}
	result := d.reader.Lookup(ip.Unmap())
	if !result.Found() {
		return false
	}
	return result.Decode(v) == nil
}

// close stops any download, waits for it and closes the copy in use. It
// leaves the copy's file, which a later process may replace.
func (d *database) close() {
	d.attemptMu.Lock()
	d.closed = true
	d.attemptMu.Unlock()
	d.cancel()
	d.running.Wait()
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.reader != nil {
		_ = d.reader.Close()
	}
	d.reader, d.month, d.file = nil, "", ""
}

// databaseCollector exposes a database's health: whether it's loaded, and
// how old the loaded release is.
type databaseCollector struct {
	db     *database
	loaded *prometheus.Desc
	age    *prometheus.Desc
}

// newDatabaseCollector is db's health metrics.
func newDatabaseCollector(db *database) *databaseCollector {
	labels := prometheus.Labels{"database": db.name()}
	return &databaseCollector{
		db: db,
		loaded: prometheus.NewDesc("goblog_stats_database_loaded",
			"Whether a copy of the database is loaded: 1 if so, 0 if lines go without its data.", nil, labels),
		age: prometheus.NewDesc("goblog_stats_database_age_seconds",
			"How old the loaded copy of the database is: the time since the start of its release's month.",
			nil, labels),
	}
}

// Describe sends the metrics' descriptions.
func (c *databaseCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.loaded
	ch <- c.age
}

// Collect sends whether the database is loaded, and its age if it is.
func (c *databaseCollector) Collect(ch chan<- prometheus.Metric) {
	month := c.db.loadedMonth()
	if month == "" {
		ch <- prometheus.MustNewConstMetric(c.loaded, prometheus.GaugeValue, 0)
		return
	}
	ch <- prometheus.MustNewConstMetric(c.loaded, prometheus.GaugeValue, 1)
	released, err := time.Parse(monthLayout, month)
	if err != nil {
		return
	}
	ch <- prometheus.MustNewConstMetric(c.age, prometheus.GaugeValue, c.db.now().Sub(released).Seconds())
}
