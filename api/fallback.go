package api

import (
	"bytes"
	_ "embed" // for fallbackTemplate
	"fmt"
	"html/template"
	"net/http"
	"strconv"
)

// fallbackTemplate is goblog's built-in page: one self-contained HTML
// document (inline CSS, a system font stack, no JavaScript, no other
// requests) for when the rendered site can't answer.
//
//go:embed fallback.html
var fallbackTemplate string

// fallbackPage is what varies between the built-in pages.
type fallbackPage struct {
	Title    string
	Label    string
	Headline string
	Message  string
	// Error turns the pane red.
	Error bool
	// LinkHome adds a link to /.
	LinkHome bool
}

// fallbackPages are the built-in pages, rendered once.
type fallbackPages struct {
	notFound    []byte
	unavailable []byte
}

// renderFallbackPages renders the built-in 404 and 503 pages. They need
// nothing from the site, so they work before the first render.
func renderFallbackPages() (fallbackPages, error) {
	tmpl, err := template.New("fallback").Parse(fallbackTemplate)
	if err != nil {
		return fallbackPages{}, fmt.Errorf("parsing the built-in page: %w", err)
	}
	render := func(p fallbackPage) ([]byte, error) {
		var buf bytes.Buffer
		if execErr := tmpl.Execute(&buf, p); execErr != nil {
			return nil, fmt.Errorf("rendering the built-in %q page: %w", p.Label, execErr)
		}
		return buf.Bytes(), nil
	}
	var pages fallbackPages
	pages.notFound, err = render(fallbackPage{
		Title:    "not found",
		Label:    "[no name]",
		Headline: "E212: Can't open file for reading: no such file or directory",
		Message:  "Nothing lives at this address.",
		Error:    true,
		LinkHome: true,
	})
	if err != nil {
		return fallbackPages{}, err
	}
	pages.unavailable, err = render(fallbackPage{
		Title:    "starting",
		Label:    "[starting]",
		Headline: "Rendering the site",
		Message:  "The site is being rendered. Try again shortly.",
	})
	if err != nil {
		return fallbackPages{}, err
	}
	return pages, nil
}

// serveNotFound answers with the built-in 404 page. Like the site's own 404,
// it has the short cache lifetime.
func (p fallbackPages) serveNotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", cacheShort)
	serveFallback(w, r, http.StatusNotFound, p.notFound)
}

// serveFallback writes a built-in page, uncompressed: it has no gzip
// variant, so (as for any file without one) there's no Vary.
func serveFallback(w http.ResponseWriter, r *http.Request, status int, page []byte) {
	h := w.Header()
	h.Del("Content-Encoding")
	h.Del("Vary")
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Length", strconv.Itoa(len(page)))
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(page)
	}
}
