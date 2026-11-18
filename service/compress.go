package service

import (
	"path"
	"strings"
)

// minGzipSize is the size below which a file isn't worth compressing: the
// gzip header and a request's overhead outweigh what it would save.
const minGzipSize = 256

// compressibleExtensions are the output file types that get a gzip variant:
// text formats. Images other than SVG and fonts (woff2 is compressed
// already) are left alone.
var compressibleExtensions = map[string]bool{ //nolint:gochecknoglobals // fixed set
	".html": true, ".htm": true, ".css": true, ".js": true, ".mjs": true, ".json": true, ".map": true,
	".xml": true, ".svg": true, ".txt": true, ".webmanifest": true,
}

// compressible reports whether the output file name is a text type that
// WriteSite writes a gzip variant of, if it's big enough.
func compressible(name string) bool {
	return compressibleExtensions[strings.ToLower(path.Ext(name))]
}
