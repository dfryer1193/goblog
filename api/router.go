package api

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/dfryer1193/mjolnir/health"
	"github.com/dfryer1193/mjolnir/router"
)

// indexFile is the file served for a directory's URL.
const indexFile = "index.html"

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

	r.Handle(WebhookPath, webhookHandler{hook: hook})
	files := fileHandler{site: site}
	r.Handle("/*", files)
	return r, nil
}

// fileHandler serves the current site's files: a directory's URL serves its
// index.html, a directory's URL without the trailing slash redirects to it,
// and anything else is a 404. It never lists directories.
type fileHandler struct {
	site Site
}

func (h fileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	files := h.site.Files()
	if files == nil {
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

	f, err := files.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if info.IsDir() {
		if _, err = fs.Stat(files, path.Join(name, indexFile)); err != nil {
			http.NotFound(w, r)
			return
		}
		// The target is a directory of the site, so it can't point elsewhere.
		http.Redirect(w, r, "/"+name+"/", http.StatusMovedPermanently) //nolint:gosec // see above

		return
	}
	content, ok := f.(io.ReadSeeker)
	if !ok {
		http.Error(w, "file isn't seekable", http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, info.Name(), time.Time{}, content)
}
