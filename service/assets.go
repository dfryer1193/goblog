package service

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"sync"
	"text/template"

	"github.com/dfryer1193/goblog/models"
)

// staticDir is the folder of staticFS holding the files that come with
// goblog rather than from the Content Repository. Its subfolders are site
// sections: static/fonts/x.woff2 is served under /fonts/ (FontsSection).
// Files in static/licenses (LicensesSection) are the only ones that keep
// their names; every other one gets a content hash in its name.
const staticDir = "static"

// treeScriptSource is the tree pane's script in staticFS: progressive
// enhancement that remembers folded folders across pages. It's served under
// /assets/ like every other versioned static file.
const treeScriptSource = "static/assets/tree.js"

// stylesheetSource is the stylesheet template in staticFS. {{asset "x"}} in
// it is the site path of staticFS's static/x, and {{palette}} is
// PaletteCSS. It's served under /assets/.
const stylesheetSource = "static/style.css"

// staticFS holds the static assets: the stylesheet, the tree script and the
// fonts, with their licence.
//
//go:embed static
var staticFS embed.FS

// assets are the static files every site gets. Apart from the licences,
// their names carry a hash of their contents, so a changed file gets a new
// URL, and the server can let clients cache /assets/ and /fonts/ for good.
type assets struct {
	// Stylesheet is the stylesheet's site path, e.g. /assets/style.0123abcd45.css.
	Stylesheet string
	// TreeScript is the tree script's site path, e.g. /assets/tree.0123abcd45.js.
	TreeScript string
	// files are the output files, by output path.
	files map[string][]byte
}

// siteAssets returns the static assets. They depend only on the binary, so
// they're made once.
var siteAssets = sync.OnceValues(makeAssets) //nolint:gochecknoglobals // computed once from embedded files

// makeAssets versions every static file except the stylesheet template, then
// renders the stylesheet with their versioned paths and the code highlighting
// classes, and versions that too.
func makeAssets() (*assets, error) {
	a := &assets{files: map[string][]byte{}}
	paths := map[string]string{}
	err := fs.WalkDir(staticFS, staticDir, func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || name == stylesheetSource {
			return err
		}
		data, rerr := staticFS.ReadFile(name)
		if rerr != nil {
			return fmt.Errorf("reading %s: %w", name, rerr)
		}
		rel := strings.TrimPrefix(name, staticDir+"/")
		if path.Dir(rel) == models.LicensesSection {
			a.files[rel] = data
			paths[rel] = "/" + rel
			return nil
		}
		paths[rel] = a.add(rel, data)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("listing static files: %w", err)
	}
	a.TreeScript = paths[strings.TrimPrefix(treeScriptSource, staticDir+"/")]

	src, err := staticFS.ReadFile(stylesheetSource)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", stylesheetSource, err)
	}
	tmpl, err := template.New(stylesheetSource).Option("missingkey=error").Funcs(template.FuncMap{
		"palette": PaletteCSS,
		"asset": func(name string) (string, error) {
			p, ok := paths[name]
			if !ok {
				return "", fmt.Errorf("no static file %s", name)
			}
			return p, nil
		},
	}).Parse(string(src))
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", stylesheetSource, err)
	}
	var css strings.Builder
	if err = tmpl.Execute(&css, nil); err != nil {
		return nil, fmt.Errorf("rendering %s: %w", stylesheetSource, err)
	}
	css.WriteString("\n/* Highlighted code. */\n")
	if err = writeCodeCSS(&css); err != nil {
		return nil, err
	}
	a.Stylesheet = a.add(path.Join(models.AssetsSection, path.Base(stylesheetSource)), []byte(css.String()))
	return a, nil
}

// add adds data as the output file name, with a hash of data inserted before
// its extension, and returns its site path.
func (a *assets) add(name string, data []byte) string {
	sum := sha256.Sum256(data)
	ext := path.Ext(name)
	versioned := strings.TrimSuffix(name, ext) + "." + hex.EncodeToString(sum[:5]) + ext
	a.files[versioned] = data
	return "/" + versioned
}
