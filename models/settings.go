package models

import "time"

// SiteSettings are the site-wide settings from the Content Repository's
// site.yaml.
type SiteSettings struct {
	// Title is the site's name, shown on every page.
	Title string
	// Author is the name of the blog's author.
	Author string
	// Description says what the blog is about, for page metadata.
	Description string
	// URL is the site's base URL, e.g. https://blog.example.com, with no
	// trailing slash. site.yaml must set it; it's "" only in the default
	// settings, for a Content Repository with no site.yaml.
	URL string
	// Host is the label the status bar and info line show for the site.
	// "" means the first DNS label of URL, e.g. "blog".
	Host string
	// Nav are the navigation entries, in the order the site shows them.
	Nav []NavLink
	// Timezone decides which day "today" is for Publish Dates. Nil means
	// site.yaml doesn't set one.
	Timezone *time.Location
}

// NavLink is one navigation entry.
type NavLink struct {
	// Name is the entry's label, e.g. "about".
	Name string
	// Link is where it goes, e.g. "/about/".
	Link string
}
