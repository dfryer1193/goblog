package service

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"

	"github.com/dfryer1193/goblog/models"
)

// templateFS holds the page templates. layout.html wraps every page; each
// other file defines the "content" of one kind of page.
//
//go:embed templates/*.html
var templateFS embed.FS

// pageKinds are the page templates, by name. Each is templates/<name>.html
// parsed together with templates/layout.html.
var pageKinds = []string{"index", "post"} //nolint:gochecknoglobals // fixed list of embedded templates

// pageData is what every page template gets. Each kind of page fills in the
// fields it needs.
type pageData struct {
	// Title is the page's own title. Empty means the site's title.
	Title string
	// Posts are the Posts a listing page shows, in display order.
	Posts []*models.Post
	// Post is the Post a Post page shows.
	Post *models.Post
}

// output collects a build's output files.
type output struct {
	templates map[string]*template.Template
	files     map[string][]byte
}

func newOutput() (*output, error) {
	out := &output{templates: map[string]*template.Template{}, files: map[string][]byte{}}
	for _, kind := range pageKinds {
		t, err := template.ParseFS(templateFS, "templates/layout.html", "templates/"+kind+".html")
		if err != nil {
			return nil, fmt.Errorf("parsing %s template: %w", kind, err)
		}
		out.templates[kind] = t
	}
	return out, nil
}

// page renders the kind template with data into the output file at name.
func (o *output) page(name, kind string, data pageData) error {
	t, ok := o.templates[kind]
	if !ok {
		return fmt.Errorf("no %s template", kind)
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout.html", data); err != nil {
		return fmt.Errorf("rendering %s: %w", name, err)
	}
	o.files[name] = buf.Bytes()
	return nil
}
