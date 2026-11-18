package service

import (
	"errors"
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

// err returns the bad references, all on one line, or nil if there are none.
func (r *referencing) err() error {
	if len(r.errs) == 0 {
		return nil
	}
	msgs := make([]string, len(r.errs))
	for i, err := range r.errs {
		msgs[i] = err.Error()
	}
	return errors.New(strings.Join(msgs, "; "))
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
	// images is the set of imageFiles in content, listed on first use.
	images map[string]bool
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
		var (
			dest *[]byte
			kind string
		)
		switch n := n.(type) {
		case *ast.Link:
			dest, kind = &n.Destination, "link"
		case *ast.Image:
			dest, kind = &n.Destination, "image"
		default:
			return ast.WalkContinue, nil
		}
		resolved, err := x.resolve(r.from, string(*dest), kind == "image")
		if err != nil {
			r.errs = append(r.errs, fmt.Errorf("%s %q %w", kind, *dest, err))
		} else {
			*dest = []byte(resolved)
		}
		return ast.WalkContinue, nil
	})
}

// resolve returns the site URL for the destination dest found in the Content
// Repository file from. Destinations that aren't relative paths, such as
// absolute URLs, come back unchanged. An image may only point into images/.
func (x *references) resolve(from, dest string, image bool) (string, error) {
	u, err := url.Parse(dest)
	if err != nil {
		return "", errors.New("is not a valid URL")
	}
	if u.Scheme != "" || u.Host != "" || u.Path == "" || strings.HasPrefix(u.Path, "/") {
		return dest, nil
	}
	target := path.Join(path.Dir(from), u.Path)
	if target == ".." || strings.HasPrefix(target, "../") {
		return "", errors.New("escapes the Content Repository")
	}

	var site *url.URL
	switch {
	case strings.HasPrefix(target, imagesDir+"/"):
		site = &url.URL{Path: "/" + target}
	case !image && path.Dir(target) == postsDir && path.Ext(target) == markdownExt:
		site = &url.URL{
			Path:     "/posts/" + strings.TrimSuffix(path.Base(target), markdownExt) + "/",
			Fragment: u.Fragment,
		}
	default:
		return "", fmt.Errorf("points at %s, which isn't on the site", target)
	}
	exists, err := x.exists(target)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", fmt.Errorf("points at %s, which is not found", target)
	}
	return site.String(), nil
}

// exists reports whether a reference can point at name: an image in
// images/, or another regular file. A symlink isn't one, so a reference can't
// reach outside the Content Repository through it.
func (x *references) exists(name string) (bool, error) {
	if !strings.HasPrefix(name, imagesDir+"/") {
		info, err := fs.Lstat(x.content, name)
		return err == nil && info.Mode().IsRegular(), nil
	}
	if x.images == nil {
		names, err := imageFiles(x.content)
		if err != nil {
			return false, err
		}
		x.images = make(map[string]bool, len(names))
		for _, n := range names {
			x.images[n] = true
		}
	}
	return x.images[name], nil
}
