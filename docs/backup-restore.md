# Backing up and moving the game data

The rules are in [ADR-0016](adr/0016-azure-auf-zeit-daten-ziehen-um.md): the
Azure instance only ever runs temporarily, and the game data moves between
Compose, Kubernetes and Azure as a logical dump, one writer at a time.

This page is the how. It describes what has actually been done and checked —
a dump taken from the Azure instance before its teardown, decoded, verified
and restored into a local PostgreSQL on 2026-09-12. Restoring into Azure or
Kubernetes has not been done yet, and neither has turning these steps into
tasks; both are [#213](https://github.com/stuttgart-things/schmetterpause/issues/213).

## What a dump is here

- A plain-SQL `pg_dump`: schema, data and `goose_db_version`, taken with
  `--no-owner --no-acl`, because the administrator on Flexible Server is not a
  superuser and the roles differ in every environment anyway.
- Restored into an **empty** database with `psql -v ON_ERROR_STOP=1`. The
  application migrates forward from there on its next start.
- Only ever into the **same or a newer** PostgreSQL major, and the same or a
  newer application version. The target is 18 everywhere: Compose, the kcl
  default (#221) and Azure are on it. The office cluster still runs 17 until its
  data moves to an 18 cluster (#213); a dump from it goes into any of the
  others, and nothing goes back into it.
- **Sensitive.** It holds display names, PIN hashes and recovery-code hashes.
  Name it `schmetterpause-*.sql` in the repository root, which is gitignored,
  keep it at mode `600`, and never let it become a CI artefact.

## Azure: dump before `task tf:destroy`

### Why not from your own machine

Both obvious ways failed, and the steps below exist because of it.

**Outbound port 5432 is blocked in the office network.** A firewall rule on the
server for your public address does nothing against that — the connection
never leaves the building. Check before trying:

```sh
curl -s --max-time 10 http://portquiz.net:5432/ >/dev/null && echo open || echo blocked
```

**Docker Hub refuses anonymous pulls from Azure.** A Container Instance with
`postgres:17-alpine` fails with `RegistryErrorResponse` from
`index.docker.io`. `ghcr.io/cloudnative-pg/postgresql:<major>` works — the image
the kcl module runs the database with — and has `pg_dump`, `psql`, `gzip`,
`base64` and `sha256sum`.

So the dump runs **inside Azure**, in a one-off Container Instance that reaches
the server through the existing "allow Azure services" rule. It prints the dump
gzipped and base64-encoded between markers, with a checksum and row counts, and
you fetch it with `az container logs`. Nothing but HTTPS leaves your machine.

### Why a separate resource group

`azurerm` refuses to delete a resource group that still contains resources it
does not manage (`prevent_deletion_if_contains_resources` defaults to `true`).
A Container Instance inside `schmetterpause-rg` would make `tf:destroy` fail
halfway. It goes into `schmetterpause-dump-rg`, which is deleted afterwards.

### 1. Run the dump in Azure

From the repository root, logged in (`task tf:login`). The password is read
from `terraform/terraform.tfvars` and only ever written into a mode-`600` file
that is deleted as soon as the container exists — never onto a command line or
into output.

```sh
server=$(terraform -chdir=terraform output -raw postgres_fqdn)
az group create -n schmetterpause-dump-rg -l westeurope -o none

umask 077
pw=$(sed -n 's/^postgres_password *= *"\(.*\)".*/\1/p' terraform/terraform.tfvars)
cat > aci-dump.yaml <<EOF
apiVersion: '2021-10-01'
location: westeurope
name: schmetterpause-dump
type: Microsoft.ContainerInstance/containerGroups
properties:
  osType: Linux
  restartPolicy: Never
  containers:
    - name: dump
      properties:
        image: ghcr.io/cloudnative-pg/postgresql:18
        resources:
          requests:
            cpu: 1.0
            memoryInGB: 1.0
        environmentVariables:
          - name: PGHOST
            value: ${server}
          - name: PGUSER
            value: schmetterpause
          - name: PGDATABASE
            value: schmetterpause
          - name: PGSSLMODE
            value: require
          - name: PGPASSWORD
            secureValue: '${pw}'
        command:
          - /bin/sh
          - -c
          - |
            set -eu
            cat > /tmp/c.sql <<'SQL'
            select 'players', count(*) from players
            union all select 'identities', count(*) from identities
            union all select 'player_credentials', count(*) from player_credentials
            union all select 'matches', count(*) from matches
            union all select 'match_sets', count(*) from match_sets
            union all select 'ttr_history', count(*) from ttr_history
            union all select 'tournaments', count(*) from tournaments
            union all select 'tournament_players', count(*) from tournament_players
            union all select 'kiosk_grants', count(*) from kiosk_grants
            union all select 'goose max version', max(version_id) from goose_db_version where is_applied
            order by 1;
            select 'matches ' || status, count(*) from matches group by status order by 1;
            SQL
            echo "==COUNTS-BEFORE=="
            psql -X -A -t -F '|' -f /tmp/c.sql
            pg_dump --no-owner --no-acl -f /tmp/d.sql
            echo "==COUNTS-AFTER=="
            psql -X -A -t -F '|' -f /tmp/c.sql
            echo "==SHA256== \$(sha256sum /tmp/d.sql | cut -d' ' -f1)"
            echo "==BYTES== \$(wc -c < /tmp/d.sql)"
            echo "==BEGIN=="
            gzip -9 -c /tmp/d.sql | base64
            echo "==END=="
EOF
unset pw
az container create -g schmetterpause-dump-rg --file aci-dump.yaml -o none
rm -f aci-dump.yaml
```

The image tag is the server's major: `postgres_version` in
`terraform/schmetterpause.auto.tfvars`. `az container create` may print `None`
for the state; that only means it returned before the instance view was filled.

### 2. Wait for it, fetch the output

```sh
az container show -g schmetterpause-dump-rg -n schmetterpause-dump \
  --query 'containers[0].instanceView.currentState.{state:state, exit:exitCode}' -o tsv
# Terminated	0

umask 077
az container logs -g schmetterpause-dump-rg -n schmetterpause-dump --container-name dump > dump.log
```

On 2026-09-12 the image pull took 38 seconds and the dump itself six.

### 3. Decode and verify

```sh
umask 077
f="schmetterpause-azure-$(date +%Y-%m-%d-%H%M).sql"
sed -n '/^==BEGIN==$/,/^==END==$/p' dump.log | sed '1d;$d' | base64 -d | gunzip > "$f"
chmod 600 "$f"

[ "$(sed -n 's/^==SHA256== //p' dump.log)" = "$(sha256sum "$f" | cut -d' ' -f1)" ] && echo "sha256 ok"
[ "$(sed -n 's/^==BYTES== //p' dump.log)" = "$(wc -c < "$f")" ] && echo "size ok"

sed -n '/^==COUNTS-BEFORE==$/,/^==COUNTS-AFTER==$/p' dump.log | sed '1d;$d' > counts-before.txt
sed -n '/^==COUNTS-AFTER==$/,/^==SHA256==/p' dump.log | sed '1d;$d' > counts-after.txt
cmp -s counts-before.txt counts-after.txt && echo "nothing was written during the dump"
```

If the counts before and after differ, somebody entered a result while the dump
ran. Dump again rather than guessing which half is in the file.

### 4. Prove it restores

A dump is only a backup once it has been restored. Into a throwaway container
of the same major:

```sh
docker run -d --name sp-restore-probe \
  -e POSTGRES_USER=schmetterpause -e POSTGRES_PASSWORD=probe -e POSTGRES_DB=schmetterpause \
  postgres:18-alpine
until docker exec sp-restore-probe pg_isready -U schmetterpause >/dev/null 2>&1; do sleep 1; done

docker exec -i sp-restore-probe psql -U schmetterpause -d schmetterpause -X -q -v ON_ERROR_STOP=1 < "$f" \
  && echo "restore ok"

# The same queries the container ran.
cat > counts.sql <<'SQL'
select 'players', count(*) from players
union all select 'identities', count(*) from identities
union all select 'player_credentials', count(*) from player_credentials
union all select 'matches', count(*) from matches
union all select 'match_sets', count(*) from match_sets
union all select 'ttr_history', count(*) from ttr_history
union all select 'tournaments', count(*) from tournaments
union all select 'tournament_players', count(*) from tournament_players
union all select 'kiosk_grants', count(*) from kiosk_grants
union all select 'goose max version', max(version_id) from goose_db_version where is_applied
order by 1;
select 'matches ' || status, count(*) from matches group by status order by 1;
SQL
docker exec -i sp-restore-probe psql -U schmetterpause -d schmetterpause -X -A -t -F '|' \
  < counts.sql > counts-restored.txt
diff <(sort counts-before.txt) <(sort counts-restored.txt) && echo "counts identical"

docker rm -f sp-restore-probe
rm -f counts.sql
```

If a table was added by a migration since this page was written, add it to both
copies of the query.

**Compare sorted.** The Alpine image sorts text bytewise even though it reports
`en_US.utf8` — musl has no collation — while Flexible Server sorts by the
locale. `match_sets` and `matches` swap places, the numbers do not. The same
difference shows up anywhere the application orders by text, which is worth
knowing when a restored instance lists names in a different order.

### 5. Clean up, then tear down

```sh
az group delete -n schmetterpause-dump-rg --yes
rm -f dump.log counts-before.txt counts-after.txt counts-restored.txt
task tf:destroy
```

Keep the dump somewhere a person chooses — it is the only copy once the server
is gone.

## What travels, and what does not

- **Pending matches travel as pending.** Somebody has to confirm them on the
  next instance. Confirm them before dumping if that is easier.
- **Players sign in once.** A new instance is a new host, so no browser is
  recognised. PINs and recovery codes come along in the dump, and signing in
  with them works (ADR-0007) — `SP_SESSION_KEY` only matters behind a custom
  domain that stays the same.
- **The Flexible Server's own backups do not.** They are deleted with it.

## The run on 2026-09-12

The first teardown of the Azure instance, after a tournament on 2026-09-11.

| | |
| --- | --- |
| File | `schmetterpause-azure-2026-09-12-0549.sql`, 43,049 bytes |
| Source | PostgreSQL 17.11 on Flexible Server; `pg_dump` 17.11 from the CNPG image |
| Content | 7 players, 9 identities, 14 credentials, 34 matches (33 confirmed, 1 pending), 35 sets, 66 rating-history rows, 2 tournaments with 12 entries, migrations at `20260904120000` |
| Checks | checksum and size match, counts unchanged during the dump, restore into `postgres:17-alpine` without an error, sorted counts identical |

## Not covered yet

- **Restoring into Azure.** The same blocked port applies in the other
  direction, so a restore also has to run inside Azure — and it has to happen
  before the application's init container migrates the empty database, or the
  schema collides. #213.
- **Kubernetes.** A dump from CloudNativePG, and a restore into it, are #213 as
  well. `task office:backup` covers Compose and writes the same kind of file.
- **The office cluster's major.** Compose, the kcl default and Azure are on 18;
  the office cluster still runs 17. Its data moves to a new 18 cluster with the
  dump and restore tasks, which is what makes the majors equal everywhere.
  #213.
