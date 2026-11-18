package api

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/dfryer1193/mjolnir/health"
	"github.com/dfryer1193/mjolnir/router"

	"github.com/dfryer1193/goblog/models"
)

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
	files := fileHandler{site: site, pages: pages}
	r.Handle(WebhookPath, webhookHandler{hook: hook, files: files})
	r.Handle("/*", files)
	return r, nil
}

// fileHandler serves the current site's files: a directory's URL serves its
// index.html, a directory's URL without the trailing slash redirects to it,
// and anything else is a 404, answered with the site's 404.html (or the
// built-in 404 page if that can't be read). Before the first render, every
// request gets the built-in 503 page. It never lists directories. A file
// with a gzip variant (see service.WriteSite) is served gzipped to clients that accept
// gzip, and Cache-Control follows cacheControl.
type fileHandler struct {
	site  Site
	pages fallbackPages
}

func (h fileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Until the first render, every request gets the built-in "starting"
	// page, whatever its method.
	files := h.site.Files()
	if files == nil {
		h.pages.serveUnavailable(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	urlPath := r.URL.Path
	name := strings.TrimPrefix(path.Clean("/"+urlPath), "/")
	if name == "" {
		name = "."
	}
	if strings.HasSuffix(urlPath, "/") {
		name = path.Join(name, models.IndexFile)
	}
	// Until there's a file to serve, the answer (a redirect or a 404) may
	// change with the next render.
	w.Header().Set("Cache-Control", cacheShort)

	info, err := fs.Stat(files, name)
	switch {
	case err != nil:
		h.notFound(w, r, files)
	case info.IsDir():
		if _, err = fs.Stat(files, path.Join(name, models.IndexFile)); err != nil {
			h.notFound(w, r, files)
			return
		}
		// The target is a directory of the site, so it can't point elsewhere.
		http.Redirect(w, r, "/"+name+"/", http.StatusMovedPermanently) //nolint:gosec // see above
	default:
		w.Header().Set("Cache-Control", cacheControl(name))
		if !sendFile(w, r, files, name, http.StatusOK) {
			h.notFound(w, r, files)
		}
	}
}

// serveMissing answers a request for a path the site doesn't have: the
// site's 404 page, or the built-in "starting" page before the first render.
func (h fileHandler) serveMissing(w http.ResponseWriter, r *http.Request) {
	files := h.site.Files()
	if files == nil {
		h.pages.serveUnavailable(w, r)
		return
	}
	h.notFound(w, r, files)
}

// notFound answers with the site's 404 page and status 404, or, if the site
// has no 404 page or it can't be read, the built-in one.
func (h fileHandler) notFound(w http.ResponseWriter, r *http.Request, files fs.FS) {
	w.Header().Set("Cache-Control", cacheShort)
	if !sendFile(w, r, files, models.NotFoundFile, http.StatusNotFound) {
		h.pages.serveNotFound(w, r)
	}
}

// sendFile sends the site file name with status. If name has a gzip variant
// (see service.WriteSite), every response says it varies by
// Accept-Encoding, and the variant is sent, whole, to a client that accepts
// gzip, unless the request has a Range (which is of the plain file's
// bytes). Otherwise the plain file is sent; with status 200, through
// [http.ServeContent], so Range requests work. sendFile writes nothing and
// returns false if name isn't a regular file.
func sendFile(w http.ResponseWriter, r *http.Request, files fs.FS, name string, status int) bool {
	f, info, ok := openRegular(files, name)
	if !ok {
		return false
	}
	defer f.Close()
	h := w.Header()
	h.Set("Content-Type", contentType(name))

	if gz, gzInfo, hasGzip := openRegular(files, name+models.GzipSuffix); hasGzip {
		defer gz.Close()
		h.Add("Vary", "Accept-Encoding")
		if r.Header.Get("Range") == "" && acceptsGzip(r.Header.Values("Accept-Encoding")) {
			h.Set("Content-Encoding", "gzip")
			writeWhole(w, r, gz, gzInfo.Size(), status)
			return true
		}
	}
	if content, seekable := f.(io.ReadSeeker); seekable && status == http.StatusOK {
		http.ServeContent(w, r, info.Name(), time.Time{}, content)
		return true
	}
	writeWhole(w, r, f, info.Size(), status)
	return true
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

// writeWhole writes the size bytes of f with status, after the headers set
// so far. A HEAD request gets no body.
func writeWhole(w http.ResponseWriter, r *http.Request, f io.Reader, size int64, status int) {
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = io.Copy(w, f)
	}
}
