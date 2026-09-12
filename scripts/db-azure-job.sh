# Runs inside the one-off Azure Container Instance that scripts/db.sh creates —
# never on the operator's machine, which cannot reach port 5432
# (docs/backup-restore.md). The container image is
# ghcr.io/cloudnative-pg/postgresql, because Docker Hub refuses anonymous pulls
# from Azure.
#
# scripts/db.sh inlines this file into the container's command, after writing
# the counting queries to /tmp/c.sql. PGHOST, PGUSER, PGDATABASE, PGSSLMODE and
# PGPASSWORD are set; JOB says what to do.
#
# Everything the job reports goes to stdout between ==MARKER== lines. The logs
# are the only channel back, so the markers are the interface.
set -eu

counts() {
	psql -X -q -A -t -F '|' -v ON_ERROR_STOP=1 -f /tmp/c.sql
}

# The number of players, 0 when the table does not exist yet.
players() {
	if [ "$(psql -X -A -t -c "select to_regclass('public.players') is not null")" = t ]; then
		psql -X -A -t -c 'select count(*) from public.players'
	else
		echo 0
	fi
}

case "$JOB" in
dump)
	echo "==COUNTS-BEFORE=="
	counts
	pg_dump --no-owner --no-acl -f /tmp/d.sql
	echo "==COUNTS-AFTER=="
	counts
	echo "==SHA256== $(sha256sum /tmp/d.sql | cut -d' ' -f1)"
	echo "==BEGIN=="
	gzip -9 -c /tmp/d.sql | base64
	echo "==END=="
	;;
restore)
	n=$(players)
	if [ "$n" != 0 ]; then
		echo "==REFUSED== players has $n rows"
		exit 3
	fi
	# The application's init container has usually migrated the empty database
	# already, so the tables exist without rows. Reset the schema and load the
	# dump in one transaction: a failure leaves the database as it was.
	{
		echo 'SET client_min_messages = warning;'
		echo 'DROP SCHEMA public CASCADE;'
		echo 'CREATE SCHEMA public;'
		cat /dump/dump.sql
	} | psql -X -q -1 -v ON_ERROR_STOP=1 >/dev/null
	echo "==COUNTS=="
	counts
	echo "==END=="
	;;
*)
	echo "unknown JOB '$JOB'"
	exit 2
	;;
esac
