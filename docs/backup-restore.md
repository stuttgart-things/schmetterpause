# Backing up and moving the game data

The rules are in [ADR-0016](adr/0016-azure-auf-zeit-daten-ziehen-um.md): the
Azure instance only ever runs temporarily, and the game data moves between
Compose, Kubernetes and Azure as a logical dump, one writer at a time.

This page is the how. Everything on it has been run: a full round trip
Kubernetes → Azure → Kubernetes → Compose on 2026-09-12, through the two tasks
below, with row counts compared at every station and a PIN sign-in on Azure and
on Compose. The record is at the end.

## The two tasks

```sh
task db:dump    ENV=compose|kubernetes|azure
task db:restore ENV=compose|kubernetes|azure FILE=schmetterpause-….sql
```

The logic is `scripts/db.sh`; `scripts/db-counts.sql` is what gets counted.

**A dump** writes `schmetterpause-<env>-<time>.sql` — a plain `pg_dump` with
`--no-owner --no-acl`, schema, data and `goose_db_version` — and next to it
`….sql.counts`, the row counts of every table. It counts before and after the
dump and refuses if the two differ: somebody entered a result while it ran.

**A restore** refuses a database that already has players. Otherwise it resets
the schema — the application's own migration has usually created empty tables
already — and loads the dump as the role the application connects as, in one
transaction, so a failure leaves the database as it was. Then it compares the
counts with the dump's `.counts`, sorted.

**Both files are sensitive.** They hold display names, PIN hashes and
recovery-code hashes. `schmetterpause-*.sql` and `schmetterpause-*.sql.counts`
in the repository root are gitignored and written with mode `600`. Never let
one become a CI artefact, and keep the one from a teardown somewhere a person
chooses — once the server is gone, it is the only copy.

## First: back up the office

The office plays on the local Compose stack — project `schmetterpause`, volume
`schmetterpause_pgdata`, shared by `task up` and `task office:up`. That volume
is the ranking; the Kubernetes cluster was only ever a trial. `task down`
deletes it.

Before any work that moves or could touch game data, dump it and prove the dump
restores — a backup nobody restored is not a backup:

```sh
task db:dump ENV=compose        # reads only; the app keeps running

# Probe-restore into a throwaway project with its own volume and no ports.
cat > /tmp/compose.probe.yaml <<'EOF'
services:
  db:
    ports: !reset []
  app:
    ports: !reset []
EOF
COMPOSE_PROJECT_NAME=sp-probe COMPOSE_FILE=compose.yaml:/tmp/compose.probe.yaml \
  docker compose up --detach --wait db
COMPOSE_PROJECT_NAME=sp-probe COMPOSE_FILE=compose.yaml:/tmp/compose.probe.yaml \
  task db:restore ENV=compose FILE=schmetterpause-compose-….sql     # "counts match"
COMPOSE_PROJECT_NAME=sp-probe COMPOSE_FILE=compose.yaml:/tmp/compose.probe.yaml \
  docker compose down --volumes
```

`down --volumes` only ever with the probe project named. Without the variables
it is the office.

`task office:backup` still writes a plain dump of the same database; `db:dump`
adds the counts that make a restore checkable.

## Compose

Whatever `docker compose` would address from the repository root: the project
named after the directory, or `COMPOSE_PROJECT_NAME` and `COMPOSE_FILE` when set.
The database container has to be running.

A restore stops the app first — it migrates on start and serves requests,
neither of which belongs in the middle of a restore — and starts it again
afterwards, also when the load fails.

## Kubernetes

Whatever kubeconfig and context `kubectl` uses. The task names them before it
does anything; check that line.

```sh
KUBECONFIG=~/.kube/homerun2-test1 task db:dump ENV=kubernetes
KUBECONFIG=~/.kube/homerun2-test1 task db:restore ENV=kubernetes NAMESPACE=… FILE=…
```

`NAMESPACE` defaults to `schmetterpause`, `DB_CLUSTER` to `schmetterpause-db`.
`pg_dump` and `psql` run inside the CloudNativePG primary over its local socket,
so no credential leaves the cluster. The load runs behind `SET ROLE <owner>`:
as `postgres`, every table would belong to `postgres` and the application could
not touch them.

A restore refuses while the application's Deployment has replicas. Scaling it is
a decision about that environment, so the task leaves it to you:

```sh
kubectl -n <namespace> scale deployment/schmetterpause --replicas=0
# restore
kubectl -n <namespace> scale deployment/schmetterpause --replicas=1
```

A throwaway cluster to restore into, the way the round trip made one:

```sh
kubectl create namespace schmetterpause-restore-test
task kcl:secrets NAMESPACE=schmetterpause-restore-test
task kcl:database PROFILE=existing-secrets -- -D config.namespace=schmetterpause-restore-test \
  | kubectl apply -f -
```

