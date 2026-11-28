# MaxMind's test databases

`GeoLite2-City-Test.mmdb`, `GeoIP2-City-Test.mmdb` and `GeoLite2-ASN-Test.mmdb`
are MaxMind's published test databases, vendored unchanged. The seam-2 tests
serve them, as monthly releases, from a local stand-in for DB-IP's download
site. They're in MaxMind's format, as DB-IP's City Lite and ASN Lite
databases are, but their data is made up for testing.

- **Origin:** <https://github.com/maxmind/MaxMind-DB>, `test-data/`
- **Commit:** `5c972a8a85df5557c122f79d6e172c7a5cfe93c5`
- **Fetched:** 2026-11-27
- **Licence:** Copyright (c) 2013 - 2026 by MaxMind, Inc. Licensed under the
  Apache License, Version 2.0 (`LICENSE-APACHE`) or the MIT License
  (`LICENSE-MIT`), at your option.

The tests use the two databases' differences to tell one month's release
from the next: `214.0.1.1` is only in `GeoIP2-City-Test.mmdb` (AU, Victoria),
and `214.78.0.1` only in `GeoLite2-City-Test.mmdb` (US, California).

`GeoLite2-ASN-Test.mmdb` puts `1.0.0.1` in AS15169 (Google Inc.), a hosting
network in the vendored hosting list, and `89.160.20.112` in AS29518
(Bredband2 AB), an ISP that isn't.

Only these three files are vendored. Some other files in that directory are
deliberately hostile, so don't copy the directory wholesale.
