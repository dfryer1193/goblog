package service

import (
	"errors"
	"fmt"
	"io/fs"
)

// imagesDir is the Content Repository folder holding images, which may have
// subfolders. It's copied to the same path in the output.
const imagesDir = "images"

// imageFiles lists the images in content: every regular file under images/.
// Symlinks, whether to files or folders, aren't followed, so nothing outside
// the Content Repository can become an image.
func imageFiles(content fs.FS) ([]string, error) {
	info, err := fs.Lstat(content, imagesDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", imagesDir, err)
	}
	if !info.IsDir() {
		return nil, nil
	}
	var names []string
	err = fs.WalkDir(content, imagesDir, func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			names = append(names, name)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", imagesDir, err)
	}
	return names, nil
}

// copyImages copies every image in the Content Repository into the output
// unchanged, whether or not a Post references it.
func copyImages(b *siteBuild) error {
	names, err := imageFiles(b.content)
	if err != nil {
		return err
	}
	for _, name := range names {
		data, rerr := fs.ReadFile(b.content, name)
		if rerr != nil {
			return fmt.Errorf("copying %s: %w", name, rerr)
		}
		b.out.files[name] = data
	}
	return nil
}
