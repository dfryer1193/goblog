package api

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/dfryer1193/mjolnir/health"
	"github.com/dfryer1193/mjolnir/router"

	"github.com/dfryer1193/goblog/service"
)

// indexFile is the file served for a directory's URL.
const indexFile = "index.html"

// notFoundFile is the site's 404 page, served for every path the site
// doesn't have.
const notFoundFile = "404.html"

// Site is the rendered site goblog serves.
type Site interface {
	// Ready reports whether a site has been rendered and switched in. It
	// backs /readyz, so it must be cheap.
	Ready(ctx context.Context) bool
	// Files are the site's files, or nil before the first render. Each call
	// may return a newer site.
	Files() fs.FS
}

// NewRouter builds goblog's HTTP handler: mjolnir's router, with its probes
// (/livez, /readyz and /startupz), the webhook endpoint at WebhookPath, and
// site's files.
func NewRouter(site Site, hook Webhook) (*router.Router, error) {
	r, err := router.New(router.Config{
		Health: health.Config{
			Started: health.AlwaysStarted,
			Ready:   site.Ready,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("building router: %w", err)
	}

	pages, err := renderFallbackPages()
	if err != nil {
		return nil, err
	}
	r.Handle(WebhookPath, webhookHandler{hook: hook})
	files := fileHandler{site: site, pages: pages}
	r.Handle("/*", files)
	return r, nil
}

// fileHandler serves the current site's files: a directory's URL serves its
// index.html, a directory's URL without the trailing slash redirects to it,
// and anything else is a 404, answered with the site's 404.html. It never
// lists directories. A file with a gzip
// variant (see service.WriteSite) is served gzipped to clients that accept
// gzip, and Cache-Control follows cacheControl.
type fileHandler struct {
	site  Site
	pages fallbackPages
}

func (h fileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	files := h.site.Files()
	if files == nil {
		w.Header().Set("Cache-Control", cacheNever)
		http.Error(w, "goblog hasn't rendered the site yet", http.StatusServiceUnavailable)
		return
	}

	urlPath := r.URL.Path
	name := strings.TrimPrefix(path.Clean("/"+urlPath), "/")
	if name == "" {
		name = "."
	}
	if strings.HasSuffix(urlPath, "/") {
		name = path.Join(name, indexFile)
	}
	// Until there's a file to serve, the answer (a redirect or a 404) may
	// change with the next render.
	w.Header().Set("Cache-Control", cacheShort)

	f, err := files.Open(name)
	if err != nil {
		h.notFound(w, r, files)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		h.notFound(w, r, files)
		return
	}
	if info.IsDir() {
		if _, err = fs.Stat(files, path.Join(name, indexFile)); err != nil {
			h.notFound(w, r, files)
			return
		}
		// The target is a directory of the site, so it can't point elsewhere.
		http.Redirect(w, r, "/"+name+"/", http.StatusMovedPermanently) //nolint:gosec // see above

		return
	}
	serveFile(w, r, files, name, f, info)
}

// serveFile serves the site file name, already open as f: its gzip variant
// if there is one and the client accepts gzip, else f itself.
func serveFile(w http.ResponseWriter, r *http.Request, files fs.FS, name string, f fs.File, info fs.FileInfo) {
	w.Header().Set("Cache-Control", cacheControl(name))
	gz, err := files.Open(name + service.GzipSuffix)
	if err == nil {
		defer gz.Close()
		w.Header().Add("Vary", "Accept-Encoding")
		// A Range is of the plain file's bytes, so a Range request gets the
		// plain file.
		if r.Header.Get("Range") == "" && acceptsGzip(r.Header.Values("Accept-Encoding")) {
			serveGzip(w, r, name, gz)
			return
		}
	}
	content, ok := f.(io.ReadSeeker)
	if !ok {
		serverError(w, "file isn't seekable")
		return
	}
	http.ServeContent(w, r, info.Name(), time.Time{}, content)
}

// notFound answers with the site's 404 page and status 404: gzipped if
// there's a variant and the client accepts gzip, like serveFile. If the site
// has no 404 page, or it can't be read, the built-in one answers instead.
func (h fileHandler) notFound(w http.ResponseWriter, r *http.Request, files fs.FS) {
	w.Header().Set("Cache-Control", cacheShort)
	gzName := notFoundFile + service.GzipSuffix
	_, err := fs.Stat(files, gzName)
	hasGzip := err == nil
	if hasGzip && acceptsGzip(r.Header.Values("Accept-Encoding")) {
		if gz, info, ok := openRegular(files, gzName); ok {
			defer gz.Close()
			w.Header().Add("Vary", "Accept-Encoding")
			w.Header().Set("Content-Encoding", "gzip")
			writeWhole(w, r, notFoundFile, gz, info.Size(), http.StatusNotFound)
			return
		}
	}
	f, info, ok := openRegular(files, notFoundFile)
	if !ok {
		h.pages.serveNotFound(w, r)
		return
	}
	defer f.Close()
	if hasGzip {
		w.Header().Add("Vary", "Accept-Encoding")
	}
	writeWhole(w, r, notFoundFile, f, info.Size(), http.StatusNotFound)
}

// openRegular opens the site file name if it's a regular file.
func openRegular(files fs.FS, name string) (fs.File, fs.FileInfo, bool) {
	f, err := files.Open(name)
	if err != nil {
		return nil, nil, false
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		_ = f.Close()
		return nil, nil, false
	}
	return f, info, true
}

// serveGzip serves gz, the gzip variant of the site file name, whole. It
// doesn't support Range requests.
func serveGzip(w http.ResponseWriter, r *http.Request, name string, gz fs.File) {
	w.Header().Set("Content-Encoding", "gzip")
	serveWhole(w, r, name, gz, http.StatusOK)
}

// serveWhole serves f, which holds the site file name's content (possibly
// gzipped), whole and with the given status.
func serveWhole(w http.ResponseWriter, r *http.Request, name string, f fs.File, status int) {
	info, err := f.Stat()
	if err != nil {
		w.Header().Del("Content-Encoding")
		serverError(w, "can't read the file")
		return
	}
	writeWhole(w, r, name, f, info.Size(), status)
}

// writeWhole writes f, size bytes of the site file name's content (possibly
// gzipped), with the given status.
func writeWhole(w http.ResponseWriter, r *http.Request, name string, f io.Reader, size int64, status int) {
	h := w.Header()
	contentType := mime.TypeByExtension(path.Ext(name))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	h.Set("Content-Type", contentType)
	h.Set("Content-Length", strconv.FormatInt(size, 10))
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = io.Copy(w, f)
	}
}

// serverError responds with a 500 that nothing caches.
func serverError(w http.ResponseWriter, text string) {
	w.Header().Del("Vary")
	w.Header().Set("Cache-Control", cacheNever)
	http.Error(w, text, http.StatusInternalServerError)
}
