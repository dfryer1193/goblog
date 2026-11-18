package models

import (
	"maps"
	"slices"
	"time"
)

// Site is the result of one site build: every output file, plus the
// Problems that made the build skip parts of the Content Repository and the
// Warnings about parts it worked around.
type Site struct {
	// Files maps each output file's slash-separated path, relative to the
	// output root, to its contents, e.g. "index.html" or
	// "posts/picluster/index.html".
	Files map[string][]byte
	// Problems lists the files the build skipped, and why.
	Problems []Problem
	// Warnings lists what the build worked around in files it didn't
	// skip.
	Warnings []Warning
	// Timezone is the timezone the build decided "today" in, which is
	// when a Post's Publish Date has arrived.
	Timezone *time.Location
	// Today is the build's date in Timezone. Rendering again on a later
	// date may publish scheduled Posts.
	Today Date
}

// Paths returns the output files' paths in sorted order.
func (s *Site) Paths() []string {
	return slices.Sorted(maps.Keys(s.Files))
}
