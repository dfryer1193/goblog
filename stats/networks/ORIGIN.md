# The hosting-network list

`hosting-asns.txt` is the AS numbers of hosting, cloud and datacenter
networks, one per line in ascending order. Site Statistics embed it to say
whether a Page View or Visitor 404 comes from a **hosting network**.

It's generated from ipverse/as-metadata's `as.json`: the `asn` of every entry
whose `metadata.category` is `hosting` ("Hosting, cloud, or content
provider"). Nothing else from that file is kept.

- **Origin:** <https://github.com/ipverse/as-metadata>
- **Commit:** `12aa88df0aca920b09529ce11352fd0731820773`
- **Fetched:** 2026-11-27
- **Size:** 12287 AS numbers
- **Licence:** CC0 1.0 Universal, in `LICENSE`. No attribution is required,
  but the origin is recorded here anyway.

ipverse's categories are automated, from the AS's primary function, and its
README warns they won't always be right. The list also has the CDNs that
iCloud Private Relay and Cloudflare WARP egress from (Cloudflare's AS13335,
Akamai's and Fastly's), so their users read as on a hosting network. See the
README's Site Statistics section.

## Updating

ipverse commits a new version daily, without tags, so pin a commit on its
`main` branch. From the repository root, with `curl` and `jq`:

```sh
stats/networks/update.sh <commit>
make lint test
```

The script downloads `as.json` (about 70 MB) to a temporary directory,
regenerates `hosting-asns.txt` and `LICENSE` at that commit, and records the
commit, date and size above. Check the licence is still CC0, and commit the
result.
