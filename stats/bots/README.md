# Vendored Bot list

Site Statistics embed this file with `go:embed` to recognise Bots.

## `crawler-user-agents.json`

- **What:** a list of crawler, bot and HTTP-library User-Agent patterns
  (regular expressions), each with tags such as `http-library` and
  `feed-reader`.
- **Origin:** [monperrus/crawler-user-agents](https://github.com/monperrus/crawler-user-agents),
  tag `v1.86.0`, commit `1965f15f9e521c8bfca1ae9bfbed48f8df0b1bca`, copied
  unchanged.
- **Licence:** MIT, Copyright (c) 2017 Martin Monperrus. The full text is in
  [`crawler-user-agents.LICENSE`](crawler-user-agents.LICENSE).

goblog uses each entry's `pattern` and `tags`:

- An entry's Bot name comes from its pattern, with the anchors, escapes and a
  trailing slash removed (`Googlebot\/` is `Googlebot`). A few patterns
  that would read badly have a name of their own, in `botName` in
  [`../bot.go`](../bot.go). Entries tagged `http-library` are all the Bot
  `unknown`. So the names are bounded by the list.
- Entries tagged `feed-reader` are feed readers when they request the feed.

### Updating it

1. Pick a newer tag at
   <https://github.com/monperrus/crawler-user-agents/tags> and note its
   commit.
2. Download the file and licence at that commit:

   ```sh
   commit=<commit>
   base=https://raw.githubusercontent.com/monperrus/crawler-user-agents/$commit
   curl -fsSL -o crawler-user-agents.json "$base/crawler-user-agents.json"
   curl -fsSL -o crawler-user-agents.LICENSE "$base/LICENSE"
   ```

3. Update the tag and commit above, check the licence is still MIT, and run
   `make lint test` from the repository's root. Every pattern must still
   compile with Go's `regexp`.
4. Bot names come from the patterns, so a changed pattern changes its Bot's
   name in the metrics and lines. Check the diff for changed patterns of
   Bots you watch.
