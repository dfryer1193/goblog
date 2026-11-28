# Research: site statistics for goblog

Researched 2026-11-27. Claims come from primary sources: official docs, specs, regulator guidance, and source code at pinned tags or commits. Where something couldn't be sourced primarily, the text says so.

**Pinned source trees** (citation prefix → repo @ tag = commit):

| Prefix | Repo | Pin |
|---|---|---|
| `pl:` | plausible/analytics | `v3.2.2` = [`8ace1f3`][pl] |
| `ce:` | plausible/community-edition | [`5ce37d8`][ce] (its compose pins image `v3.2.2`) |
| `gc:` | arp242/goatcounter | `v2.7.0` = [`7e91d8a`][gc] (released 2025-12-15, still the latest release) |
| `um:` | umami-software/umami | `v3.5.0` = [`d3c62a8`][um] |
| `umd:` | umami-software/docs | [`9fef3bc`][umd] |
| `eg:` | envoyproxy/gateway | `v1.10.0` = [`02e9f89`][eg] |
| `ga:` | allinurl/goaccess | `v1.12` = [`ef3c4f3`][ga] |
| `mm:` | oschwald/maxminddb-golang | `v2.9.1` = [`f39be33`][mm] |
| `dd:` | matomo-org/device-detector | `6.5.2` = [`e83ce7a`][dd] |
| `cua:` | monperrus/crawler-user-agents | [`1965f15`][cua] |
| `nb:` | samuelclay/NewsBlur | [`394e46d`][nb] |

Citations look like `pl:lib/plausible/session/salts.ex#L81`. To build a URL, use `https://github.com/<repo>/blob/<pin>/<path>#L<n>`. Web pages are cited by URL and were all retrieved on 2026-11-27.

**Not reachable from this environment:** goatcounter.com and umami.is (connection refused). For GoatCounter, the help pages and pricing text come from the templates in its repo (`gc:tpl/help/*.md`, `gc:tpl/home.gohtml`), which is what the site serves. For Umami, the docs come from `umd:`. **Umami Cloud's prices are not sourced** (see §7d).

[pl]: https://github.com/plausible/analytics/tree/8ace1f34f6ebc1d6257ffdc6f9e5a16b6728045c
[ce]: https://github.com/plausible/community-edition/tree/5ce37d85f75fba08f2aeef016310f9f7abab5804
[gc]: https://github.com/arp242/goatcounter/tree/7e91d8a9bdbb0dd48496e498c5680f8f3477a1b4
[um]: https://github.com/umami-software/umami/tree/d3c62a805beda8643cb468924dda38617fb4c286
[umd]: https://github.com/umami-software/docs/tree/9fef3bc8812561dc0d2eb5eba9406a1a4e05622a
[eg]: https://github.com/envoyproxy/gateway/tree/02e9f89d1c1dc16b723de48fa3289dfa413a6e01
[ga]: https://github.com/allinurl/goaccess/tree/ef3c4f3c3e42b378e705fb3a1a5ef1c2e72e6348
[mm]: https://github.com/oschwald/maxminddb-golang/tree/f39be33372c0c622f1fb45f4764d4166b022ce06
[dd]: https://github.com/matomo-org/device-detector/tree/e83ce7a97f05d8ee3e1eee7ea77a73b3bdfebd9a
[cua]: https://github.com/monperrus/crawler-user-agents/tree/1965f15f9e521c8bfca1ae9bfbed48f8df0b1bca
[nb]: https://github.com/samuelclay/NewsBlur/tree/394e46da66d53ca30c8907a59dfdb480003711ab

**goblog facts this report relies on** (from this worktree):

- Every page is a static file served by `fileHandler` (`api/router.go:66-109`). HTML, the feed and the sitemap are sent with `Cache-Control: public, max-age=60` (`api/cache.go:21`), so a reload or back-navigation within 60 s may never reach the server.
- goblog sets no `Referrer-Policy` and no CSP header, and doesn't read `X-Forwarded-For` (grep of the tree).
- The no-JS test (`service/nojs_test.go:26`) rejects `<script`, `on*=` attributes, `javascript:` and any `.js` file other than the tree script. It does **not** reject `<img>` tags or CSS `url()`.

---

## 1. Metrics and how each is defined

Definitions come from Plausible's [metrics definitions](https://plausible.io/docs/metrics-definitions) page, Umami's metric definitions (`umd:content/docs/metric-definitions.mdx`), and GoatCounter's help pages (`gc:tpl/help/sessions.md`).

