#!/bin/sh
# Dumps the game database out of one environment and restores it into another:
# Compose, a CloudNativePG cluster on Kubernetes, or Azure Flexible Server.
#
#   sh scripts/db.sh dump    compose|kubernetes|azure
#   sh scripts/db.sh restore compose|kubernetes|azure FILE
#
# Run through `task db:dump` and `task db:restore`. The rules are
# docs/adr/0016; why each step looks the way it does is docs/backup-restore.md.
#
#   - One format everywhere: a plain pg_dump with --no-owner --no-acl.
#   - A restore only goes into a database without players. It resets the schema
#     the application's own migration may already have created and loads as
#     the role the application connects as, in one transaction.
#   - Every dump writes row counts next to itself as FILE.counts, taken before
#     and after the dump and required to be equal. Every restore compares
#     against them, sorted, because row order follows the collation.
#
# Configuration comes from the environment; the Taskfile passes its defaults:
#   DB_NAME, DB_OWNER       schmetterpause (Azure reads both from the tfvars)
#   NAMESPACE, DB_CLUSTER   schmetterpause, schmetterpause-db
#   APP_DEPLOYMENT          schmetterpause
#   TF_DIR                  terraform
# Compose honours COMPOSE_PROJECT_NAME and COMPOSE_FILE like any docker compose
# call; Kubernetes uses whatever kubeconfig and context kubectl would.
set -eu

cd "$(dirname "$0")/.."
umask 077

DB_NAME=${DB_NAME:-schmetterpause}
DB_OWNER=${DB_OWNER:-schmetterpause}
NAMESPACE=${NAMESPACE:-schmetterpause}
DB_CLUSTER=${DB_CLUSTER:-schmetterpause-db}
APP_DEPLOYMENT=${APP_DEPLOYMENT:-schmetterpause}
TF_DIR=${TF_DIR:-terraform}

COUNTS_SQL=scripts/db-counts.sql
JOB_SCRIPT=scripts/db-azure-job.sh

say() { echo "db: $*"; }
die() {
	echo "db: $*" >&2
	exit 1
}

usage() {
	die "usage: db.sh dump compose|kubernetes|azure | db.sh restore compose|kubernetes|azure FILE"
}

