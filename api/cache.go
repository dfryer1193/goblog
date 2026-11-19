package api

import (
	"strings"

	"github.com/dfryer1193/goblog/models"
)

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

// immutablePrefixes are the site sections whose files are named by content
// hash, so they never change under the same URL. goblog writes them itself
// (the Content Repository can't: they're reserved paths).
var immutablePrefixes = []string{ //nolint:gochecknoglobals // fixed set
	models.FontsSection + "/", models.AssetsSection + "/",
}

// cacheControl returns the Cache-Control for serving the site file name.
func cacheControl(name string) string {
	for _, prefix := range immutablePrefixes {
		if strings.HasPrefix(name, prefix) {
			return cacheImmutable
		}
	}
	if strings.HasPrefix(name, models.ImagesSection+"/") {
		return cacheImages
	}
	return cacheShort
}
