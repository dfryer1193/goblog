package stats

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"regexp/syntax"
	"slices"
	"strings"
	"sync"
)

// botUnknown is the Bot name of a request that's a Bot's by heuristics (no
// User-Agent, or an HTTP library's) rather than by the crawler list.
const botUnknown = "unknown"

// crawlerList is monperrus/crawler-user-agents' list of crawler
// User-Agent patterns. See bots/README.md for its origin, licence and how
// to update it.
//
//go:embed bots/crawler-user-agents.json
var crawlerList []byte

// The crawler list's tags that matter to Site Statistics.
const (
	// tagHTTPLibrary marks an HTTP library or command-line tool, whose
	// requests are the Bot "unknown".
	tagHTTPLibrary = "http-library"
	// tagFeedReader marks a feed reader.
	tagFeedReader = "feed-reader"
)

// prefilterLen is how many bytes of a pattern's literal the prefilter
// indexes it by.
const prefilterLen = 3

// crawler is one entry in the crawler list.
type crawler struct {
	// name is its Bot name.
	name string
	// feedReader is whether the list tags it as a feed reader.
	feedReader bool
	// re is its pattern, or nil if the pattern is a literal string.
	re *regexp.Regexp
}

// candidate is a crawler that may match a User-Agent containing literal.
type candidate struct {
	// crawler is the crawler's index in the list.
	crawler int
	// literal is a string that every User-Agent the crawler matches
	// contains, or one of several such strings.
	literal string
}

// crawlers matches User-Agents against the crawler list. Trying every
// pattern on every request would be slow, so each pattern is indexed by the
// first bytes of the literal strings that every match contains, and only
// the patterns whose literal is in the User-Agent are tried.
type crawlers struct {
	list []crawler
	// byPrefix are the candidates whose literal starts with each
	// prefilterLen bytes, in the list's order.
	byPrefix map[string][]candidate
	// unindexed are the indexes into list of the crawlers without such
	// literals, in order. They're always tried.
	unindexed []int
}

// parsedCrawlers is the crawler list, parsed once.
var parsedCrawlers = sync.OnceValues(newCrawlers) //nolint:gochecknoglobals // computed once from an embedded file

// newCrawlers parses the crawler list.
func newCrawlers() (*crawlers, error) {
	var entries []struct {
		Pattern string   `json:"pattern"`
		Tags    []string `json:"tags"`
	}
	if err := json.Unmarshal(crawlerList, &entries); err != nil {
		return nil, fmt.Errorf("parsing the crawler list: %w", err)
	}
	c := &crawlers{list: make([]crawler, len(entries)), byPrefix: map[string][]candidate{}}
	for i, e := range entries {
		cr := crawler{name: botName(e.Pattern), feedReader: slices.Contains(e.Tags, tagFeedReader)}
		if slices.Contains(e.Tags, tagHTTPLibrary) {
			cr.name = botUnknown
		}
		literals := []string{e.Pattern}
		if regexp.QuoteMeta(e.Pattern) != e.Pattern {
			re, err := regexp.Compile(e.Pattern)
			if err != nil {
				return nil, fmt.Errorf("compiling crawler pattern %q: %w", e.Pattern, err)
			}
			cr.re = re
			literals = requiredLiterals(re)
		}
		c.list[i] = cr
		if slices.ContainsFunc(literals, func(l string) bool { return len(l) < prefilterLen }) {
			c.unindexed = append(c.unindexed, i)
			continue
		}
		for _, l := range literals {
			key := l[:prefilterLen]
			c.byPrefix[key] = append(c.byPrefix[key], candidate{crawler: i, literal: l})
		}
	}
	return c, nil
}

