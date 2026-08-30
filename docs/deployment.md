# Deploying without GitOps

The intended path onto a cluster is an ArgoCD Application that consumes the OCI
artefact `ghcr.io/stuttgart-things/schmetterpause-kustomize` and patches it per
environment — that is issue #81. This document describes the other path: the
same manifests straight onto a cluster with `task` and `kubectl`, no Argo in
between.

It is meant for a test cluster, for the first bring-up of a new environment, and
for fault-finding when it is unclear whether a problem comes from the
application or from Argo. It is not meant for continuous operation: there is no
drift detection and no prune.

The examples use `cicd-test2` throughout. Replace the cluster domain, the
gateway name and the store name with the target environment's.

## What the cluster has to bring

Three things, plus one that depends on the secrets variant you pick.

```sh
# 1. CloudNativePG operator
kubectl -n postgres get deploy cloudnative-pg

# 2. A programmed gateway with a TLS listener
kubectl -n default get gateway cilium-gateway
kubectl -n default get gateway cilium-gateway \
  -o jsonpath='{range .spec.listeners[*]}{.name}  {.hostname}  {.tls.certificateRefs[*].name}{"\n"}{end}'

# 3. Does the gateway accept routes from other namespaces?
kubectl -n default get gateway cilium-gateway \
  -o jsonpath='{range .spec.listeners[*]}{.name}  {.allowedRoutes.namespaces.from}{"\n"}{end}'

# 4. A StorageClass for Postgres
kubectl get storageclass
```

On 3: the Gateway API default is `Same`. Unless that says `All` or carries a
matching selector, the gateway will not accept the HTTPRoute from the
application's namespace — visible only on the route, not on the gateway.

**The External Secrets Operator is not a prerequisite.** It is the more
comfortable of two paths, not the only one; the next section puts both side by
side.

## Secrets: two paths

The application needs two Secrets, and they are named the same under either
variant, because the Deployment reads them by name:

| Secret | Keys | Read by |
| --- | --- | --- |
| `schmetterpause-app` | `SP_SESSION_KEY`, optionally `SP_KIOSK_TOKEN` | the application |
| `schmetterpause-db` | `username`, `password`, `SP_DATABASE_URL` | CloudNativePG and the application |

`schmetterpause-db` is of type `kubernetes.io/basic-auth` and does double duty:
CloudNativePG reads the owner credentials from it at `initdb`, the application
reads `SP_DATABASE_URL`. Both coming from the same Secret is why the role and
the DSN cannot drift apart.

Two Secrets rather than one is least privilege: the migration initContainer gets
only the database Secret and never sees the session key.

Changing `SP_SESSION_KEY` is not a restart, it is logging every player out. The
test `TestARestartWithADifferentKeyForgetsEverybody` records that.

### Variant A — External Secrets

The path for a cluster that has a `ClusterSecretStore` anyway. Both Secrets are
then produced from one Vault entry and refreshed on a schedule; the repository
and the command line hold only a path, never a value.

```sh
kubectl get crd | grep externalsecrets.external-secrets.io
kubectl get clustersecretstore
```

The store is named `vault-<cluster>`, never plain `vault`. `kubectl get
clustersecretstore` is the reliable source. A store reporting `Ready=True`
proves the login works — not that the policy may read the entry. That only
shows on the ExternalSecret.

The entry sits under the mount the store itself carries, and is named
`schmetterpause`:

| Key | Contents | Form |
| --- | --- | --- |
| `session-key` | key for the session cookie | 32 bytes, base64 |
| `username` | the database's owner role | `schmetterpause` |
| `password` | that role's password | hex, no special characters |
| `kiosk-token` | only when the kiosk is on | hex |

`password` and `kiosk-token` are deliberately hex rather than base64: the
password is interpolated into a DSN unescaped, and an `@`, `/`, `:` or `?` in it
makes the URL parse differently. The session key passes through no URL and may
be base64.

`remoteRef.key` holds the entry name and nothing else. The store already carries
the mount and `version: v2`, and ESO composes `<mount>/data/<key>` from them.
Both ways of getting this wrong point at something that exists nowhere, and
report nothing at apply time:

```
schmetterpause                    correct
schmetterpause/data/cicd-test2    -> cicd-test2/data/schmetterpause/data/…
schmetterpause-cicd-test2         -> the cluster twice; the mount IS the cluster
```

### Variant B — ordinary Kubernetes Secrets

The path for a cluster without ESO. One command creates both Secrets, generates
the values, and assembles the DSN the way the ESO template would:

```sh
kubectl create ns schmetterpause
task kcl:secrets NAMESPACE=schmetterpause
```

The task creates and does not update. A second call aborts rather than setting a
new session key — that would not be a failure anyone sees, it would be one where
everybody is logged out the next morning.

With the kiosk: `task kcl:secrets NAMESPACE=schmetterpause KIOSK=true`.

Doing it by hand works the same way when the values come from somewhere else.
All that matters is that the password inside `SP_DATABASE_URL` is the same one
as under `password`, and that it contains no character that takes a URL apart.

From here on only the profile differs: **variant A renders with `PROFILE=base`,
variant B with `PROFILE=existing-secrets`.**

## 1. Postgres operator

Once per cluster, not per application:

```sh
helmfile apply \
  --file 'git::https://github.com/stuttgart-things/helm.git@database/postgres.yaml.gotmpl' \
  --state-values-set namespace=postgres \
  --state-values-set version=0.29.0
```

## 2. Namespace

The base deliberately renders none. Several applications can share a namespace,
and when more than one ships it as a resource, ArgoCD reports a SharedResource —
and then one application's prune cycle deletes the other's namespace.

