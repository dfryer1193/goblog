package service

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"

	"github.com/dfryer1193/goblog/models"
)

// File modes for written output. The site is public, so its files are
// world-readable.
const (
	outputDirMode  fs.FileMode = 0o755
	outputFileMode fs.FileMode = 0o644
)

// WriteSite writes every file of site under dir, creating dir if needed.
//
// dir must not exist yet or be empty, so that no stale files from an earlier
// build are left behind and nothing already there is overwritten.
func WriteSite(dir string, site *models.Site) error {
	if err := os.MkdirAll(dir, outputDirMode); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf("opening %s: %w", dir, err)
	}
	defer root.Close()

	if err = checkEmpty(root, dir); err != nil {
		return err
	}
	for _, name := range site.Paths() {
		if err = root.MkdirAll(path.Dir(name), outputDirMode); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
		if err = root.WriteFile(name, site.Files[name], outputFileMode); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
	}
	return nil
}

// checkEmpty returns an error unless root, opened from dir, is empty.
func checkEmpty(root *os.Root, dir string) error {
	f, err := root.Open(".")
	if err != nil {
		return fmt.Errorf("reading %s: %w", dir, err)
	}
	defer f.Close()
	_, err = f.ReadDir(1)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading %s: %w", dir, err)
	}
	return fmt.Errorf("%s is not empty; remove it or choose another output directory: %w", dir, fs.ErrExist)
}
