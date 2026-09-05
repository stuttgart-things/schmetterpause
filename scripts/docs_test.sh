#!/bin/sh
# Checks that the documentation site still describes the documentation.
#
# Two kinds of drift, both silent and both only visible once the site is
# published: an ADR index that no longer lists every ADR, and a nav that names
# a page nobody wrote or omits one somebody did. MkDocs says nothing about
# either — a missing page is an INFO line in a build log, and a stale index is
# just prose.
#
# Runs in the Lint stage of the pipeline, so it uses nothing the Go image does
# not have. In particular it does not parse YAML: it reads the nav entries out
# of mkdocs.yml with sed, which is enough to check that both sides name the
# same files.
set -eu

cd "$(dirname "$0")/.."

fail=0

note() {
	echo "FAIL: $1" >&2
	fail=1
}

# --- The generated ADR index is current ---------------------------------
current=$(mktemp)
trap 'rm -f "$current"' EXIT
sh scripts/adr_index.sh >"$current"

if ! diff -u docs/adr/index.md "$current" >/dev/null 2>&1; then
	note "docs/adr/index.md is stale — run 'task docs:index'"
	diff -u docs/adr/index.md "$current" >&2 || true
fi

# --- Every ADR is in the nav -------------------------------------------
for adr in docs/adr/[0-9]*.md; do
	grep -q "adr/${adr##*/}" mkdocs.yml ||
		note "${adr} is not in the nav in mkdocs.yml"
done

# --- Every page the nav names exists ------------------------------------
#
# Matches the nav's "- Titel: pfad.md" lines. Top-level keys such as site_url
# are not indented list items and so cannot match.
nav_pages=$(sed -n 's/^ *- *[^:]*: *\([A-Za-z0-9._/-]*\.md\) *$/\1/p' mkdocs.yml)

if [ -z "$nav_pages" ]; then
	note "no nav entries found in mkdocs.yml — has the nav been restructured?"
fi

for page in $nav_pages; do
	[ -f "docs/${page}" ] || note "the nav names docs/${page}, which does not exist"
done

[ "$fail" -eq 0 ] || exit 1

echo "docs: ADR index and nav are current"
