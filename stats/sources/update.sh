#!/bin/sh
# Refreshes the vendored search-engine and social list from
# matomo-org/searchengine-and-social-list at a commit, and records it in
# ORIGIN.md. Usage, from the repository root:
#
#   stats/sources/update.sh <commit>
#
# Then run `make lint test`, and commit the result.
set -eu

commit=${1:?usage: $0 <commit sha of matomo-org/searchengine-and-social-list>}
dir=$(dirname "$0")
base=https://raw.githubusercontent.com/matomo-org/searchengine-and-social-list/$commit

for f in SearchEngines.yml Socials.yml AIAssistants.yml LICENSE; do
	curl -fsSL -o "$dir/$f" "$base/$f"
done
sed -i \
	-e "s/^- \*\*Commit:\*\* .*/- **Commit:** \`$commit\`/" \
	-e "s/^- \*\*Fetched:\*\* .*/- **Fetched:** $(date -u +%Y-%m-%d)/" \
	"$dir/ORIGIN.md"
