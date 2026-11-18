package models

import (
	"maps"
	"slices"
)

// Site is the result of one site build: every output file plus the Problems
// that made the build skip parts of the Content Repository.
type Site struct {
	// Files maps each output file's slash-separated path, relative to the
	// output root, to its contents, e.g. "index.html" or
	// "posts/picluster/index.html".
	Files map[string][]byte
	// Problems lists the files the build skipped, and why.
	Problems []Problem
}

// Paths returns the output files' paths in sorted order.
func (s *Site) Paths() []string {
	return slices.Sorted(maps.Keys(s.Files))
}
