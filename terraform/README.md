# Azure Container Apps

Terraform for running Schmetterpause without a cluster: the same image as on
Compose and Kubernetes, on Azure Container Apps, against Azure Database for
PostgreSQL Flexible Server.

This is the third definition of how the application is run, beside
`compose.yaml` and the kcl module in [`kcl/`](../kcl/README.md), and all three
describe one contract. Read [Kept in step with kcl](#kept-in-step-with-kcl)
before changing anything here or there.

## What it stands up

Everything lives in one resource group, `<name_prefix>-rg`.

| Resource | Notes |
| --- | --- |
| Log Analytics workspace | container logs, 30 days |
| Container Apps environment | |
| PostgreSQL Flexible Server | Burstable B1ms, version 17, TLS required, 7 days of backups, public access through the "allow Azure services" rule |
| Container app `<name_prefix>-app` | external HTTPS, exactly one replica, `migrate up` as init container, liveness on `/healthz`, readiness on `/readyz` |

No Redis (invariant 3).

## Prerequisites

The Azure CLI. Terraform authenticates through its login session, so a manual
apply needs no service principal:

```sh
task tf:login                          # TENANT=<tenant-id> for a specific tenant
task tf:login -- --use-device-code     # on a machine without a browser
```

It also makes `subscription_id` from `terraform.tfvars` the CLI's default, when
that file exists.

`tf:plan`, `tf:apply` and `tf:destroy` refuse to start without the CLI, without
`terraform.tfvars`, with an expired session, or when the session cannot see
`subscription_id` — each with a message saying which. `tf:init`, `tf:output`
and `tf:check` need no login.

Once per subscription, logged in:

```sh
az provider register -n Microsoft.App --wait
az provider register -n Microsoft.OperationalInsights --wait
az provider register -n Microsoft.DBforPostgreSQL --wait
```

The image `ghcr.io/stuttgart-things/schmetterpause` is public, so the app needs
no registry credentials.

## Deploying

```sh
cp terraform/terraform.tfvars.example terraform/terraform.tfvars   # fill it in

task tf:login
task tf:init
task tf:plan
task tf:apply
task tf:output
```

Extra flags go after `--`, e.g. `task tf:apply -- -auto-approve`.

Checking:

```sh
url=$(terraform -chdir=terraform output -raw app_url)
curl -sSI "${url}/healthz"   # 200
curl -sSI "${url}/readyz"    # 200 once the database answers
```

`task tf:check` runs `terraform fmt`, `terraform validate` and the tests in
`tests/`, and needs no Azure login. The tests mock the provider, so they check
this configuration's own logic — the variable rules, and that the environment
handed to the app matches what kcl renders — not what Azure would accept.

A first apply creates the server before the app, and the app's first revision
waits on the init container, which migrates the empty database. There is
nothing to run by hand.

## Values

Three are required and have no default.

**`subscription_id`** — where it goes.

**`session_key`** — signs the recognition cookie. Generate it once and keep it:

```sh
openssl rand -base64 32
```

A new key is not a restart, it logs every player out. Losing
`terraform.tfvars` and generating a fresh one is the same thing.

**`postgres_password`** — letters and digits only, with at least one upper-case
letter, one lower-case letter and one digit:

```sh
openssl rand -base64 48 | tr -dc 'A-Za-z0-9' | cut -c1-40
```

This is not the hex that `task kcl:secrets` generates for a cluster, and the
difference is forced. The password is interpolated into `SP_DATABASE_URL`
unescaped, so it must not contain anything that takes a URL apart — which hex
satisfies. Flexible Server additionally insists on three of four character
classes, which lowercase hex does not. Mixed-case alphanumerics satisfy both,
and the variable's validation says so before Azure does.

The rest are optional:

| Variable | Default | Meaning |
| --- | --- | --- |
| `kiosk_token` | *(empty)* | `SP_KIOSK_TOKEN`; empty means there is no kiosk. `openssl rand -hex 32` |
| `bootstrap_admin` | *(empty)* | `SP_BOOTSTRAP_ADMIN`; the player has to have joined, so this is a second apply |
| `log_level` | `info` | `SP_LOG_LEVEL` |
| `public_base_url` | *(generated address)* | `SP_PUBLIC_BASE_URL`; set it once a custom domain is bound |
| `extra_env_vars` | `{}` | plain variables with no dedicated setting yet; never secrets |
| `image` | pinned release | Renovate moves the default |
| `min_replicas` | `1` | `0` scales to zero between games, see below |
| `location` | `westeurope` | has to be allowed by the subscription's policy |
| `name_prefix` | `schmetterpause` | changing it replaces every resource, the database included |
| `postgres_user` / `postgres_db` | `schmetterpause` | |
| `postgres_version` / `postgres_sku` | `17` / `B_Standard_B1ms` | |

### `min_replicas`

`1` keeps a replica running whether anybody plays or not. `0` saves that, at the
price of a cold start — image pull, init container, migration check — on the
first request after a quiet period. The sign-in rate limit lives in memory and
is forgotten on scale-in; docs/adr/0007 accepts a restart forgetting who was
being slowed down, since it errs towards letting people in.

It never goes above 1. `max_replicas` is not a variable, for the same reason
`config.replicas` has a `check:` in kcl.

## Kept in step with kcl

The Terraform must not become a second description of the application that
quietly drifts from the first. Every setting here mirrors one in
`kcl/schema.k`, and `variables.tf` names the field above each variable.

| kcl | Terraform | |
| --- | --- | --- |
| `image` | `image` | pinned, never `latest` |
| `replicas == 1` (check) | `max_replicas = 1` | goose takes no session lock |
| `migrateInitContainer` | init container `migrate up`, `SP_AUTO_MIGRATE=false` | it gets `SP_DATABASE_URL` and nothing else |
| `logLevel` | `log_level` | same four values |
| `publicBaseURL` | `public_base_url` | derived when empty; TLS terminates in front of the app |
| `kioskEnabled` + `kiosk-token` | `kiosk_token` | |
| `bootstrapAdmin` | `bootstrap_admin` | |
| `session-key` | `session_key` | |
| `dbOwner` / `dbName` / `dbSSLMode` | `postgres_user` / `postgres_db` / `sslmode=require` | |
| `password` | `postgres_password` | alphanumeric rather than hex, see above |
| `dbImage` | `postgres_version` | |
| `extraEnvVars` | `extra_env_vars` | applied last, so they override |
| `SP_COOKIE_SECURE` absent | absent | defaults to true in the code |

**The rule:** a change to the application's runtime surface — a new, renamed or
removed `SP_*` variable in `internal/config/config.go`, a new secret, a changed
default, probe path, port or migration rule — lands in `kcl/` and `terraform/`
in the same pull request. For a new variable that means `schema.k` plus
`configmap.k` or `externalsecret.k` on one side, `variables.tf` plus the
`app_env` local or a `secret` block in `main.tf` on the other.

### Where it cannot be the same

**Rollover.** kcl's Deployment uses `Recreate`: the old pod is gone before the
new one runs `migrate up`. Container Apps has no such strategy. In single-
revision mode the new revision starts, migrates and takes traffic while the old
one still serves, against the already-migrated schema. That is safe only
because migrations are forward and additive (invariant 8), so the old code
still finds every table and column it knew. A destructive migration, which
needs a separate step anyway, needs the app stopped first here — there is no
`Recreate` to lean on.

**Resources.** kcl sets requests and limits. Container Apps allows only fixed
CPU/memory pairs, so the app and the init container get 0.25 vCPU and 0.5 GiB
each, a valid allocation whether or not Azure counts the init container.

**Database storage.** 32 GiB, the smallest Flexible Server offers, where
`database.k` defaults to 8 Gi.

**No demo field.** `seedEnabled` has no counterpart: it exists for preview
environments, and those live on the cluster.

## Tearing down

This instance is only ever run temporarily — stood up for a test or an
occasion, then removed (docs/adr/0016). Tearing down is the normal end of its
life, not an exception.

**Dump the database first.** `destroy` removes the resource group and with it
the database, its data and Flexible Server's own backups — those cannot be
taken along. The dump is what the next `apply` restores, into Azure or into
another environment; the tasks for both are #213. Until they exist, a dump from
Azure needs a firewall rule for the machine running `pg_dump`, because the
only rule here admits Azure services and nothing else.

```sh
task tf:destroy
```

Changing `name_prefix` replaces the server and has the same effect.

## Not yet

- **Private networking.** The "allow Azure services" rule opens the Postgres
  port to every Azure tenant, with only the password in the way. Fine for a
  test, not for keeping; the step after is VNet integration with private access.
- **Remote state.** The state is a local file and holds the session key and the
  database password in plain text. One operator at a time until it moves to an
  `azurerm` backend.
- **Automated deploys.** Applies are manual. A workflow with OIDC federated
  credentials would replace them.
- **A custom domain.** Point a DNS-only CNAME at the app's address, bind it with
  a managed certificate, then set `public_base_url`. The generated
  `*.azurecontainerapps.io` host already has valid TLS, so none of this blocks a
  test.
- **Moving data across.** `pg_restore --no-owner --no-acl` against
  `terraform output -raw postgres_fqdn`. The administrator on Flexible Server is
  not a superuser, which is why ownership and grants are left out.

## Related

- Issue #206 — what was decided here and why
- [`docs/deployment.md`](../docs/deployment.md) — the cluster without GitOps
- [`kcl/README.md`](../kcl/README.md) — the module this mirrors
- `docs/adr/0001` — Postgres, and the managed option on Azure
