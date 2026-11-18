package service

import (
	"encoding/xml"
)

// sitemapFile is the sitemap's output file, at /sitemap.xml.
const sitemapFile = "sitemap.xml"

// sitemapNS is the sitemaps.org XML namespace.
const sitemapNS = "http://www.sitemaps.org/schemas/sitemap/0.9"

type sitemapURLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	NS      string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

// renderSitemap renders /sitemap.xml, listing the site's public pages as
// absolute URLs: the index, the Published Posts, the Pages, /tags/ and the
// tags of the Published Posts. Drafts and tags only Drafts have are left
// out, even when the build includes them. It needs the base URL, so without
// one (see hasFeed) there's no sitemap.
func renderSitemap(b *siteBuild) error {
	if !hasFeed(&b.settings) {
		return nil
	}
	posts := publishedPosts(b.posts)
	set := sitemapURLSet{NS: sitemapNS, URLs: []sitemapURL{{Loc: b.absURL("/")}}}
	for _, p := range posts {
		set.URLs = append(set.URLs, sitemapURL{Loc: b.absURL(p.URL()), LastMod: p.PublishDate.String()})
	}
	for _, p := range b.pages {
		set.URLs = append(set.URLs, sitemapURL{Loc: b.absURL(p.URL())})
	}
	set.URLs = append(set.URLs, sitemapURL{Loc: b.absURL(tagsURL)})
	for _, tag := range collectTags(posts) {
		set.URLs = append(set.URLs, sitemapURL{Loc: b.absURL(tag.URL())})
	}

	return b.out.xml(sitemapFile, set)
}
