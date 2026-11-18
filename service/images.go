package service

import (
	"errors"
	"fmt"
	"io/fs"
)

// imagesDir is the Content Repository folder holding images, which may have
// subfolders. It's copied to the same path in the output.
const imagesDir = "images"

// copyImages copies every image in the Content Repository into the output
// unchanged, whether or not a Post references it.
func copyImages(b *siteBuild) error {
	err := fs.WalkDir(b.content, imagesDir, func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		data, err := fs.ReadFile(b.content, name)
		if err != nil {
			return err
		}
		b.out.files[name] = data
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("copying %s: %w", imagesDir, err)
	}
	return nil
}