// match returns the first crawler in the list that matches userAgent, if
// any does. With feedReaders, it considers only the list's feed readers.
func (c *crawlers) match(userAgent string, feedReaders bool) (crawler, bool) {
	best := len(c.list)
	for i := 0; i+prefilterLen <= len(userAgent); i++ {
		for _, cand := range c.byPrefix[userAgent[i:i+prefilterLen]] {
			if cand.crawler >= best {
				break
			}
			cr := c.list[cand.crawler]
			if (!feedReaders || cr.feedReader) && strings.HasPrefix(userAgent[i:], cand.literal) &&
				(cr.re == nil || cr.re.MatchString(userAgent)) {
				best = cand.crawler
			}
		}
	}
	for _, j := range c.unindexed {
		if j >= best {
			break
		}
		if cr := c.list[j]; (!feedReaders || cr.feedReader) && cr.re.MatchString(userAgent) {
			best = j
		}
	}
	if best == len(c.list) {
		return crawler{}, false
	}
	return c.list[best], true
}

// requiredLiterals are literal strings at least one of which every match of
// re contains, or nil if it can't tell.
func requiredLiterals(re *regexp.Regexp) []string {
	parsed, err := syntax.Parse(re.String(), syntax.Perl)
	if err != nil {
		return nil
	}
	return literalsOf(parsed)
}

// literalsOf are literal strings at least one of which every match of re
// contains, or nil if it can't tell.
func literalsOf(re *syntax.Regexp) []string {
	switch re.Op { //nolint:exhaustive // any other operator, such as a repeat, can match without a literal
	case syntax.OpLiteral:
		if re.Flags&syntax.FoldCase != 0 {
			return nil
		}
		return []string{string(re.Rune)}
	case syntax.OpCapture:
		return literalsOf(re.Sub[0])
	case syntax.OpAlternate:
		var all []string
		for _, sub := range re.Sub {
			literals := literalsOf(sub)
			if literals == nil {
				return nil
			}
			all = append(all, literals...)
		}
		return all
	case syntax.OpConcat:
		// The sub-expression whose shortest literal is longest filters best.
		var best []string
		bestLen := 0
		for _, sub := range re.Sub {
			literals := literalsOf(sub)
			if literals == nil {
				continue
			}
			if shortest := shortestLen(literals); shortest > bestLen {
				best, bestLen = literals, shortest
			}
		}
		return best
	default:
		return nil
	}
}

// shortestLen is the length of the shortest of literals.
func shortestLen(literals []string) int {
	return len(slices.MinFunc(literals, func(a, b string) int { return len(a) - len(b) }))
}

// botName is the Bot name for the crawler list's pattern: the pattern
// without its anchors, escapes and trailing slash, unless that's hard to
// read, as for "yandex\.com\/bots".
func botName(pattern string) string {
	readable := map[string]string{
		`AdsBot-Google([^-]|$)`:        "AdsBot-Google",
		`yandex\.com\/bots`:            "YandexBot",
		`(sistrix|SISTRIX) [cC]rawler`: "SISTRIX",
		`Ahrefs(Bot|SiteAudit)`:        "AhrefsBot",
		`S[eE][mM]rushBot`:             "SemrushBot",
		`[pP]ingdom`:                   "Pingdom",
		`Bark[rR]owler`:                "Barkrowler",
		`pinterest\.com\/bot`:          "Pinterestbot",
		`[cC]laude[bB]ot`:              "ClaudeBot",
		`Datadog( |\/)Synthetic`:       "Datadog Synthetic",
		`inoreader`:                    "Inoreader",
		`theoldreader\.com`:            "The Old Reader",
		`Feedfetcher-Google`:           "Google Feedfetcher",
	}
	if name, ok := readable[pattern]; ok {
		return name
	}
	name := strings.TrimSuffix(strings.TrimPrefix(pattern, "^"), "$")
	return strings.TrimRight(strings.ReplaceAll(name, `\`, ""), "/")
}

// botByHeuristics reports whether userAgent is a Bot's though the crawler
// list doesn't say so: it's empty, or the default of an HTTP library or
// command-line tool the list misses.
func botByHeuristics(userAgent string) bool {
	if userAgent == "" {
		return true
	}
	switch userAgent {
	case "Ruby", "node", "undici":
		return true
	}
	for _, prefix := range []string{
		"python-urllib3/", "Java/", "Java-http-client/", "GuzzleHttp/", "Faraday v", "Dart/", "Deno/", "ureq/", "xh/",
	} {
		if strings.HasPrefix(userAgent, prefix) {
			return true
		}
	}
	return false
}
