package service

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"maps"
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
// fields it needs; output.page fills in the ones every page shares.
type pageData struct {
	// Site are the site settings: title, author, description, base URL and
	// navigation. Filled in by output.page.
	Site *models.SiteSettings
	// Stylesheet is the stylesheet's site path. Filled in by output.page.
	Stylesheet string
	// Path is the page's own site path, e.g. /posts/x/. Filled in by
	// output.page.
	Path string
	// Tree is the tree pane, with this page's entry selected. Filled in by
	// output.page.
	Tree []treeLine
	// Windows are the status bar's windows. Filled in by output.page.
	Windows []window
	// Build is the build's date, time, host and commit. Filled in by
	// output.page.
	Build buildInfo

	// Title is the page's own title. Empty means the site's title.
	Title string
	// Label is the title label of the active pane, e.g. "picluster.md".
	Label string
	// WindowTitle is the page's #T in its status bar window, e.g.
	// "picluster.md" in the posts window.
	WindowTitle string
	// NotFound marks the 404 page, whose active pane is red.
	NotFound bool
	// Posts are the Posts a listing page shows, in display order.
	Posts []*models.Post
	// Post is the Post a Post page shows.
	Post *models.Post
	// Page is the Page a Page's page shows.
	Page *models.Page
	// Tags are the tags the tags page lists.
	Tags []*models.Tag
	// Tag is the tag a tag page shows; its Posts are also in Posts.
	Tag *models.Tag
}

// templateFuncs are the functions every template can call.
var templateFuncs = template.FuncMap{ //nolint:gochecknoglobals // fixed set
	// tagURL is the site path of the tag with the given name.
	"tagURL": models.TagURL,
}

// output collects a build's output files.
type output struct {
	// settings are passed to every page as pageData.Site.
	settings *models.SiteSettings
	// stylesheet is passed to every page as pageData.Stylesheet.
	stylesheet string
	// tree is the tree pane, with nothing selected.
	tree []treeLine
	// build is passed to every page as pageData.Build.
	build     buildInfo
	templates map[string]*template.Template
	files     map[string][]byte
}

func newOutput(settings *models.SiteSettings) (*output, error) {
	static, err := siteAssets()
	if err != nil {
		return nil, err
	}
	names, err := fs.Glob(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("listing templates: %w", err)
	}
	out := &output{
		settings: settings, stylesheet: static.Stylesheet,
		templates: map[string]*template.Template{}, files: maps.Clone(static.files),
	}
	for _, name := range names {
		if name == layoutTemplate {
			continue
		}
		t, perr := template.New(path.Base(layoutTemplate)).Funcs(templateFuncs).
			ParseFS(templateFS, layoutTemplate, name)
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
	data.Stylesheet = o.stylesheet
	data.Path = sitePath(name)
	data.Tree = treeFor(o.tree, data.Path)
	data.Windows = windowsFor(o.settings.Nav, &data)
	data.Build = o.build
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, path.Base(layoutTemplate), data); err != nil {
		return fmt.Errorf("rendering %s: %w", name, err)
	}
	o.files[name] = buf.Bytes()
	return nil
}
