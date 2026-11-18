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
)

// staticDir is the site section holding the stylesheet, fonts and other
// files that come with goblog rather than from the Content Repository.
const staticDir = "static"

// stylesheetSource is the stylesheet template in staticFS. {{asset "x"}} in
// it is the site path of staticFS's static/x.
const stylesheetSource = "static/style.css"

// staticFS holds the static assets: the stylesheet and the fonts with their
// licence.
//
//go:embed static
var staticFS embed.FS

// assets are the static files every site gets. Their names carry a hash of
// their contents, so a changed file gets a new URL and each URL can be cached
// for good.
type assets struct {
	// Stylesheet is the stylesheet's site path, e.g. /static/style.0123abcd.css.
	Stylesheet string
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
		paths[strings.TrimPrefix(name, staticDir+"/")] = a.add(path.Base(name), data)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("listing static files: %w", err)
	}

	src, err := staticFS.ReadFile(stylesheetSource)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", stylesheetSource, err)
	}
	tmpl, err := template.New(stylesheetSource).Option("missingkey=error").Funcs(template.FuncMap{
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
	a.Stylesheet = a.add(path.Base(stylesheetSource), []byte(css.String()))
	return a, nil
}

// add adds data as the static file called name, with a hash of data
// inserted before its extension, and returns its site path.
func (a *assets) add(name string, data []byte) string {
	sum := sha256.Sum256(data)
	ext := path.Ext(name)
	versioned := path.Join(staticDir, strings.TrimSuffix(name, ext)+"."+hex.EncodeToString(sum[:5])+ext)
	a.files[versioned] = data
	return "/" + versioned
}
