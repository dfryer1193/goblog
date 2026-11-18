package api

import "strings"

// Cache-Control values goblog sends.
const (
	// cacheImmutable is for files whose names change whenever their content
	// does, so a cached copy never goes stale.
	cacheImmutable = "public, max-age=31536000, immutable"
	// cacheImages is for the Content Repository's images, which keep their
	// names when the author replaces them.
	cacheImages = "public, max-age=86400"
	// cacheShort is for everything that changes when content does: HTML, the
	// feed, the sitemap, redirects and 404s.
	cacheShort = "public, max-age=60"
	// cacheNever is for responses that say nothing about the site, such as
	// "not rendered yet".
	cacheNever = "no-store"
)

// immutablePrefixes are the site directories whose files are named by
// version or content hash, so they never change under the same URL. goblog
// writes them itself (the Content Repository can't: they're reserved paths),
// and every file under them must be named that way.
var immutablePrefixes = []string{"fonts/", "assets/"} //nolint:gochecknoglobals // fixed set

// cacheControl returns the Cache-Control for serving the site file name.
func cacheControl(name string) string {
	for _, prefix := range immutablePrefixes {
		if strings.HasPrefix(name, prefix) {
			return cacheImmutable
		}
	}
	if strings.HasPrefix(name, "images/") {
		return cacheImages
	}
	return cacheShort
}
