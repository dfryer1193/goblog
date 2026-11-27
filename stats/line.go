package stats

import "time"

// streamMarker is the value of every line's stream field, which tells Site
// Statistics lines apart from goblog's ordinary logs on stdout.
const streamMarker = "goblog_stats"

// The classes of request a line can be.
const (
	// classPageView is a Page View.
	classPageView = "page_view"
)

// line is one Site Statistics JSON line. Its field names are a contract
// (see the README): they're only ever added to.
type line struct {
	// Stream is always streamMarker.
	Stream string `json:"stream"`
	// Time is when the request was answered, in UTC.
	Time time.Time `json:"time"`
	// Class is the request's class, such as classPageView.
	Class string `json:"class"`
	// Path is the request's path, without its query.
	Path string `json:"path"`
	// Status is the response's status code.
	Status int `json:"status"`
	// Kind is the kind of page, such as kindPost.
	Kind string `json:"kind"`
	// VisitorID tells Visitors apart for the day; see visitorIDFor. It's
	// omitted without a salt.
	VisitorID string `json:"visitor_id,omitempty"`
	// Browser is a Page View's browser family, such as "Firefox", or
	// "other".
	Browser string `json:"browser,omitempty"`
	// BrowserMajor is a Page View's browser's major version, such as
	// "145". It's omitted when it isn't known.
	BrowserMajor string `json:"browser_major,omitempty"`
	// OS is a Page View's operating system family, such as "macOS", or
	// "other".
	OS string `json:"os,omitempty"`
	// Device is a Page View's device type, such as "mobile", or "unknown".
	Device string `json:"device,omitempty"`
	// Source is the Source's name, absent for no Source.
	Source string `json:"source,omitempty"`
	// SourceKind is the kind of Source, such as sourceSearch.
	SourceKind string `json:"source_kind,omitempty"`
	// CampaignSource, CampaignMedium and CampaignName are a Campaign's
	// tags, each absent if it isn't tagged.
	CampaignSource string `json:"campaign_source,omitempty"`
	CampaignMedium string `json:"campaign_medium,omitempty"`
	CampaignName   string `json:"campaign_name,omitempty"`
	// Country is the Visitor's country, as an ISO 3166-1 alpha-2 code
	// such as "US". It's omitted without the location database, or if it
	// doesn't know.
	Country string `json:"country,omitempty"`
	// Region is the Visitor's state or province, as its English name such
	// as "New York". It's omitted as Country is, or if the location
	// database has a country but no region.
	Region string `json:"region,omitempty"`
	// Entry is whether the referrer isn't the site itself.
	Entry *bool `json:"entry,omitempty"`
}

// withSource is l saying its Visitor came from src.
func (l line) withSource(src source) line {
	l.Source, l.SourceKind, l.Entry = src.Name, src.Kind, &src.Entry
	l.CampaignSource, l.CampaignMedium, l.CampaignName = src.Campaign.Source, src.Campaign.Medium, src.Campaign.Name
	return l
}