# Temporary files — the dump in flight, generated job definitions holding a
# password — live in a private directory that goes away however this ends.
# Hooks registered in cleanup_hooks run first: restarting a stopped app,
# deleting an Azure resource group.
tmp=$(mktemp -d)
cleanup_hooks=""
cleanup() {
	status=$?
	for hook in $cleanup_hooks; do
		"$hook" || true
	done
	rm -rf "$tmp"
	exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

same_counts() {
	sort "$1" >"$tmp/sorted-a"
	sort "$2" >"$tmp/sorted-b"
	cmp -s "$tmp/sorted-a" "$tmp/sorted-b"
}

# The number of players through a backend's psql, 0 when the table does not
# exist yet.
players_in() {
	exists=$(echo "select to_regclass('public.players') is not null;" | "$1")
	if [ "$exists" = t ]; then
		echo "select count(*) from public.players;" | "$1"
	else
		echo 0
	fi
}

# The cascade lists every table it drops as a NOTICE, which buries the lines
# that matter; warnings and errors still come through.
reset_sql() {
	printf '%s\n' 'SET client_min_messages = warning;' 'DROP SCHEMA public CASCADE;' 'CREATE SCHEMA public;'
}

# Prints the lines between ==NAME== and the next ==MARKER in a job log.
section() {
	sed -n "/^==$1==\$/,/^==[A-Z]/p" "$2" | sed '1d;$d'
}

# ── Compose ──────────────────────────────────────────────────────────────────

compose_check() {
	command -v docker >/dev/null || die "docker is missing"
	[ -n "$(docker compose ps --status running --quiet db)" ] ||
		die "the Compose database is not running — task up, or task office:up"
}

compose_psql() {
	docker compose exec -T db psql -U "$DB_OWNER" -d "$DB_NAME" -X -q -A -t -F '|' -v ON_ERROR_STOP=1
}

compose_dump() {
	docker compose exec -T db pg_dump -U "$DB_OWNER" -d "$DB_NAME" --no-owner --no-acl
}

compose_load() {
	docker compose exec -T db psql -U "$DB_OWNER" -d "$DB_NAME" -X -q -1 -v ON_ERROR_STOP=1 >/dev/null
}

# The app migrates on start and serves requests; neither belongs in the middle
# of a restore. Stopped here, started again by the cleanup hook — also when the
# restore fails, so a failed restore does not leave the app down.
compose_before_restore() {
	if [ -n "$(docker compose ps --status running --quiet app)" ]; then
		say "stopping the app for the restore"
		docker compose stop app >/dev/null 2>&1
		cleanup_hooks="$cleanup_hooks compose_start_app"
	fi
}

compose_start_app() {
	docker compose start app >/dev/null 2>&1 && say "app started again"
}

# ── Kubernetes (CloudNativePG) ───────────────────────────────────────────────

kubernetes_check() {
	command -v kubectl >/dev/null || die "kubectl is missing"
	say "context $(kubectl config current-context 2>/dev/null || echo '(none)'), namespace $NAMESPACE, cluster $DB_CLUSTER"
	pod=$(kubectl -n "$NAMESPACE" get clusters.postgresql.cnpg.io "$DB_CLUSTER" \
		-o jsonpath='{.status.currentPrimary}' 2>/dev/null || true)
	[ -n "$pod" ] || die "no CloudNativePG cluster $DB_CLUSTER with a primary in namespace $NAMESPACE"
}

# psql and pg_dump run as postgres inside the primary's pod, over the local
# socket — no credentials leave the cluster.
kubernetes_psql() {
	kubectl -n "$NAMESPACE" exec -i "$pod" -c postgres -- \
		psql -d "$DB_NAME" -X -q -A -t -F '|' -v ON_ERROR_STOP=1
}

kubernetes_dump() {
	kubectl -n "$NAMESPACE" exec "$pod" -c postgres -- \
		pg_dump -d "$DB_NAME" --no-owner --no-acl
}

# SET ROLE first: loaded as postgres, every table would belong to postgres and
# the application, which connects as the owner role, could not touch them.
kubernetes_load() {
	{
		printf 'SET ROLE "%s";\n' "$DB_OWNER"
		cat
	} | kubectl -n "$NAMESPACE" exec -i "$pod" -c postgres -- \
		psql -d "$DB_NAME" -X -q -1 -v ON_ERROR_STOP=1 >/dev/null
}

# Scaling a Deployment is a decision about that environment, so it is left to
# the operator rather than done behind their back.
kubernetes_before_restore() {
	replicas=$(kubectl -n "$NAMESPACE" get deployment "$APP_DEPLOYMENT" \
		-o jsonpath='{.spec.replicas}' 2>/dev/null || true)
	if [ -n "$replicas" ] && [ "$replicas" != 0 ]; then
		die "deployment/$APP_DEPLOYMENT in $NAMESPACE runs $replicas replica(s). Scale it to 0 first, and back afterwards:
    kubectl -n $NAMESPACE scale deployment/$APP_DEPLOYMENT --replicas=0"
	fi
}

# ── Azure (Flexible Server, through a Container Instance) ────────────────────

tfvar() {
	sed -n "s/^$1 *= *\"\\(.*\\)\".*/\\1/p" "$2" | head -1
}

azure_check() {
	command -v az >/dev/null || die "the Azure CLI is missing"
	command -v terraform >/dev/null || die "terraform is missing"
	az account get-access-token --output none 2>/dev/null || die "not logged in to Azure — task tf:login"
	values=$TF_DIR/schmetterpause.auto.tfvars
	secrets=$TF_DIR/terraform.tfvars
	[ -f "$secrets" ] || die "$secrets is missing"
	fqdn=$(terraform -chdir="$TF_DIR" output -raw postgres_fqdn 2>/dev/null || true)
	[ -n "$fqdn" ] || die "no Azure instance in the Terraform state — task tf:apply first"
	app_rg=$(terraform -chdir="$TF_DIR" output -raw resource_group)
	location=$(tfvar location "$values")
	prefix=$(tfvar name_prefix "$values")
	major=$(tfvar postgres_version "$values")
	DB_OWNER=$(tfvar postgres_user "$values")
	DB_NAME=$(tfvar postgres_db "$values")
	# Its own resource group, never the instance's: azurerm refuses to delete a
	# group holding resources it does not manage, so a leftover job there would
	# block tf:destroy.
	job_rg="${prefix}-dbjob-rg"
	[ "$(az group exists -n "$job_rg")" = false ] ||
		die "resource group $job_rg exists, so a previous run did not finish. Delete it: az group delete -n $job_rg --yes"
	say "server $fqdn (PostgreSQL $major), job group $job_rg"
}

azure_group() {
	az group create -n "$job_rg" -l "$location" -o none
	cleanup_hooks="$cleanup_hooks azure_delete_group"
}

azure_delete_group() {
	az group delete -n "$job_rg" --yes --no-wait -o none && say "deleting $job_rg in the background"
}

# Writes the container group definition. The password and, for a restore, the
# storage key go only into this file in the private temp directory, and it is
# removed as soon as the container exists.
azure_yaml() {
	job=$1
	password=$(tfvar postgres_password "$secrets")
	{
		cat <<EOF
apiVersion: '2021-10-01'
location: $location
name: schmetterpause-$job
type: Microsoft.ContainerInstance/containerGroups
properties:
  osType: Linux
  restartPolicy: Never
EOF
		if [ "$job" = restore ]; then
			cat <<EOF
  volumes:
    - name: dump
      azureFile:
        shareName: dump
        storageAccountName: $storage
        storageAccountKey: '$storage_key'
        readOnly: true
EOF
		fi
		cat <<EOF
  containers:
    - name: $job
      properties:
        image: ghcr.io/cloudnative-pg/postgresql:$major
        resources:
          requests:
            cpu: 1.0
            memoryInGB: 1.0
EOF
		if [ "$job" = restore ]; then
			cat <<EOF
        volumeMounts:
          - name: dump
            mountPath: /dump
            readOnly: true
EOF
		fi
		cat <<EOF
        environmentVariables:
          - name: JOB
            value: $job
          - name: PGHOST
            value: $fqdn
          - name: PGUSER
            value: $DB_OWNER
          - name: PGDATABASE
            value: $DB_NAME
          - name: PGSSLMODE
            value: require
          - name: PGPASSWORD
            secureValue: '$password'
        command:
          - /bin/sh
          - -c
          - |
EOF
		{
			echo "cat > /tmp/c.sql <<'SQL'"
			cat "$COUNTS_SQL"
			echo "SQL"
			cat "$JOB_SCRIPT"
		} | sed 's/^/            /'
	} >"$tmp/job.yaml"
	password=""
}

azure_run() {
	job=$1
	az container create -g "$job_rg" --file "$tmp/job.yaml" -o none
	rm -f "$tmp/job.yaml"
	say "waiting for the $job container"
	tries=0
	while :; do
		state=$(az container show -g "$job_rg" -n "schmetterpause-$job" \
			--query 'containers[0].instanceView.currentState.state' -o tsv 2>/dev/null || true)
		[ "$state" = Terminated ] && break
		tries=$((tries + 1))
		[ "$tries" -lt 90 ] || die "the $job container did not finish within 15 minutes"
		sleep 10
	done
	exit_code=$(az container show -g "$job_rg" -n "schmetterpause-$job" \
		--query 'containers[0].instanceView.currentState.exitCode' -o tsv)
	az container logs -g "$job_rg" -n "schmetterpause-$job" --container-name "$job" >"$tmp/job.log"
}

azure_dump_to() {
	out=$1
	azure_group
	azure_yaml dump
	azure_run dump
	if [ "$exit_code" != 0 ]; then
		tail -20 "$tmp/job.log" >&2
		die "the dump container exited with $exit_code"
	fi
	section COUNTS-BEFORE "$tmp/job.log" >"$tmp/before"
	section COUNTS-AFTER "$tmp/job.log" >"$tmp/after"
	same_counts "$tmp/before" "$tmp/after" || die "the database changed while it was being dumped — dump again"
	section BEGIN "$tmp/job.log" | base64 -d | gunzip >"$tmp/dump.sql"
	want=$(sed -n 's/^==SHA256== //p' "$tmp/job.log")
	[ "$(sha256sum "$tmp/dump.sql" | cut -d' ' -f1)" = "$want" ] ||
		die "the dump arrived with a different checksum than it left with"
	mv "$tmp/dump.sql" "$out"
	cp "$tmp/before" "$out.counts"
}

# The dump reaches the container through an Azure Files share in the job group,
# uploaded over HTTPS. The storage account goes when the group goes.
azure_upload() {
	storage="spdb$(openssl rand -hex 8)"
	az storage account create -g "$job_rg" -n "$storage" -l "$location" \
		--sku Standard_LRS --kind StorageV2 --min-tls-version TLS1_2 \
		--allow-blob-public-access false -o none
	storage_key=$(az storage account keys list -g "$job_rg" -n "$storage" --query '[0].value' -o tsv)
	AZURE_STORAGE_ACCOUNT=$storage AZURE_STORAGE_KEY=$storage_key \
		az storage share create --name dump -o none
	AZURE_STORAGE_ACCOUNT=$storage AZURE_STORAGE_KEY=$storage_key \
		az storage file upload --share-name dump --source "$1" --path dump.sql --no-progress -o none
}

azure_restore_from() {
	azure_group
	azure_upload "$1"
	azure_yaml restore
	storage_key=""
	azure_run restore
	if grep -q '^==REFUSED==' "$tmp/job.log"; then
		die "refusing: $(sed -n 's/^==REFUSED== //p' "$tmp/job.log") in the Azure database. A restore replaces everything; empty the target deliberately first"
	fi
	if [ "$exit_code" != 0 ]; then
		tail -20 "$tmp/job.log" >&2
		die "the restore container exited with $exit_code"
	fi
	section COUNTS "$tmp/job.log" >"$tmp/restored"
	# The init container migrated the empty database before the restore. A
	# restart lets it run against the restored schema, which matters as soon as
	# the image is newer than the dump.
	app="${prefix}-app"
	revision=$(az containerapp revision list -n "$app" -g "$app_rg" \
		--query '[?properties.active].name | [0]' -o tsv 2>/dev/null || true)
	if [ -n "$revision" ]; then
		az containerapp revision restart -n "$app" -g "$app_rg" --revision "$revision" -o none
		say "restarted revision $revision"
	fi
}

# ── dump and restore ─────────────────────────────────────────────────────────

dump() {
	env=$1
	"${env}_check"
	out="schmetterpause-${env}-$(date +%Y-%m-%d-%H%M).sql"
	[ ! -e "$out" ] || die "$out exists already"
	if [ "$env" = azure ]; then
		azure_dump_to "$out"
	else
		"${env}_psql" <"$COUNTS_SQL" >"$tmp/before"
		"${env}_dump" >"$tmp/dump.sql"
		"${env}_psql" <"$COUNTS_SQL" >"$tmp/after"
		same_counts "$tmp/before" "$tmp/after" || die "the database changed while it was being dumped — dump again"
		mv "$tmp/dump.sql" "$out"
		cp "$tmp/before" "$out.counts"
	fi
	chmod 600 "$out" "$out.counts"
	say "wrote $out, $(wc -c <"$out") bytes, sha256 $(sha256sum "$out" | cut -d' ' -f1)"
	say "counts: $(sort "$out.counts" | tr '\n' ' ')"
}

restore() {
	env=$1
	file=$2
	[ -f "$file" ] || die "no file $file"
	"${env}_check"
	if [ "$env" = azure ]; then
		azure_restore_from "$file"
	else
		players=$(players_in "${env}_psql")
		[ "$players" = 0 ] ||
			die "refusing: the $env database already has $players players. A restore replaces everything; empty the target deliberately first"
		"${env}_before_restore"
		{
			reset_sql
			cat "$file"
		} | "${env}_load"
		"${env}_psql" <"$COUNTS_SQL" >"$tmp/restored"
	fi
	say "restored $file into $env"
	if [ -f "$file.counts" ]; then
		if same_counts "$file.counts" "$tmp/restored"; then
			say "counts match $file.counts: $(sort "$tmp/restored" | tr '\n' ' ')"
		else
			diff "$tmp/sorted-a" "$tmp/sorted-b" >&2 || true
			die "counts differ from $file.counts"
		fi
	else
		say "WARNING: no $file.counts next to the dump. It loaded without an error, but nothing was compared."
	fi
}

[ $# -ge 2 ] || usage
case "$2" in
compose | kubernetes | azure) ;;
*) die "ENV must be compose, kubernetes or azure, not '$2'" ;;
esac
case "$1" in
dump)
	[ $# -eq 2 ] || usage
	dump "$2"
	;;
restore)
	[ $# -eq 3 ] || usage
	restore "$2" "$3"
	;;
*) usage ;;
esac