```sh
kubectl create ns schmetterpause
```

## 3. Applying the manifests

```sh
task kcl:apply -- \
  -D config.image=ghcr.io/stuttgart-things/schmetterpause:3382ad1 \
  -D config.clusterDomain=cicd-test2.4sthings.tiab.ssc.sva.de \
  -D config.gatewayName=cilium-gateway \
  -D config.secretStoreName=vault-cicd-test2 \
  -D config.vaultPath=schmetterpause
```

For variant B, without the last two lines and with the other profile:

```sh
task kcl:apply PROFILE=existing-secrets -- \
  -D config.image=ghcr.io/stuttgart-things/schmetterpause:3382ad1 \
  -D config.clusterDomain=cicd-test2.4sthings.tiab.ssc.sva.de \
  -D config.gatewayName=cilium-gateway
```

Everything after `--` is passed to `kcl`, behind the profile's own values, and a
later `-D` wins. The profile supplies the ground, the command line the handful
of values that make this environment.

**Rendering without a profile is the trap.** `kcl run kcl/main.k -D …` loads no
profile; every field not passed then falls back to its default in `schema.k`
rather than the profile's. `httpRouteEnabled` is `false` there and `secretsMode`
is `external` — both deliberately reticent, and neither what a cluster wants.
Hence `task kcl:apply` rather than `kcl run`.

`task kcl:render` shows the same thing without applying it.

For variant A, check the secrets before anything else:

```sh
kubectl -n schmetterpause get externalsecret
```

Both have to report `SecretSynced`. The pod is not running at this point — that
is correct, the database is still missing.

## 4. Postgres cluster

Only now, because CloudNativePG reads the owner password from
`schmetterpause-db` at `initdb`, and that has to be in place first — in variant
A as soon as the ExternalSecret has synced, in variant B since `task
kcl:secrets`.

```sh
helmfile apply \
  --file 'git::https://github.com/stuttgart-things/helm.git@database/postgres-cluster.yaml.gotmpl' \
  --state-values-set namespace=schmetterpause \
  --state-values-set version=0.8.1 \
  --state-values-set clusterName=schmetterpause-db \
  --state-values-set database=schmetterpause \
  --state-values-set owner=schmetterpause \
  --state-values-set appSecretName=schmetterpause-db
```

The four domain values hang together:

- `clusterName=schmetterpause-db` produces the Service `schmetterpause-db-rw`
  that the DSN looks for. It matches `config.dbClusterName` in the KCL module.
- `owner=schmetterpause` has to match `username` in the Secret, otherwise CNPG
  creates a role the DSN never uses.
- `database=schmetterpause` matches `config.dbName`.
- `appSecretName=schmetterpause-db` is the Secret created above. Without it CNPG
  invents a password of its own and the DSN no longer fits.

Everything else comes from the template's defaults: one instance, PostgreSQL 17,
8Gi, the default StorageClass, no backups, no superuser access. Why one instance
and no backups: replicas and backups protect against different things, and the
realistic danger on a test cluster is a deliberate rebuild, against which a
replica does nothing. The trigger for changing this is backups, not replicas.

The PVC appears only with the pod when the StorageClass is
`WaitForFirstConsumer`.

Once the cluster is up, the migration initContainer gets through on its next
backoff by itself. No intervention is needed; anyone impatient deletes the pod.

## 5. Checking

```sh
kubectl -n schmetterpause get cluster
kubectl -n schmetterpause get po
kubectl -n schmetterpause get httproute schmetterpause -o yaml | sed -n '/^status:/,$p'

curl -sSI https://schmetterpause.cicd-test2.4sthings.tiab.ssc.sva.de/healthz   # 200
curl -sSI http://schmetterpause.cicd-test2.4sthings.tiab.ssc.sva.de/           # 301
```

## When something does not work

What the failures on this path have in common is that none of them look like
failures.

**An ExternalSecret reports `SecretSyncedError` and everything else stays
green.** Variant A only. Usually a property name the Vault entry does not have.
The missing name is in `.status.conditions[].message`. It is visible only on the
resource itself — pods, store and gateway say nothing about it.

**The HTTPRoute has no status at all and the host answers 404.** Not
`Accepted=False`, but an empty `.status.parents`. Then the `parentRef` points at
a Gateway that does not exist — typically because `namespace` is missing, so the
route's own namespace is meant. `schema.k` now rejects an empty
`gatewayNamespace`; if the route is still statusless, the name or the
`sectionName` is wrong.

**The initContainer fails with `no such host`.** The database is missing or
named differently. The message is useful anyway: if it says
`user=schmetterpause database=schmetterpause`, the DSN parsed cleanly, so the
password contains no character that takes the URL apart and only the name is
really missing.

**The pod will not start: `secret "schmetterpause-db" not found`.** The order is
wrong — the Secret has to exist before the Deployment. In variant A that is a
matter of seconds and resolves itself; in variant B it means `task kcl:secrets`
was skipped.

## Tearing down

```sh
helmfile destroy --file 'git::https://github.com/stuttgart-things/helm.git@database/postgres-cluster.yaml.gotmpl' \
  --state-values-set namespace=schmetterpause --state-values-set clusterName=schmetterpause-db
kubectl delete ns schmetterpause
```

The Postgres cluster's PVC hangs off the namespace and goes with it. With a
StorageClass whose `reclaimPolicy` is `Delete` — `openebs-hostpath`, for
instance — the data is then gone. On a test cluster that is intended; anywhere
else it is not.
