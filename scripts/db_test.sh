#!/bin/sh
# Checks what can be checked about scripts/db.sh without a database.
#
# The part that matters most is the first: every table a migration creates has
# to be in scripts/db-counts.sql. A table missing there is a table whose rows a
# restore can lose without any comparison noticing — the counts would match
# because nobody counted it.
#
# Runs in the Lint stage of the pipeline next to docs_test.sh, so it uses
# nothing the Go image does not have: no docker, no kubectl, no az.
set -eu

cd "$(dirname "$0")/.."

fail=0

note() {
	echo "FAIL: $1" >&2
	fail=1
}

# --- Every migrated table is counted -------------------------------------
tables=$(grep -h -o -i -E 'create table (if not exists )?[a-z_]+' db/migrations/*.sql |
	awk '{print tolower($NF)}' | sort -u)
[ -n "$tables" ] || note "found no CREATE TABLE in db/migrations — has the directory moved?"
for t in $tables; do
	grep -q -E "from ${t}( |$)" scripts/db-counts.sql ||
		note "table ${t} is created in db/migrations but not counted in scripts/db-counts.sql"
done
grep -q 'goose_db_version' scripts/db-counts.sql ||
	note "scripts/db-counts.sql does not record the migration version"

# --- The scripts parse -----------------------------------------------------
for s in scripts/db.sh scripts/db-azure-job.sh; do
	sh -n "$s" || note "$s does not parse"
done

# --- Bad invocations stop before touching anything -----------------------
# A wrong ENV must not reach a backend: "kubernets" falling through to some
# default would be a restore into the wrong place.
expect_refusal() {
	if out=$(sh scripts/db.sh "$@" 2>&1); then
		note "db.sh $* succeeded, expected a refusal"
	elif ! echo "$out" | grep -q "$EXPECT"; then
		note "db.sh $* failed without saying '$EXPECT': $out"
	fi
}
EXPECT="ENV must be" expect_refusal dump kubernets
EXPECT="usage" expect_refusal restore compose
EXPECT="usage" expect_refusal dump
EXPECT="no file" expect_refusal restore compose /nonexistent.sql

if [ "$fail" -ne 0 ]; then
	exit 1
fi
echo "db: counted tables and invocations are consistent"
