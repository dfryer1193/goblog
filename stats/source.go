package stats

import (
	"embed"
	"fmt"
	"net"
	"net/url"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
	"golang.org/x/net/publicsuffix"
)

// The kinds of Source a line can say its Visitor came from.
const (
	// sourceSite is another site, named by its registrable domain.
	sourceSite = "site"
	// sourceSearch is a search engine on the list.
	sourceSearch = "search"
	// sourceAI is an AI assistant on the list.
	sourceAI = "ai"
	// sourceSocial is a social site on the list.
	sourceSocial = "social"
	// sourceCampaign is a Campaign the author tagged a link with.
	sourceCampaign = "campaign"
	// sourceDirect means no referrer and no Campaign: no Source.
	sourceDirect = "direct"
	// sourceInternal is navigation within the site: no Source.
	sourceInternal = "internal"
)

// maxValueLen caps, in characters, each Campaign value and Source name a
// line carries, since they come from the request.
const maxValueLen = 64

// searchEnginesFile is the list's file of search engines, which, unlike
// its other files, lists configurations with urls rather than hosts.
const searchEnginesFile = "SearchEngines.yml"

// listFile is one of the vendored list's files, and the kind of Source its
// entries are.
type listFile struct{ name, kind string }

// listFiles are the vendored list's files. On a host in more than one, the
// first file wins: an AI assistant that's also a search engine, such as
// ChatGPT, is an AI assistant.
func listFiles() []listFile {
	return []listFile{
		{"AIAssistants.yml", sourceAI},
		{searchEnginesFile, sourceSearch},
		{"Socials.yml", sourceSocial},
	}
}

// listFS is Matomo's search-engine and social list, with its AI
// assistants. See sources/ORIGIN.md
// for its origin, licence and how to update it.
//
//go:embed sources/*.yml
var listFS embed.FS

// source is where a Visitor came from, as a line says.
type source struct {
	// Kind is the kind of Source, such as sourceSearch.
	Kind string
	// Name is the Source's name, or "" for sourceDirect and
	// sourceInternal.
	Name string
	// Entry is whether the referrer isn't the site itself.
	Entry bool
	// Campaign is the Campaign's tags, for sourceCampaign.
	Campaign campaign
}

// campaign is a Campaign's normalised tags. Medium and Name may be "".
type campaign struct {
	Source, Medium, Name string
}

// listed is a Source on the list: its name and kind.
type listed struct {
	name, kind string
}

// sourceList maps the list's hosts, as it writes them (including the "{}"
// country placeholders), to their Sources.
type sourceList map[string]listed

// loadSourceList reads the vendored list.
func loadSourceList() (sourceList, error) {
	list := sourceList{}
	for _, file := range listFiles() {
		src, err := listFS.ReadFile("sources/" + file.name)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", file.name, err)
		}
		hosts, err := parseListFile(file.name, src)
		if err != nil {
			return nil, err
		}
		for name, names := range hosts {
			for _, host := range names {
				if _, ok := list[host]; !ok {
					list[host] = listed{name: name, kind: file.kind}
				}
			}
		}
	}
	return list, nil
}

// parseListFile maps each name in one of the list's files to its hosts.
// SearchEngines.yml lists configurations with urls, and the other files
// list hosts.
func parseListFile(name string, src []byte) (map[string][]string, error) {
	hosts := map[string][]string{}
	if name != searchEnginesFile {
		if err := yaml.Unmarshal(src, &hosts); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", name, err)
		}
		return hosts, nil
	}
	var engines map[string][]struct {
		URLs []string `yaml:"urls"`
	}
	if err := yaml.Unmarshal(src, &engines); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", name, err)
	}
	for engine, confs := range engines {
		for _, conf := range confs {
			hosts[engine] = append(hosts[engine], conf.URLs...)
		}
	}
	return hosts, nil
}

