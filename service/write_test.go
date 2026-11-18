package service_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