It came up healthy in under a minute. Delete the namespace afterwards; the
volume goes with it.

## Azure

Needs an applied instance (`task tf:apply`) and a live login (`task tf:login`).
The task reads the server, the major version and the credentials from the
Terraform state and the two `.tfvars` files.

```sh
task db:dump    ENV=azure                       # before task tf:destroy
task db:restore ENV=azure FILE=schmetterpause-….sql   # after task tf:apply
```

It does not connect from your machine. Both directions run in a one-off Azure
Container Instance, for three reasons found the hard way:

- **The office network blocks outbound port 5432.** A firewall rule on the
  server for your address does nothing against that.
  `curl -s --max-time 10 http://portquiz.net:5432/ >/dev/null && echo open || echo blocked`
  shows it.
- **Docker Hub refuses anonymous pulls from Azure** (`RegistryErrorResponse`).
  The container runs `ghcr.io/cloudnative-pg/postgresql:<major>` instead — the
  image the kcl module runs the database with, at the server's major.
- **It gets its own resource group,** `<name_prefix>-dbjob-rg`. `azurerm` refuses
  to delete a resource group that holds resources it does not manage
  (`prevent_deletion_if_contains_resources` defaults to `true`), so anything
  left in `schmetterpause-rg` would break `tf:destroy`. The job group is deleted
  whenever the task ends, successful or not. If a run is killed hard and the
  group survives, the next run says so and stops.

The container reaches the server through the existing "allow Azure services"
rule. A dump comes back through `az container logs`, gzipped and base64-encoded
between markers with a sha256 that the task checks. A restore goes in through an
Azure Files share in the job group, uploaded over HTTPS. The password and the
storage key only ever sit in a mode-`600` file in a private temporary directory,
deleted as soon as the container exists.

After a restore the task restarts the application's revision, so its init
container runs `migrate up` against the restored schema — which matters as soon
as the image is newer than the dump.

On 2026-09-12 a dump took 156 seconds and a restore 119, most of it pulling the
image and creating the storage account.

## Versions

A dump only ever goes into the **same or a newer** PostgreSQL major, and the
same or a newer application version. The target is 18: Compose, the kcl default
(#221) and Azure (#224) are on it. The CloudNativePG cluster on homerun2-test1
still runs 17; a dump from it restores into any of the others, and nothing goes
back into it.

## What travels, and what does not

- **Players sign in once.** A new environment is a new host, so no browser is
  recognised. PINs and recovery codes come along in the dump, and signing in
  with them works — checked on Azure and on Compose. `SP_SESSION_KEY` only
  matters behind an address that stays the same.
- **Pending and disputed matches travel as they are.** Somebody still has to
  confirm them on the other side.
- **Flexible Server's own backups do not.** They are deleted with the server.
- **Row order can differ.** The Alpine images sort text bytewise even though
  they report `en_US.utf8` — musl has no collation — while Flexible Server
  sorts by locale. That is why the counts are compared sorted, and it is worth
  knowing when a restored instance lists names in a different order.

## The round trip on 2026-09-12

The data was the tournament from 2026-09-11 — the office Kubernetes cluster has
no PINs, so it could not show a sign-in. Every arrow is one `task db:dump` or
`task db:restore`.

| Station | PostgreSQL | Result |
| --- | --- | --- |
| Tournament dump from the first Azure teardown | 17.11 | taken by hand that morning, 43,049 bytes |
| → Kubernetes, fresh test cluster | 18.6 | restored, counts match |
| Kubernetes → dump | 18.6 | 43,171 bytes, counts equal before and after |
| → Azure, fresh instance | 18 | restored in 119 s, revision restarted, healthy |
| Azure → dump | 18.6 | 43,146 bytes in 156 s, checksum ok |
| → Kubernetes, a second fresh test cluster | 18.6 | restored, counts match |
| Kubernetes → dump → Compose, app running | 18.6 | app stopped, restored, counts match, app started, healthy |
| PIN sign-in | | on Azure and on Compose, `player signed in` in both logs |

The sorted counts hash to the same value at every station: 7 players,
9 identities, 14 credentials, 34 matches (33 confirmed, 1 pending), 35 sets,
66 rating-history rows, 2 tournaments with 12 entries, migrations at
`20260904120000`.

Before it started, the office Compose database was dumped and probe-restored:
15 players, 66 matches, counts matching. The round trip never touched it.

## Not covered

- **Emptying a database that has players.** There is no task for it, on
  purpose: a restore replaces everything, and the moment somebody wants that for
  the office is a decision to make by hand, after a verified backup.
- **The Kubernetes cluster's major.** homerun2-test1 still runs 17. It holds no
  office data, so whether it moves to 18 or goes away is open.
