package service

import (
	"fmt"
	"io/fs"
	"net/url"
	"path"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// referencesKey holds a *referencing in a goldmark parser.Context: the
// Content Repository file being parsed and the bad references found in it.
var referencesKey = parser.NewContextKey() //nolint:gochecknoglobals // goldmark context keys are package-level

// referencing is the state of resolving one file's references.
type referencing struct {
	// from is the Content Repository path of the file being parsed.
	from string
	// errs are its references that can't be resolved.
	errs []error
}

// newReferencing returns a parser.Context for parsing the Content Repository
// file at from, and the state its references are resolved into.
func newReferencing(from string) (parser.Context, *referencing) {
	r := &referencing{from: from}
	pc := parser.NewContext()
	pc.Set(referencesKey, r)
	return pc, r
}

// references is a goldmark extension that rewrites relative links and images
// in the Content Repository into the site's URLs. Files are only parsed with
// it through a parser.Context from newReferencing.
type references struct {
	content fs.FS
}

// Extend adds the reference rewriting to m.
func (x *references) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithASTTransformers(util.Prioritized(x, 100))) //nolint:mnd // goldmark priority
}

// Transform rewrites every link and image destination in doc.
func (x *references) Transform(doc *ast.Document, _ text.Reader, pc parser.Context) {
	r, ok := pc.Get(referencesKey).(*referencing)
	if !ok {
		return
	}
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if link, isLink := n.(*ast.Link); isLink {
			dest, err := x.resolve(r.from, string(link.Destination))
			if err != nil {
				r.errs = append(r.errs, err)
			} else {
				link.Destination = []byte(dest)
			}
		}
		return ast.WalkContinue, nil
	})
}

// resolve returns the site URL for the destination dest found in the Content
// Repository file from. Destinations that aren't relative paths come back
// unchanged.
func (x *references) resolve(from, dest string) (string, error) {
	u, err := url.Parse(dest)
	if err != nil {
		return "", fmt.Errorf("link %q is not a valid URL", dest)
	}
	if u.Scheme != "" || u.Host != "" || u.Path == "" || strings.HasPrefix(u.Path, "/") {
		return dest, nil
	}
	target := path.Join(path.Dir(from), u.Path)
	if target == ".." || strings.HasPrefix(target, "../") {
		return "", fmt.Errorf("link %q escapes the Content Repository", dest)
	}
	if path.Dir(target) == postsDir && path.Ext(target) == ".md" {
		if !isFile(x.content, target) {
			return "", fmt.Errorf("link %q: %s not found", dest, target)
		}
		site := &url.URL{Path: "/posts/" + strings.TrimSuffix(path.Base(target), ".md") + "/", Fragment: u.Fragment}
		return site.String(), nil
	}
	return dest, nil
}

// isFile reports whether name is a regular file in content. A symlink isn't
// one, so a reference can't reach outside the Content Repository through it.
func isFile(content fs.FS, name string) bool {
	info, err := fs.Lstat(content, name)
	return err == nil && info.Mode().IsRegular()
}
