# Site Statistics without a database or cookies

The author wants Site Statistics (where Visitors come from, how many there are, and what they read) in the Grafana they already run, without adding a database to goblog (ADR-0001), without JavaScript, and without anything that needs a consent banner. So goblog's own request handler classifies each request as a Page View, a Bot request or a feed fetch, and emits what it learns as structured JSON lines on stdout plus Prometheus metrics. The cluster's existing log and metrics stores collect them, and recording rules there roll the lines up into daily summaries kept long-term. goblog stores nothing. Visitors are told apart by a hash of IP and User-Agent with a salt that is random, shared by every replica, rotated at 00:00 UTC, and destroyed when it rotates, so no Visitor can be re-identified once the day ends. Raw IP addresses are never stored, and neither is anything set on the reader's device.

## Considered Options

- **Parsing the gateway's access logs.** This needs no goblog code, but the gateway can't salt and hash a field. It would have to log raw IPs, which are personal data, or truncated ones, which merge strangers on mobile carriers into one Visitor. It would also move bot detection, Source grouping and geolocation into untested log-pipeline regexes, and a log-format change would apply to every site on the gateway.
- **A self-hosted analytics service (GoatCounter, Umami, Plausible CE).** Each is a new stateful workload with its own dashboard, where the author wants Grafana. GoatCounter's source writes raw IP+UA session keys to its database, against its own documentation. Plausible CE needs ClickHouse and at least 2 GB of RAM, too much for a memory-constrained cluster.
- **A tracking pixel or script.** An image request's `Referer` is the goblog page, so a pixel loses the Visitor's Source. Regulators treat pixels as device access that needs consent. A script would break the site's no-JavaScript rule.
- **A salt derived from a stored secret plus the date** (Umami's approach). It's easier to share across replicas, but anyone holding the secret can rebuild every old hash for every IPv4 address, so it isn't anonymous.

## Consequences

- The real client IP has to reach goblog, so the gateway must preserve source addresses. Once it does, the gateway's own access log must stop recording full client IPs, or it would store what goblog refuses to.
- The daily salt lives outside goblog, in a Secret that a scheduled job replaces. If it's missing, goblog emits everything except Visitor IDs.
- goblog's JSON line format and metrics are a public contract. The dashboard, recording rules and infrastructure live in the private deployment repo.
- Comments are still expected to be the first feature that needs a database.
