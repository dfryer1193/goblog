package service_test

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dfryer1193/goblog/models"
	"github.com/dfryer1193/goblog/service"
)

func TestWriteSitePutsEveryOutputFileOnDisk(t *testing.T) {
	repo := contentRepo(map[string]string{
		"posts/hello.md": "---\ntitle: Hello\ndate: 2026-01-01\n---\nHi.\n",
	})
	site := build(t, repo, now, service.BuildOptions{})
	out := filepath.Join(t.TempDir(), "public")

	if err := service.WriteSite(out, site); err != nil {
		t.Fatalf("WriteSite: %v", err)
	}

	for _, name := range site.Paths() {
		got, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(name)))
		if err != nil {
			t.Fatalf("reading written %s: %v", name, err)
		}
		if string(got) != string(site.Files[name]) {
			t.Errorf("%s on disk differs from the built file", name)
		}
	}
}

func TestWriteSiteRefusesANonEmptyDirectory(t *testing.T) {
	site := build(t, contentRepo(nil), now, service.BuildOptions{})
	out := t.TempDir()
	if err := os.WriteFile(filepath.Join(out, "keep.txt"), []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := service.WriteSite(out, site)

	if err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("WriteSite into a non-empty directory: got %v, want a 'not empty' error", err)
	}
	if _, statErr := os.Stat(filepath.Join(out, "index.html")); statErr == nil {
		t.Error("WriteSite wrote into the non-empty directory anyway")
	}
}

func TestWriteSiteAddsGzipVariantsOfTextFiles(t *testing.T) {
	text := strings.Repeat("goblog renders text that compresses well. ", 40)
	site := &models.Site{Files: map[string][]byte{
		"index.html":            []byte("<!doctype html><p>" + text),
		"posts/x/index.html":    []byte("<p>" + text),
		"fonts/mono.css":        []byte("body{}" + text),
		"feed.xml":              []byte("<feed>" + text),
		"sitemap.xml":           []byte("<urlset>" + text),
		"images/diagram.svg":    []byte("<svg>" + text),
		"robots.txt":            []byte(text),
		"images/photo.png":      []byte(text),
		"images/photo.jpg":      []byte(text),
		"fonts/mono.woff2":      []byte(text),
		"posts/tiny/index.html": []byte("<p>tiny</p>"),
	}}
	out := filepath.Join(t.TempDir(), "public")

	if err := service.WriteSite(out, site); err != nil {
		t.Fatalf("WriteSite: %v", err)
	}

	for _, name := range []string{
		"index.html", "posts/x/index.html", "fonts/mono.css", "feed.xml", "sitemap.xml",
		"images/diagram.svg", "robots.txt",
	} {
		if got := gunzipFile(t, filepath.Join(out, filepath.FromSlash(name+".gz"))); got != string(site.Files[name]) {
			t.Errorf("%s.gz doesn't decompress to %s", name, name)
		}
	}
	for _, name := range []string{
		"images/photo.png", "images/photo.jpg", "fonts/mono.woff2", "posts/tiny/index.html",
	} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(name+".gz"))); err == nil {
			t.Errorf("WriteSite wrote %s.gz, want no gzip variant", name)
		}
	}
}

// gunzipFile returns the decompressed contents of the gzip file at name.
func gunzipFile(t *testing.T, name string) string {
	t.Helper()
	f, err := os.Open(name)
	if err != nil {
		t.Fatalf("opening gzip variant: %v", err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("decompressing %s: %v", name, err)
	}
	return string(data)
}
