package models

// The rendered site's layout: the output files and folders the site build
// writes, relative to the output root (see Site.Files), and so the ones the
// server serves.
const (
	// IndexFile is the page a folder's URL serves: /x/ is x/index.html.
	IndexFile = "index.html"
	// NotFoundFile is the site's 404 page.
	NotFoundFile = "404.html"
	// FeedFile is the Atom feed, at /feed.xml.
	FeedFile = "feed.xml"
	// SitemapFile is the sitemap, at /sitemap.xml.
	SitemapFile = "sitemap.xml"
	// PostsSection holds a page per Post, at /posts/<slug>/.
	PostsSection = "posts"
	// TagsSection holds the tags page and a page per tag, at /tags/<slug>/.
	TagsSection = "tags"
	// ImagesSection holds the Content Repository's images, under the
	// same paths as in its images/ folder. They keep their names when the
	// author replaces them.
	ImagesSection = "images"
	// AssetsSection holds the stylesheet. Every file in it has a content
	// hash in its name, so it never changes under the same URL.
	AssetsSection = "assets"
	// FontsSection holds the fonts. Every file in it has a content hash
	// in its name, so it never changes under the same URL.
	FontsSection = "fonts"
	// LicensesSection holds the licences of goblog's own static files,
	// under their own names.
	LicensesSection = "licenses"
	// GzipSuffix ends the name of an output file's gzip variant:
	// index.html.gz is the gzip variant of index.html.
	GzipSuffix = ".gz"
)