// lookup finds host, which is lower case, on the list the way Matomo
// does: as it is, without a www. or mobile prefix, with its country code
// as "{}", or as a parent domain (l.facebook.com is facebook.com).
func (l sourceList) lookup(host string) (listed, bool) {
	bare := host
	for _, prefix := range []string{"www.", "m.", "mobile."} {
		bare = strings.TrimPrefix(bare, prefix)
	}
	candidates := []string{host, bare}
	if suffix, icann := publicsuffix.PublicSuffix(bare); icann && len(bare) > len(suffix) &&
		suffix != "com" && suffix != "org" && suffix != "net" {
		candidates = append(candidates, strings.TrimSuffix(bare, suffix)+"{}") // google.co.uk is google.{}
	}
	if label, rest, ok := strings.Cut(bare, "."); ok && len(label) == 2 {
		candidates = append(candidates, "{}."+rest) // cn.bing.com is {}.bing.com
	}
	for _, c := range candidates {
		if s, ok := l[c]; ok {
			return s, true
		}
	}
	for parts := strings.Split(bare, "."); len(parts) > 2; parts = parts[1:] {
		if s, ok := l[strings.Join(parts[1:], ".")]; ok {
			return s, true
		}
	}
	return listed{}, false
}

// sourceOf is where req's Visitor came from, for a site whose own host is
// siteHost ("" for none).
func (l sourceList) sourceOf(req request, siteHost string) source {
	if siteHost == "" {
		siteHost = hostOf(req.host)
	}
	ref, scheme := referrerHost(req.referer)
	internal := ref != "" && sameSite(ref, siteHost)
	if c, ok := campaignOf(req.query()); ok {
		return source{Kind: sourceCampaign, Name: c.Source, Entry: !internal, Campaign: c}
	}
	switch {
	case ref == "":
		return source{Kind: sourceDirect, Entry: true}
	case internal:
		return source{Kind: sourceInternal}
	}
	if s, ok := l.lookup(ref); ok {
		return source{Kind: s.kind, Name: s.name, Entry: true}
	}
	return source{Kind: sourceSite, Name: siteName(ref, scheme), Entry: true}
}

// referrerHost is the lower-case host of a Referer header, or "" if it has
// none, and then its scheme.
func referrerHost(referer string) (string, string) {
	u, err := url.Parse(strings.TrimSpace(referer))
	if err != nil {
		return "", ""
	}
	return hostOf(u.Host), strings.ToLower(u.Scheme)
}

// hostOf is hostport's host, lower case, without its port or brackets, or
// a trailing dot.
func hostOf(hostport string) string {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	return strings.TrimSuffix(strings.ToLower(host), ".")
}

// sameSite reports whether host is siteHost, ignoring a www. prefix on
// either.
func sameSite(host, siteHost string) bool {
	return strings.TrimPrefix(host, "www.") == strings.TrimPrefix(siteHost, "www.")
}

// siteName is the Source name of a referrer from host that isn't on the
// list: its registrable domain. An app's package name (from a non-web
// scheme such as android-app), an IP address or a host with no registrable
// domain is named as it is.
func siteName(host, scheme string) string {
	if scheme != "http" && scheme != "https" || net.ParseIP(host) != nil {
		return capped(host)
	}
	domain, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		return capped(host)
	}
	return capped(domain)
}

// campaignOf is the Campaign that query tags, if it tags one: utm_source,
// with utm_medium and utm_campaign, or else ref.
func campaignOf(query url.Values) (campaign, bool) {
	if src := normalised(query.Get("utm_source")); src != "" {
		return campaign{
			Source: src,
			Medium: normalised(query.Get("utm_medium")),
			Name:   normalised(query.Get("utm_campaign")),
		}, true
	}
	if src := normalised(query.Get("ref")); src != "" {
		return campaign{Source: src}, true
	}
	return campaign{}, false
}

// normalised is a Campaign value trimmed, lower-cased and capped.
func normalised(value string) string {
	return capped(strings.ToLower(strings.TrimSpace(value)))
}

// capped is s cut to at most maxValueLen characters, and made valid UTF-8.
func capped(s string) string {
	s = strings.ToValidUTF8(s, "")
	if utf8.RuneCountInString(s) <= maxValueLen {
		return s
	}
	return string([]rune(s)[:maxValueLen])
}
