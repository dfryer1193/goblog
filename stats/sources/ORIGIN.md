# The search-engine and social list

`SearchEngines.yml`, `Socials.yml` and `AIAssistants.yml` are Matomo's
search-engine and social list, vendored unchanged. Site Statistics embed them
to name the search engines and social sites that Visitors come from.

- **Origin:** <https://github.com/matomo-org/searchengine-and-social-list>
- **Commit:** `35966f211dd594f9be5dbdc0b49fe67648059fb0`
- **Fetched:** 2026-11-27
- **Licence:** CC0 1.0 Universal, in `LICENSE`. No attribution is required,
  but the origin is recorded here anyway.

goblog reads only each entry's name and hosts. `SearchEngines.yml` entries
are Sources of kind `search`, `Socials.yml` entries of kind `social`, and
`AIAssistants.yml` entries of kind `ai`. Where a host is in more than one file, the
first of `AIAssistants.yml`, `SearchEngines.yml` and `Socials.yml` wins, so
an AI assistant the search engines list too, such as ChatGPT, is `ai`.

## Updating

The list's tags are rare, so pin a commit on its `master` branch. From the
repository root:

```sh
stats/sources/update.sh <commit>
make lint test
```

The script downloads the three lists and `LICENSE` at that commit, and
records the commit and date above. Commit the result.

Grouping other referrers by registrable domain uses the Public Suffix List
compiled into `golang.org/x/net/publicsuffix`. Updating that list means
updating `golang.org/x/net`.
