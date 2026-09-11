# Platform requirements

What a Kubernetes cluster has to bring before schmetterpause makes sense on it.
[Deployment](deployment.md) describes how to put the application onto a cluster
and [`kcl/README.md`](https://github.com/stuttgart-things/schmetterpause/blob/main/kcl/README.md)
how the manifests are rendered. This page is the step before both: the
platform they assume.

Everything here falls into one of three tiers:

- **Required** — without it the application does not run, or runs in a way
  nobody can use.
- **Recommended** — the application runs without it, but only as a test: no
  way to rebuild, rotate or restore without manual work.
- **Optional** — makes operating it more comfortable; nothing depends on it.

The versions under *Tested with* are what the reference cluster ran on
2026-09-10 (see [Reference platform](#reference-platform)). They are
measurements, not minimums. Where a real floor exists, the section says where it
comes from.

## At a glance

| Capability | Tier | What schmetterpause uses it for | Tested with |
| --- | --- | --- | --- |
| Kubernetes | required | — | 1.35.3 (RKE2), linux/amd64 |
| PostgreSQL 14–18 with TLS | required | all state | PostgreSQL 18 |
| A StorageClass (RWO) | required for an in-cluster database | the database volume | openebs-hostpath 4.6.0 |
| Gateway API with an HTTPS listener | required for the rendered exposure | reaching the application | Gateway API v1.4.1 on Cilium 1.19.3 |
| DNS and a TLS certificate for the hostname | required | the `Secure` session cookie | cert-manager 1.21.1, Vault PKI wildcard |
| CloudNativePG operator | recommended | the database's lifecycle | 1.30.0 |
| External Secrets Operator and a store | recommended | the two Secrets | ESO 2.10.0 against OpenBao |
| Barman Cloud plugin and object storage | recommended | database backups and point-in-time restore | plugin v0.15.0, MinIO |
| Velero | optional | backups of the Kubernetes objects | 1.18.1 |
| trust-manager | optional | a CA bundle for a privately signed object store | 0.24.0 |
| GitOps (Argo CD or Flux) | optional | keeping the cluster at what Git says | Flux |

## Kubernetes

**The application itself asks for nothing version-specific.** It is a
Deployment, a Service, a ServiceAccount and a ConfigMap on stable APIs, and the
image is published for `linux/amd64` and `linux/arm64`.

**The floor comes from the operators around it.** CloudNativePG 1.30 supports
Kubernetes 1.34, 1.35 and 1.36 (tested upstream on 1.31–1.33 and 1.37), and
both its Helm chart and the Barman Cloud plugin's refuse anything below 1.29.
A CloudNativePG release is supported for about six months, so in practice: run
a Kubernetes version the CloudNativePG release you install still supports, and
move both together.

**Pod security.** The pod spec already satisfies the `restricted` Pod Security
Standard: non-root (UID and GID 65532), `seccompProfile: RuntimeDefault`, all
capabilities dropped, no privilege escalation and a read-only root filesystem.
No hostPath, no host network.

**Resources.** The application requests 50m CPU and 64Mi memory and is limited
to 500m and 256Mi (`kcl/schema.k`). The database is separate: a CloudNativePG
instance gets no requests unless the Cluster sets them.

**One replica, on purpose.** Migrations run through goose's package-level API,
which takes no lock, so a second pod migrating at the same time is unsafe;
`schema.k` refuses `replicas` other than 1. There is nothing to autoscale and no
PodDisruptionBudget to set. Draining the node means a short outage. A
one-instance database has its own drain behaviour — CloudNativePG documents
node maintenance for it, and it is worth reading before the first drain rather
than during it.

**What the pod talks to.** Only the database. The one HTTP client call in the
binary is the container health check against its own `/readyz` on loopback;
there is no external identity provider and no outbound API.

**Operational surface.** Logs are JSON on stdout. `SIGTERM` drains in-flight
requests for `SP_SHUTDOWN_TIMEOUT` (15s by default). The liveness probe is
`/healthz` and the readiness probe `/readyz`, which checks the database. There
is no metrics endpoint.

## Exposure: Gateway API

The kcl module renders exposure as Gateway API and only as Gateway API:

- an `HTTPRoute` (`gateway.networking.k8s.io/v1`) attached to the listener named
  by `gatewaySectionNameHTTPS` (default `https`),
- a second `HTTPRoute` on the plain listener (`gatewaySectionNameHTTP`, default
  `http`) that answers with a redirect to https.

That needs the Gateway API **standard channel** CRDs at v1.0 or later (v1 of
`HTTPRoute` and its `RequestRedirect` filter) and an implementation that
programs them.

**The Gateway has to offer three things.**

1. **An HTTPS listener with a certificate for the hostname.** TLS ends at the
   Gateway; a wildcard for the cluster domain works. The route attaches to that
   listener by name, and the name matters: attached to both listeners, the plain
   one serves the application too, the browser never sends the `Secure` session
   cookie back over it, and every player is forgotten between two clicks
   (issue #70).
2. **Routes from the application's namespace.** `allowedRoutes.namespaces.from`
   defaults to `Same`. Unless it is `All` or a selector that matches, the route
   is rejected — visible on the route's status, not on the Gateway.
3. **An HTTP listener, if the redirect is wanted.** Without one, set
   `httpRedirectEnabled: false`.

**DNS.** `<name>.<clusterDomain>` (or `host`) has to resolve to the Gateway.

**Behind the Gateway the application sees plain HTTP.** Set
`SP_PUBLIC_BASE_URL` — the module derives it from the hostname — or the printed
QR sheet points phones at a cluster-internal address.

**Without Gateway API** render with `httpRouteEnabled: false` and expose the
Service some other way (Ingress, a LoadBalancer in front of a proxy). One thing
does not change: the browser has to reach the application over HTTPS, because
`SP_COOKIE_SECURE` defaults to `true` and a cookie that is only sent over HTTPS
never comes back over HTTP.

## Storage

**The application stores nothing on disk.** Its root filesystem is read-only and
it mounts no volume.

**The database needs one `ReadWriteOnce` volume.**

- `dbStorageClass` empty means the cluster's **default** StorageClass. A cluster
  without a default class leaves the volume `Pending` — set one, or name the
  class.
- `WaitForFirstConsumer` is fine; the volume appears with the database pod.
- Size: 8Gi in `database.k`, 1Gi in the Flux bundle. A class with
  `allowVolumeExpansion: true` lets it grow without a restore.
- **`reclaimPolicy: Delete` plus a deleted namespace is a deleted database.**
  With a node-local class such as `openebs-hostpath` the data is also bound to
  one node. On either, backups are not a nice-to-have but the only copy.

## Database

**PostgreSQL 14–18, reachable over TLS.** The schema uses nothing beyond core
PostgreSQL (`gen_random_uuid()` is built in). The range is what CloudNativePG
1.30 supports. The application is built and tested against **PostgreSQL 18**
(`compose.yaml`, the Dagger pipeline), and `database.k` renders
`postgresql:18` by default. A database still on 17 pins `dbImage` in its
profile: CloudNativePG treats a higher major in `imageName` as an offline
in-place upgrade (`pg_upgrade`).

The DSN carries `sslmode=require`, so the server has to offer TLS.
CloudNativePG does by default.

**The contract with the database** (see [Deployment](deployment.md#secrets-two-paths)):

- the owner role equals `username` in the database Secret,
- the database is `dbName` (default `schmetterpause`),
- the host is `<dbClusterName>-rw.<namespace>.svc` unless `dbHost` says
  otherwise,
- the password contains no character that takes a URL apart — it is
  interpolated into the DSN unescaped.

**CloudNativePG is recommended, not required.** Any PostgreSQL the pod can reach
works through `SP_DATABASE_URL`; the Azure Container Apps path uses a managed
server. What the operator adds is a database that is declared next to the
application: bootstrapped from the same Secret the DSN comes from, TLS without
extra work, and the backup integration below.

**Order is not load-bearing.** A CloudNativePG Cluster applied before its
`initdb` Secret exists waits in `Setting up primary` and bootstraps about 30
seconds after the Secret appears (measured on CloudNativePG 1.30). ExternalSecrets
syncing a few seconds late do not break a bring-up.

**Keep the database out of prune.** The volume hangs off the Cluster object, so a
GitOps tool that prunes the Cluster deletes the data. `database.k` sets Argo CD's
`Prune=false,Delete=false`; the Flux bundle puts
`kustomize.toolkit.fluxcd.io/prune: disabled` on the Cluster *and* its
namespace, because pruning the namespace would take the Cluster with it.

## Secrets

The application reads two Secrets by name — `schmetterpause-app` and
`schmetterpause-db`. [Deployment](deployment.md#secrets-two-paths) lists their
keys and the two ways to produce them.

**The External Secrets Operator is recommended, not required.** Without it,
render with the `existing-secrets` profile and create the Secrets once
(`task kcl:secrets`). With it:

- ESO has to serve `external-secrets.io/v1`, which it does from 0.16 on (0.17
  and later serve nothing else). On an older operator set
  `externalSecretApiVersion: external-secrets.io/v1beta1`.
- A `ClusterSecretStore` or `SecretStore` whose policy may read the entry. A
  store reporting `Ready` proves the login, not the read — that shows only on
  the ExternalSecret.

## Backup and restore

Two different things are worth keeping, and they need two different tools:

| What | Tool | Without it |
| --- | --- | --- |
| The rows in the database — players, matches, rankings | CloudNativePG with the Barman Cloud plugin, to object storage | a lost volume is a lost league |
| The Kubernetes objects around it | Velero | nothing, under GitOps: Git renders them again |

### The database: Barman Cloud plugin

Continuous WAL archiving plus a scheduled base backup, so a restore can reach
any point after the oldest base backup the retention keeps.

**What the platform has to provide:**

- **The plugin**, in the operator's namespace — the operator discovers it by a
  label on its Service. Its chart needs **cert-manager** for the TLS between
  operator and plugin, and adds the `ObjectStore` CRD.
- **An S3-compatible bucket** and a credential scoped to it.
- **The endpoint's CA, as a Secret**, if the object store is privately signed.
  `ObjectStore.spec.configuration.endpointCA` only takes a Secret; trust-manager
  publishes its bundle as a ConfigMap. An ExternalSecret can carry both, the
  credential from the store and the CA copied from the ConfigMap with
  `templateFrom`.

**Use the plugin, not `spec.backup.barmanObjectStore`.** CloudNativePG 1.30
warns on every Cluster that uses the in-tree Barman Cloud support that it is
removed in 1.31.0.

**Give it its own bucket.** Velero treats unknown top-level directories in its
bucket as an invalid store; sharing one breaks the Velero backups.

**Recovery is a new Cluster**, bootstrapped from the object store under another
name, never an in-place overwrite. Measured on the reference platform: ready in
51–82 seconds for this database, rows written after the last base backup
included through WAL replay. The manifest is in the Flux bundle's
[`schmetterpause-db-backup` README](https://github.com/stuttgart-things/flux/blob/main/apps/tabletennis/components/schmetterpause-db-backup/README.md).

**Two traps, both paid for:**

- Switching an *existing* Cluster to the plugin restarts its pod. A
  `ScheduledBackup` with `immediate: true` fires into that restart and fails
  (`instance manager was restarted during backup`), and the archiver counts
  failures until the new pod is up. Take one manual backup afterwards. A Cluster
  created with the plugin from the start is not affected.
- The backup Secret holds a full CA bundle and is large. Copying it with
  `kubectl apply` fails on the `last-applied-configuration` annotation's
  262144-byte limit; use `kubectl create`.

### The Kubernetes objects: Velero

**Optional.** Without a node agent or CSI snapshots Velero backs up objects
only: the Deployment, the ExternalSecrets, the Cluster *definition* — not the
rows. Under GitOps those come back from Git anyway. Velero earns its place for
state applied by hand, and for bringing a whole namespace back in one step.

## Reference platform

`labda-dev-a`, a single-node LabDA cluster built by the
[stuttgart-things Flux bundle](https://github.com/stuttgart-things/flux) with
the `tabletennis-backup`, `cnpg-operator` and `cnpg-barman-cloud` components.
State on 2026-09-10:

| Component | Version |
| --- | --- |
| Kubernetes | v1.35.3+rke2r2, containerd 2.2.2, Ubuntu 26.04, amd64 |
| Gateway API | v1.4.1, standard channel |
| Gateway implementation | Cilium 1.19.3 |
| CloudNativePG | 1.30.0 (chart 0.29.0) |
| PostgreSQL | 18 (`ghcr.io/cloudnative-pg/postgresql:18`) |
| Barman Cloud plugin | v0.15.0 (chart 0.8.0) |
| External Secrets Operator | 2.10.0 |
| cert-manager / trust-manager | 1.21.1 / 0.24.0 |
| StorageClass | openebs-hostpath (OpenEBS 4.6.0), `WaitForFirstConsumer`, default |
| Velero | 1.18.1 (chart 12.1.0) |
| Object storage | MinIO |
| schmetterpause | v0.6.0 |

## Preflight

Before the first deployment:

```sh
# Kubernetes version -- and does the CloudNativePG release support it?
kubectl version

# Gateway API and an implementation
kubectl get gatewayclass
kubectl get gateway -A \
  -o jsonpath='{range .items[*]}{.metadata.namespace}/{.metadata.name}: {range .spec.listeners[*]}{.name}={.protocol},from={.allowedRoutes.namespaces.from} {end}{"\n"}{end}'

# A default StorageClass, or a dbStorageClass to name
kubectl get storageclass

# CloudNativePG, and the Barman Cloud plugin if backups are wanted
kubectl get crd clusters.postgresql.cnpg.io objectstores.barmancloud.cnpg.io
kubectl get deploy -A -l app.kubernetes.io/name=cloudnative-pg

# External Secrets, if that variant is used
kubectl get clustersecretstore

# Velero, if used
kubectl -n velero get backupstoragelocations.velero.io
```

After it, for backups: `ContinuousArchiving=True` on the Cluster and one backup
in phase `completed`. A Ready Kustomization or a synced Application says neither.
