package service

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"path"
	"strings"

	"github.com/dfryer1193/goblog/models"
)

// layoutTemplate wraps every page and holds the partials pages share.
const layoutTemplate = "templates/layout.html"

// templateFS holds the page templates. Each templates/<kind>.html other than
// the layout defines the "content" of one kind of page, rendered with
// output.page(name, "<kind>", data).
//
//go:embed templates/*.html
var templateFS embed.FS

// pageData is what every page template gets. Each kind of page fills in the
// fields it needs; output.page fills in Site.
type pageData struct {
	// Site are the site settings: title, author, description, base URL and
	// navigation.
	Site *models.SiteSettings
	// Title is the page's own title. Empty means the site's title.
	Title string
	// Posts are the Posts a listing page shows, in display order.
	Posts []*models.Post
	// Post is the Post a Post page shows.
	Post *models.Post
}

// output collects a build's output files.
type output struct {
	// settings are passed to every page as pageData.Site.
	settings  *models.SiteSettings
	templates map[string]*template.Template
	files     map[string][]byte
}

func newOutput(settings *models.SiteSettings) (*output, error) {
	names, err := fs.Glob(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("listing templates: %w", err)
	}
	out := &output{settings: settings, templates: map[string]*template.Template{}, files: map[string][]byte{}}
	for _, name := range names {
		if name == layoutTemplate {
			continue
		}
		t, perr := template.ParseFS(templateFS, layoutTemplate, name)
		if perr != nil {
			return nil, fmt.Errorf("parsing %s: %w", name, perr)
		}
		out.templates[strings.TrimSuffix(path.Base(name), ".html")] = t
	}
	return out, nil
}

// page renders the kind template with data into the output file at name.
func (o *output) page(name, kind string, data pageData) error {
	t, ok := o.templates[kind]
	if !ok {
		return fmt.Errorf("no %s template", kind)
	}
	data.Site = o.settings
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, path.Base(layoutTemplate), data); err != nil {
		return fmt.Errorf("rendering %s: %w", name, err)
	}
	o.files[name] = buf.Bytes()
	return nil
}