| Metric | Plausible | Umami | GoatCounter | From HTTP requests alone? |
|---|---|---|---|---|
| **Unique visitors** | People per day. No persistent ID, so other devices and other days count as new visitors. Several sessions on the same day count once. | "Unique number of sessions". The session ID is a hash of website ID, IP, UA and a salt that rotates monthly by default (umd:…metric-definitions.mdx#L35, #L128). | GoatCounter counts **visits, not people**: the first load of each path within a session (see §3). It can be turned off, and then every pageview counts (gc:tpl/help/sessions.md#L1-L14). | Yes: hash IP+UA+salt (§3). |
| **Pageviews** | Every page load. | "Views": every event collected (#L127). | Every page load. "Visits" is the headline number. | Yes, but bots, prefetches and HEAD requests have to be filtered out (§5). |
| **Visits / sessions** | Start when the visitor lands and end after 30 min with no action. | A "visit" is a hash of the session ID and a salt that rotates hourly, and can span hours (#L129; um:src/lib/collection-session.ts#L110-L113). | The session lasts 8 h, in memory (gc:tpl/help/sessions.md#L21). | Yes: group requests by visitor hash and an idle timeout. |
| **Bounce rate** | The visitor neither views more than one page nor triggers a custom event. | A visit with exactly one event, divided by visits (#L130). | Not reported (no definition in gc help). | Approximately: a session with one HTML request. |
| **Visit duration** | Sum of session lengths divided by sessions. A single-page visit counts as 0 s. | Time from first to last event per visit. Only multi-page visits contribute (#L131). | No definition found. | Approximately: first to last request. The time spent on the last page is unknowable, which is the same limitation Umami has. |
| **Time on page, scroll depth, engagement** | Engagement events sent by the script on navigation or tab switch. | Not among the default metrics. Umami collects Web Vitals instead. | No definition found. | **No: needs client-side JS.** |
| **Top pages, entry/exit pages** | Entry is the first page of a visit. Exit rate is exits divided by that page's pageviews. | Shown as pages. | Pages. | Yes. Same-origin navigations carry the full previous URL as `Referer` (§2), so the path through the site is visible too. |
| **Referrer sources** | Shown only when a source starts a new visit. | `document.referrer`, sent in the payload. | `Referer`, or `r=` on the pixel. | Yes: `Referer` on the HTML request. **Not** through a pixel (§2). |
| **UTM / campaigns** | `utm_source/medium/campaign/content/term`, plus `ref` and `source` ([sources docs](https://plausible.io/docs/top-referrers)). | `utm_*` plus ad click IDs (`gclid`, `fbclid`, …). | `utm_campaign`/`campaign`; source from `utm_source`, `ref`, `src` or `source` (gc:tpl/help/campaigns.md). | Yes: query string on the landing request. |
| **Country / region / city** | From IP. Plausible stores only country, region and city ([data policy](https://plausible.io/data-policy)). | From IP or CDN headers. | Country by default (bundled GeoLite2 Country, §4). | Yes: IP to offline DB (§4). |
| **Browser / OS / device** | From the UA, using device-detector data ([Events API](https://plausible.io/docs/events-api)). | Browser and OS from the UA. Device is "identified from the combination of OS and screen". | From the UA. | Browser and OS yes. Device class mostly, from the UA. |
| **Screen size** | Not in the data-policy table. | `screen` comes from the JS payload. | `s=` comes from JS (gc:tpl/help/pixel.md#L18). | **No: JS only.** |
| **Language** | Not collected. | `navigator.language`, from JS. | Not collected. | Partly. `Accept-Language` is available server-side, but none of the three tools documents using it. |

Plausible's data policy says its script also sends scroll depth and engagement time, and that it skips browsers that report automation (Selenium, Playwright) ([data policy](https://plausible.io/data-policy)). Neither signal is available server-side.

---

## 2. What `Referer` actually contains today

**The spec default.** "The default referrer policy is `strict-origin-when-cross-origin`" ([Referrer Policy, Editor's Draft §3](https://w3c.github.io/webappsec-referrer-policy/)). An `<a>` element without a `referrerpolicy` attribute has the empty policy, which falls back to that default. Under this policy (§3.7):

- **Same-origin** requests send the full URL, path and query included.
- **Cross-origin HTTPS→HTTPS** requests send only the origin (`https://news.example/`).
- **HTTPS→HTTP** requests send no `Referer` at all.

The older W3C TR snapshot still lists `no-referrer-when-downgrade` as the fallback ([TR](https://www.w3.org/TR/referrer-policy/) §3.9). The Editor's Draft is current.

**Browsers.**

- Chrome made `strict-origin-when-cross-origin` its default starting in Chrome 85 ([Chrome for Developers](https://developer.chrome.com/blog/referrer-policy-new-chrome-default)).
- Firefox did the same in Firefox 87 ([Mozilla Security Blog, 2021-03-22](https://blog.mozilla.org/security/2021/03/22/firefox-87-trims-http-referrers-by-default-to-protect-user-privacy/)).
- Safari's ITP 2.3 goes further for some cases. It downgrades `document.referrer` to the eTLD+1 when the referrer has link decoration and the user came from a classified domain ([WebKit blog](https://webkit.org/blog/9521/intelligent-tracking-prevention-2-3/)).
- The referring site can tighten its policy further, for example with `<meta name="referrer" content="origin">`. GoatCounter's source notes that Hacker News does exactly this (gc:ref.go#L22).

**What this means in practice.**

- **Sources come as hostnames, not pages.** Plausible's docs say so directly: "browsers only send the domain name of the referrer and not the actual URL" ([sources](https://plausible.io/docs/top-referrers)). Google doesn't include search keywords either. Plausible gets those from Search Console instead.
- **"Direct / None" is large.** It covers typed URLs, email, and messaging apps (WhatsApp, Slack, Discord) that send no referrer ([Plausible](https://plausible.io/docs/metrics-definitions)).
- **Internal navigation keeps full URLs.** Because goblog sets no policy, the default applies, and a `Referer` from one goblog page to another carries the full previous URL.
- **A tracking pixel can't see the visitor's referrer.** The pixel request's `Referer` is the goblog page itself. A static page can't embed the visitor's original referrer without JS. GoatCounter says so: "The tracking pixel won't allow recording the referrer or screen size" (gc:tpl/help/pixel.md#L18). Server-side counting on the HTML request **does** see the real `Referer`.

**Classifying sources.**

| Tool | Search engines / social | Spam | Tag params (priority over `Referer`) |
|---|---|---|---|
| Plausible | RefInspector, loaded from `priv/ref_inspector/referers.yml` (Snowplow referer-parser data), plus a `custom_sources.json` (pl:lib/plausible/ingestion/source.ex#L18-L23, #L56-L64). Channels follow GA-style rules ([sources](https://plausible.io/docs/top-referrers)). | `referrer_blocklist`, which pulls Matomo's `referrer-spam-list` weekly (pl:mix.exs#L128; its README). | `utm_source`, then `source`, then `ref` (source.ex#L67-L76). |
| GoatCounter | A hand-written `groups`/alias table plus rules in `cleanRefURL`, for example all `google.*` → "Google" (gc:ref.go#L21-L49, #L147-L166). | Bot hits aren't linked to refs (gc:hit.go#L260-L262). No spam list found. | `utm_source`, `ref`, `src`, `source` (gc:hit.go#L200-L211). |
| Umami | Not researched in source. | — | `utm_*` (umd metric definitions). |

Reusable data: [snowplow-referer-parser/referer-parser](https://github.com/snowplow-referer-parser/referer-parser), [matomo-org/searchengine-and-social-list](https://github.com/matomo-org/searchengine-and-social-list) and [matomo-org/referrer-spam-list](https://github.com/matomo-org/referrer-spam-list).

**Android apps.** Android's `Intent.EXTRA_REFERRER` is "typically an http: or https: URI… it can also use the android-app: scheme to identify a native application" ([Android reference](https://developer.android.com/reference/android/content/Intent#EXTRA_REFERRER)). That is the Android API, and it doesn't say what Chrome puts in the HTTP header. **No primary source was found for the claim that browsers forward `android-app://<package>/` as the HTTP `Referer`.** GoatCounter does handle such values in practice:

- It strips the scheme (gc:ref.go#L154).
- It maps package names, for example `com.google.android.gm` → "Email" and `com.google.android.googlequicksearchbox` → "Google" (gc:ref.go#L38, #L48).
- Values that aren't http(s) get `RefSchemeOther` (gc:hit.go#L237-L242).

---

## 3. Unique visitors without cookies

| | Plausible | Umami | GoatCounter |
|---|---|---|---|
| ID | `SipHash(salt, UA <> IP <> domain <> root_domain)` (pl:lib/plausible/ingestion/event.ex#L540-L554) | `uuidv5(hash(websiteId, ip, ua, sessionSalt, distinctId, secret()))`, where `secret()` = hash(`APP_SECRET`) (um:src/lib/collection-session.ts#L111; um:src/lib/crypto.ts#L61-L64) | Session key = `siteID + User-Agent + IP`, mapped in memory to a random UUIDv4. Only the UUID reaches the DB (gc:tpl/help/sessions.md#L19-L28, pseudo-code #L36-L60) |
| Salt | 16 random bytes (salts.ex#L81). A new one is generated daily at 00:00 (pl:config/runtime.exs#L782). The current and previous salts are kept, and salts older than 48 h are deleted (salts.ex#L29, #L88). A visit that crosses midnight is matched against the previous salt's ID (event.ex#L404-L410). | **Not random.** `hash(startOfMonth(date))`, so it rotates monthly by default. `SALT_ROTATION` can set day, week, month or a fixed value (crypto.ts#L88-L92). The secret part comes from `APP_SECRET`. | No salt. The UUID is random, and the mapping expires after 8 h. |
| Window | 1 day | 1 month by default. The "visit" uses a salt that rotates hourly (collection-session.ts#L110). | 8 h |
| Stored | Hashed ID per event. The raw IP and UA are never stored ([data policy](https://plausible.io/data-policy)). | Session record. The IP isn't stored as a field (umd metric definitions). | Aggregates plus a random UUID per pageview. **But see below:** the session map, keyed by raw UA and IP, is written to the DB on shutdown. |

**GoatCounter's source doesn't match its help page.** The help says "The IP address and User-Agent are never stored to the database or disk" (gc:tpl/help/sessions.md#L28). In the source:

- The in-memory session key is the plain string `fmt.Sprintf("%s-%s-%d", ua, remoteAddr, siteID)` (gc:memstore.go#L371-L375).
- On shutdown, `StoreSessions` JSON-encodes the session maps, including `sessionHashes` (UUID → that key), into the DB table `store` (gc:memstore.go#L128-L150).
- On the next start, `Init` loads that row and deletes it (gc:memstore.go#L76-L99).

So raw UA+IP pairs for sessions active in the last 8 h sit in the database between a shutdown and the next start.

*Inference, not stated by Umami:* Umami's session salt is derived from the date and `APP_SECRET`, not from deleted random bytes. Anyone holding `APP_SECRET` can therefore recompute candidate IDs for a guessed IP and UA, at any later time. Plausible's design rules that out once a salt is deleted.

**Same technique, server-side, in Go:** a daily random salt, kept in memory or persisted, and an ID of `SHA-256/SipHash(salt‖IP‖UA)`. With **multiple replicas**, all pods need the **same** salt, or one visitor counts once per pod. That forces shared state (a DB row or a Kubernetes Secret rotated by a CronJob). Alternatively, the gateway can pin each client to one pod: Envoy Gateway's `BackendTrafficPolicy` load balancer supports `type: ConsistentHash` with `SourceIP` (eg:api/v1alpha1/loadbalancer_types.go#L84, #L106-L108). Even then, counts still have to be merged across pods.

---

## 4. Geolocation (IP → country, offline)

| DB | Licence / attribution | Updates | Access | Size (observed) |
|---|---|---|---|---|
| **MaxMind GeoLite** (Country/City/ASN) | EULA incorporating CC BY-SA 4.0. Attribution is required, e.g. "This product includes GeoLite Data created by MaxMind, available from https://www.maxmind.com". You must use updates promptly and **destroy old versions within 30 days** of a new release ([GeoLite EULA](https://www.maxmind.com/en/geolite2/eula) §3, §6). | Country and City twice weekly (Tue, Fri). ASN daily ([MaxMind release note 2022-05-25](https://dev.maxmind.com/geoip/release-notes/2022-05-25-geolite-databases-will-now-receive-twice-weekly-updates); [GeoLite on Snowflake](https://dev.maxmind.com/geoip/geolite-on-snowflake)). | Free account plus licence key, and `geoipupdate` ([updating databases](https://dev.maxmind.com/geoip/updating-databases/)). | GoatCounter's bundled `GeoLite2-Country.mmdb.gz` is 4.7 MB (git tree, gc:pkg/geo/). |
| **DB-IP Lite** (Country/City) | CC BY 4.0, "provided you give attribution to DB-IP.com" ([DB-IP Lite](https://db-ip.com/db/lite.php)). No account needed. | Monthly. | Direct download, CSV or MMDB ([country lite](https://db-ip.com/db/download/ip-to-country-lite)). | `dbip-country-lite-2026-11.mmdb.gz` is 4.2 MB; city lite is 60 MB (HTTP `Content-Length`, 2026-11-27). |
| **IPinfo Lite** (Country + ASN) | CC BY-SA 4.0, with attribution such as "IP address data is powered by IPinfo" ([IPinfo Lite](https://ipinfo.io/lite)). | Daily ("daily-updated"). | Free token. | Not measured. |

**What each tool defaults to:**

- Plausible CE ships `dbip-country.mmdb.gz` as its default, and can download GeoLite2-City instead if a MaxMind key is set (pl:config/runtime.exs#L246, #L896-L925).
- GoatCounter embeds GeoLite2-Country, and can download GeoLite2-City with an account ID and key (gc:pkg/geo/geo.go#L22-L23, #L115-L150).
- Umami reads `GEOLITE_DB_PATH` or `geo/GeoLite2-City.mmdb`, or trusted CDN headers (um:src/lib/detect.ts#L113-L118).

**Go:** `github.com/oschwald/maxminddb-golang/v2` is the reader for the MMDB format, so it reads all three DBs above. v2 uses `netip.Addr`, supports reflection-free decoding, has `Reader.Verify()`, and memory-maps files (`mmap_unix.go`). Don't rewrite an mmapped file in place: open a new Reader instead (mm:README.md#L1-L60, #L108-L110). It's "not an official MaxMind API".

**Envoy Gateway can also do the lookup** at the edge. `ClientTrafficPolicy` has a GeoIP provider (`type: MaxMind`, local `.mmdb` paths mounted into the Envoy container) (eg:api/v1alpha1/geoip_types.go). The documented use is authorization (eg:site/content/en/v1.10/tasks/security/geoip-authorization.md). Whether it can forward the country to the backend as a header wasn't checked.

### Real client IP behind Envoy Gateway

- By default Envoy Gateway sets `use_remote_address: true` on its listeners (eg:internal/xds/translator/listener.go#L371-L375). Envoy then treats the TCP peer as the client and appends it to `X-Forwarded-For` before proxying ([Envoy XFF docs](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_conn_man/headers#x-forwarded-for)). goblog would therefore read the **rightmost** XFF entry, because its TCP peer is the Envoy pod.
- When something sits in front of Envoy, `ClientTrafficPolicy.spec.clientIPDetection` takes exactly one of `xForwardedFor`, `customHeader` or `directSourceIP`. `xForwardedFor` takes either `numTrustedHops` ("the client IP is taken from the Nth address from the right") or `trustedCIDRs`, and optionally `disableXForwardedForAppend` (eg:api/v1alpha1/clienttrafficpolicy_types.go#L310-L384).
- **The TCP peer is the client only if the Service preserves it.** With `externalTrafficPolicy: Cluster`, kube-proxy SNATs traffic. `Local` "preserves the original source IP address" ([Kubernetes: Using Source IP](https://kubernetes.io/docs/tutorials/services/source-ip/)). Which one applies to the Envoy Service in this cluster wasn't checked.
- For comparison: Umami trusts the leftmost XFF entry unless `TRUSTED_PROXIES` is set, and the leftmost entry is client-spoofable (um:src/lib/ip.ts#L3-L16, #L68-L73, #L142; umd environment variables). GoatCounter's log importer uses "the last non-private IP" in XFF (gc:cmd/goatcounter/import.go#L158-L160).

---

## 5. Bots

| Source | What it is | Licence | Size |
|---|---|---|---|
| [monperrus/crawler-user-agents](https://github.com/monperrus/crawler-user-agents) | A JSON list of UA regexes. Ships a Go package with `IsCrawler`/`MatchingCrawlers` (cua:README.md). | MIT (cua:LICENSE) | 1,519 patterns |
| [matomo-org/device-detector](https://github.com/matomo-org/device-detector) | Full UA parser: browsers, OS, devices, bots, feed readers (`regexes/client/feed_readers.yml`). Plausible uses its data through UAInspector. | LGPL-3.0+ (dd:README.md) | 843 bot regexes in `regexes/bots.yml` |
| `zgo.at/isbot` | GoatCounter's bot detector (gc:go.mod). Its JS sends `b=150–153` for headless browsers (gc:tpl/help/pixel.md). | not checked | — |
| `isbot` (npm) | Umami drops a request if `isbot(userAgent)`, unless `DISABLE_BOT_CHECK` is set (um:src/app/api/send/route.ts#L96-L98). | not checked | — |

**Plausible's layers** ([bot filtering](https://plausible.io/docs/bot-traffic-filtering); pl:event.ex#L212-L215, #L257-L262, #L568-L572):

- the JS skips browsers that report automation;
- UA bots and Headless Chrome are dropped;
- referrer spam is dropped;
- about 40,800 data-centre IP ranges are filtered. **This layer is Cloud only.** CE has only "basic bot filtering… User-Agent header and referrer spam domains" ([self-hosting page](https://plausible.io/docs/self-hosting)).

**How big the problem is for server-side counting.** Plausible reports that in its own comparison it "saw 18x more pageviews in server logs than in Plausible" ([bot filtering](https://plausible.io/docs/bot-traffic-filtering)). That is a vendor's own test, not an independent measurement. It also notes that AI crawlers (GPTBot, ClaudeBot, …) don't run JS, so they appear only in server logs. GoatCounter says log import and the pixel bring "more bot requests" (gc:tpl/help/logfile.md#L19; pixel.md#L18-L19). A JS beacon filters out every non-JS client for free. Server-side counting needs, at minimum:

- a UA list;
- counting only `GET` requests for HTML with status 200 or 304;
- excluding `/feed.xml`, `/sitemap.xml`, `/robots.txt` and assets;
- optionally, data-centre ASN filtering (GeoLite ASN or IPinfo Lite ASN).

Both GoatCounter's importer and Plausible count only pageview-like requests (`-exclude static -exclude redirect` is the importer default, gc:cmd/goatcounter/import.go).

**Not researched:** whether prefetch/prerender requests (for example `Sec-Purpose: prefetch`) inflate server-side counts, and how to detect them.

---

## 6. Feed subscribers

Hosted readers fetch a feed once for all their users and report the count in the `User-Agent`:

| Reader | UA format | Source |
|---|---|---|
| Inoreader | `Inoreader/1.0 (+http://www.inoreader.com/feed-fetcher; 3 subscribers; )`. "fetches your feed only once and distributes the content to all of them" | [Inoreader Feed Fetcher](https://www.inoreader.com/feed-fetcher) (official) |
| NewsBlur | `NewsBlur Feed Fetcher - N subscriber(s) - <permalink> ("Mozilla/5.0 …")`. The fallback omits the browser suffix. | nb:apps/rss_feeds/models.py#L1864-L1899 |
| Feedly | UA `Feedly/1.0`. The official page documents the UA and says "the same feed has multiple ids… it might be extracted multiple times". **It does not document a subscriber count in the UA.** The commonly seen `…; N subscribers; …` form has no primary source. | [Feedly Fetcher](https://feedly.com/fetcher.html) |
| Others (Feedbin, The Old Reader, …) | Not researched. | — |

**Counting method** (an inference from the formats above; no standard exists): for each aggregator, take the latest `N subscribers` per day. Watch for several feed IDs per aggregator (Feedly) and sum them only if they're distinct. Then add distinct IP+UA pairs per day from readers that run on the user's own device, which fetch individually; device-detector's `feed_readers.yml` identifies them. **No primary-sourced, maintained tool for counting feed subscribers was found.** GoAccess and GoatCounter don't document one.

---

## 7. Approaches compared

### a. In-process counting in goblog (middleware)

A wrapper around `fileHandler` (`api/router.go:50`) that sees each request: path, status, `Referer`, `User-Agent`, query string and the XFF-derived IP.

**Data each approach can see:**

| | Sees real `Referer` | UTM | Country | UA | Screen/engagement |
|---|---|---|---|---|---|
| Middleware | yes | yes | yes (with mmdb) | yes | no |
| Log parsing | only if added to the log format | yes | yes | yes | no |
| Self-hosted pixel `<img>` | **no** (it sees the page instead) | only if baked in per page | yes | yes | no |
| JS beacon | yes (`document.referrer`) | yes | yes | yes | yes |

**Storage options:**

- **In memory**, flushed to a file: aggregates lost on crash; no dependency.
- **SQLite** via `modernc.org/sqlite`: pure Go, no cgo (README at `v1.63.0`; its platform list wasn't verified).
- **Postgres**, shared.

**Cost:** an mmdb reader (about 4–5 MB country DB, mmapped); a UA list (1.5k regexes, or device-detector data); a daily-salt map or a hash per request. **No primary source gives a memory or CPU benchmark** for this exact setup.

**Multi-replica problem.** Each pod sees a disjoint share of requests. That has three consequences:

- **Per-pod files or SQLite need merging.** Summing works for counts, not for unique visitors, unless the salt is shared or consistent-hash LB is used (§3).
- **SQLite can't be shared over network storage safely.** "If your data is separated from the application by a network, you want to use a client/server database" ([SQLite: use over a network](https://www.sqlite.org/useovernet.html)).
- **The no-DB decision is affected.** ADR 0001 says v1 has no DB and that comments will be the first to need one (`docs/adr/0001-render-on-change-static-output.md:12`). Stats would be an earlier, or a second, stateful feature.

Possible ways around it: a single replica owns the stats; each pod forwards events to one collector (a goblog endpoint, or an analytics service's API); or each pod writes JSON lines to stdout and something downstream aggregates them.

### b. Parsing gateway access logs

- **Envoy Gateway's default access log is JSON on stdout.** It includes `x-forwarded-for`, `user-agent`, `:authority`, method, path and response code, but **no `Referer` field** (eg:site/content/en/v1.10/tasks/observability/proxy-accesslog.md#L24-L49; eg:api/v1alpha1/envoyproxy_accesslogging_types.go#L16, #L73). You'd need a custom format on the `EnvoyProxy` resource adding `%REQ(REFERER)%` ([Envoy access-log format](https://www.envoyproxy.io/docs/envoy/latest/configuration/observability/access_log/usage)). That changes logging for every route on that gateway.
- **GoAccess** (C) parses custom formats, **including JSON log-formats** (ga:goaccess.1#L245-L258). It treats "the same IP, the same date, and the same user agent" as a unique visitor, and that count includes crawlers by default (#L26-L30). It offers `--ignore-crawlers`, `--anonymize-ip` and in-memory hash tables with optional on-disk persistence (#L151-L154, #L826-L869, #L941). No country without a GeoIP DB. Its report is an HTML page, viewed by the author rather than the site's visitors.
- **`goatcounter import`** reads `combined`, `combined-vhost`, `common`, `bunny`, `caddy` (JSON), or a custom `log:` format with `$xff`, `$referrer` and `$user_agent`. It can `-follow` a file, or read `-` (stdin). It needs a running GoatCounter and an API key (gc:cmd/goatcounter/import.go#L28-L169). Envoy's JSON isn't a predefined format, so you'd need a text Envoy format that matches a `log:` template. The help says you lose screen sizes, titles and canonical-URL handling, and gain bot noise (gc:tpl/help/logfile.md#L15-L22).
- **Matomo Log Analytics:** "an alternative… by importing web server logs" ([Matomo](https://matomo.org/log-analytics/)).

Multi-replica: logs from every Envoy pod have to be collected, for example by a log shipper or `kubectl logs`. goblog's own replica count doesn't matter. Logs contain raw IPs, so their retention becomes a privacy question in its own right.

### c. Self-hosted analytics services

| | GoatCounter | Umami | Plausible CE | Matomo |
|---|---|---|---|---|
| Language / stack | Go, single static binary (gc:Dockerfile) | Node.js ≥ 20.19 (22 LTS recommended) + PostgreSQL ≥ 12.14 (umd:content/docs/install.mdx) | Elixir app + **PostgreSQL 16 + ClickHouse 24.12** (ce:compose.yml) | PHP ≥ 8.1 + MySQL ≥ 8.0 / MariaDB ≥ 10.6 for Matomo 6 ([requirements](https://matomo.org/faq/on-premise/matomo-requirements/)) |
| DB | SQLite (default) or PostgreSQL. "SQLite should work well for most smaller sites" (gc:README.md#L81-L100) | PostgreSQL only (umd faq) | Postgres + ClickHouse | MySQL/MariaDB |
| Documented minimum resources | **None documented** | **None documented** | "At least **2 GB of RAM** is recommended". The CPU needs SSE 4.2 or NEON (ce:README.md). Ships ClickHouse low-resource configs (ce:compose.yml) | ≤100k pageviews/month: "2 CPU, 2 GB RAM, 50GB SSD" ([requirements](https://matomo.org/faq/on-premise/matomo-requirements/)) |
| arm64 image | Yes: `arp242/goatcounter:2.7.0` has amd64 and arm64 (Docker Hub API) | Yes: `ghcr.io/umami-software/umami:latest` has amd64 and arm64 (GHCR manifest). No `v3.5.0` tag on GHCR | Yes: `ghcr.io/plausible/community-edition:v3.2.2` has amd64 and arm64. `clickhouse-server:24.12-alpine` has amd64 and arm64 | Yes: official `matomo` image, many architectures including arm64 |
| No-JS input | **Pixel** `/count` (also via CSS `border-image`) (gc:tpl/help/pixel.md#L1-L12); **backend API** `POST /api/v0/count` with `ip`, `user_agent`, `ref`, `query` fields, ≤500 hits per request, persisted every 10 s (gc:handlers/api.go#L467-L540; gc:tpl/help/backend.md); **log import** | `POST /api/send` / `/api/batch`. No auth, but a real `User-Agent` is required (umd:content/docs/api/sending-stats.mdx). Separate "Pixels" entity for emails and external pages (umd pixels.mdx) | **Events API** `POST /api/event`. Needs the visitor's `User-Agent` and `X-Forwarded-For`. A server or data-centre IP gets the event silently dropped (`x-plausible-dropped: 1`) ([Events API](https://plausible.io/docs/events-api)) | Log import; HTTP tracking API (not researched) |
| Its own replicas | Sessions live in process memory (gc:memstore.go#L40-L44). *Inference:* run one replica | Stateless app over Postgres (not verified for multi-replica) | Not researched | Not researched |
| Licence | "slightly modified" EUPL 1.2 (gc:LICENSE) | MIT (GitHub licence API) | AGPL-3.0 ([self-hosting page](https://plausible.io/docs/self-hosting); GitHub licence API for plausible/analytics) | GPL-3.0 (GitHub licence API for matomo-org/matomo) |
| CE limits | — | — | Twice-yearly releases. No funnels and similar features. Basic bot filtering only ([self-hosting page](https://plausible.io/docs/self-hosting)) | — |

**Server-side forwarding note.** The backend APIs let goblog's middleware forward each pageview, with the visitor's real `Referer`, UA and IP, to the service. That gives the service's dashboard, sessions and geo with no JS in the page. The request path then calls an external service, so it would need to be async or batched (*inference*). goblog would also need to send the right client IP (§4).

### d. Hosted services

| Service | Price (as published 2026-11-27) | JS required? |
|---|---|---|
| Plausible Cloud | Starter **$9/mo**, up to 10k monthly pageviews, 1 site. Growth $14/mo (3 sites). Business $19/mo (10 sites). 30-day trial, no free tier ([plausible.io pricing section](https://plausible.io/)). Prices go up with traffic. The page localises the currency. EU-hosted (Hetzner, UpCloud) ([data policy](https://plausible.io/data-policy)). | Script by default. The Events API works without JS (see 7c). Cloud also filters data-centre IPs. |
| GoatCounter.com | "currently offered for free for reasonable public usage… millions of pageviews/day isn't". Donations via GitHub Sponsors (gc:tpl/home.gohtml). | No: pixel, backend API or log import are all available. |
| Umami Cloud | Hobby plan is "completely free", with 6-month retention. Pro and Business are paid, with 2- and 5-year retention. US or EU region (umd:content/docs/cloud/faq.mdx). **Prices unsourced** (umami.is unreachable). | Script by default. `/api/send` works server-side. |
| Cloudflare Web Analytics | "free, privacy-first analytics… without changing your DNS or using Cloudflare's proxy" ([about](https://developers.cloudflare.com/web-analytics/about/)). Unsampled for 7 days, then aggregated to about 10% ([FAQ](https://developers.cloudflare.com/web-analytics/faq/)). | **Yes.** A JS beacon from `static.cloudflareinsights.com`, or auto-injected when the site is proxied through Cloudflare ([data origin](https://developers.cloudflare.com/web-analytics/data-metrics/data-origin-and-collection/), [get started](https://developers.cloudflare.com/web-analytics/get-started/)). Edge analytics that can't be blocked need Cloudflare proxying, and the advanced version needs a Pro+ plan. |

---

## 8. Legal (brief, not legal advice)

Two regimes apply. **ePrivacy Art. 5(3)** covers storing or accessing information on the device. **GDPR** covers processing personal data, and IPs can be personal data: Recital 30 lists "internet protocol addresses" among online identifiers ([GDPR](https://eur-lex.europa.eu/eli/reg/2016/679/oj)).

**EDPB Guidelines 2/2023, v2.0** (adopted 7 October 2024) ([EDPB](https://www.edpb.europa.eu/our-work-tools/documents/public-consultations/2023/guidelines-22023-technical-scope-art-53-eprivacy_en)):

- "Gaining access" applies whenever an entity "instructs" the device to send information. JavaScript sending requests is the example given (¶32–33).
- **Tracking pixels**, including ones the site owner adds, fall within Art. 5(3) (¶47–51).
- **IP-only tracking** falls within it unless the entity can ensure the IP doesn't originate from the user's terminal (¶54–55).
- The guidelines **don't analyse exemptions**. Applicability "does not systematically mean that consent needs to be collected" (¶56).
- They don't directly address counting the ordinary page requests a site serves anyway, such as middleware or its own access logs. No primary source found settles that case.

**France (CNIL).** Audience-measurement trackers are exempt from consent ([CNIL, 4 July 2025](https://www.cnil.fr/fr/cookies-et-autres-traceurs/regles/cookies-solutions-pour-les-outils-de-mesure-daudience)) if they are:

- used strictly for audience measurement, solely for the publisher;
- producing anonymous statistics only;
- not cross-referenced with other processing, nor sent to third parties in non-anonymous form;
- not tracking the person across sites or apps.

The CNIL also recommends informing users (a privacy policy is enough), a tracker lifetime of 13 months at most, and keeping data 25 months at most.

**UK (ICO, PECR).** There is now a "statistical purposes" exception for storage or access whose sole purpose is collecting statistics about use of the service. It requires clear information **and "a simple means of objecting, free of charge"** ([ICO: What are the exceptions?](https://ico.org.uk/for-organisations/direct-marketing-and-privacy-and-electronic-communications/guidance-on-the-use-of-storage-and-access-technologies/what-are-the-exceptions/)).

**The tools' own positions:**

- **Plausible:** "you do not need cookie banners… or to collect consent" ([data policy](https://plausible.io/data-policy)).
- **GoatCounter:** "*probably* doesn't require a GDPR consent notice", based on no PII plus legitimate interest; "I am not a lawyer" (gc:tpl/help/gdpr.md).
- **Umami:** "GDPR compliance depends on how you configure and use it" (umd:content/docs/faq.mdx).

Other member states' rules (for example Germany's TDDDG) weren't researched.

---

## Implications for goblog

**The no-JS invariant rules out:**

- screen size, scroll depth, time on page, engagement time and Web Vitals;
- the bot filtering that comes free with a JS beacon;
- Plausible's and Umami's standard scripts, and Cloudflare Web Analytics (beacon only).

It does **not** rule out: `Referer`, UTM, country, browser/OS, pageviews, visitor and session estimates, entry/exit pages, or feed subscriber counts. All of those can be derived from requests goblog already serves.

**A pixel is a gray area.** A GoatCounter-style `<img>`/CSS pixel passes `service/nojs_test.go` as written. It still adds a third-party request (or a self-hosted one) and **loses the referrer**, which is the author's main interest. Under EDPB ¶47–51 it is also explicitly a "tracking pixel".

**The trade-offs, set out without a recommendation:**

| Option | Main-goal fit (referrers/campaigns/country) | Footprint | Statefulness / replicas | Ops | Privacy surface |
|---|---|---|---|---|---|
| **A. Middleware + own aggregates** (file/SQLite) | Full | Smallest: in-process, an mmdb of about 5 MB, a UA list | Adds state to a stateless server, against ADR 0001's "no DB in v1". Replicas need a shared salt and merging, a single owner, or consistent hashing | Code to write and maintain: classification lists, bot rules, a dashboard | Lowest if only aggregates and a rotating salt are kept. No third party. |
| **B. Middleware → analytics API** (GoatCounter `/api/v0/count`, Umami `/api/send`, Plausible `/api/event`) | Full, plus a ready-made dashboard | goblog stays stateless. The service brings its own footprint (see C) or is hosted | Works with any number of goblog replicas | Async forwarding in goblog, plus running or paying for the service | The IP and UA go to that service, which hashes them. Hosted means a processor/third party. |
| **C. Self-hosted service (any input)** | Full | GoatCounter: one Go binary + SQLite, footprint undocumented. Umami: Node + Postgres. Plausible CE: Postgres + ClickHouse, **≥2 GB RAM recommended**. Matomo: 2 CPU / 2 GB | The service is a new stateful workload. GoatCounter wants a single replica. | Upgrades, backups, a dashboard login | As in B, kept in-cluster. |
| **D. Gateway log parsing** (GoAccess, `goatcounter import`) | Full once `Referer` is added to the Envoy format | Batch or follow process; nothing in goblog | goblog replicas don't matter. Logs must be gathered from every Envoy pod | Changing EnvoyProxy logging affects the whole gateway. Log shipping. | Raw IPs sit in logs. Their retention needs a decision. |
| **E. Hosted, JS** (Plausible Cloud, Umami Cloud, Cloudflare) | Good, with better bot filtering | None in-cluster | — | Lowest | Third party. **Violates the no-JS invariant.** |

**Cross-cutting facts any option has to handle:**

- the client IP comes from the rightmost XFF entry under Envoy Gateway's defaults, and only if the Envoy Service preserves source IPs (§4);
- bots dominate raw server-side counts (§5);
- with `max-age=60` on HTML, repeat views within 60 s are invisible server-side;
- feed subscribers come from UA strings, not from visits (§6);
- GeoLite requires an account, attribution and destroying old DBs within 30 days. DB-IP Lite and IPinfo Lite need attribution only (§4).
