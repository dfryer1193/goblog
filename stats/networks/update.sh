#!/bin/sh
# Refreshes the vendored hosting-network list from ipverse/as-metadata at a
# commit, and records it in ORIGIN.md. Usage, from the repository root:
#
#   stats/networks/update.sh <commit>
#
# It needs curl and jq. Then run `make lint test`, and commit the result.
set -eu

commit=${1:?usage: $0 <commit sha of ipverse/as-metadata>}
dir=$(dirname "$0")
base=https://raw.githubusercontent.com/ipverse/as-metadata/$commit
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# as.json is about 70 MB; only its hosting networks' AS numbers are kept.
curl -fsSL -o "$tmp/as.json" "$base/as.json"
jq -r '.[] | select(.metadata.category == "hosting") | .asn' "$tmp/as.json" |
	sort -nu >"$dir/hosting-asns.txt"
curl -fsSL -o "$dir/LICENSE" "$base/LICENSE"
sed -i \
	-e "s/^- \*\*Commit:\*\* .*/- **Commit:** \`$commit\`/" \
	-e "s/^- \*\*Fetched:\*\* .*/- **Fetched:** $(date -u +%Y-%m-%d)/" \
	-e "s/^- \*\*Size:\*\* .*/- **Size:** $(wc -l <"$dir/hosting-asns.txt" | tr -d ' ') AS numbers/" \
	"$dir/ORIGIN.md"
