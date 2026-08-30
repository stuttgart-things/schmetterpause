# Kubernetes manifests

KCL module for running Schmetterpause on Kubernetes. Renders a ServiceAccount,
a ConfigMap, two ExternalSecrets, a Deployment, a Service and two HTTPRoutes —
depending on what the profile switches on.

Everything is a variable. No hostname, no namespace, no gateway and no image
reference is fixed in the module, so the same manifests fit a laptop cluster,
the office cluster and a preview namespace.

This document describes the module. How to bring up an environment from nothing
— checking prerequisites, secrets, Postgres, ordering — is in
[`docs/deployment.md`](../docs/deployment.md), including for clusters without
the External Secrets Operator.

## Rendering

```sh
task kcl:render                      # profile base
task kcl:render PROFILE=existing-secrets
task kcl:check                       # do all profiles still render?
task kcl:kustomize                   # kustomize base into build/kustomize
task kcl:publish TAG=v1.2.3          # base to GHCR as an OCI artefact

task kcl:apply                       # render and apply to the current context
task kcl:secrets NAMESPACE=…         # the two Secrets by hand, without ESO
```

Overriding single values — everything after `--` goes to `kcl`, behind the
profile's own flags, and a later `-D` wins:

```sh
task kcl:render -- \
  -D config.image=ghcr.io/stuttgart-things/schmetterpause:v1.2.3 \
  -D config.kioskEnabled=true
task kcl:apply PROFILE=existing-secrets -- -D config.gatewayName=cilium-gateway
```

Calling `kcl run` directly is the way this goes wrong: `-D` loads no profile,
and neither does `-Y kcl/profiles/base.yaml` — the profiles are flat
`key: value` files, not KCL settings files. Either way every field not passed
falls back to its default in `schema.k`, and those are deliberately reticent:
`httpRouteEnabled` is `false`, `secretsMode` is `external`.

### Why the output looks the way it does

`kcl run` emits a list under the key `manifests:`. That is not an aesthetic
slip but a **contract**: `stuttgart-things/dagger/kcl` — the module that turns
this into a kustomize base and pushes it as an OCI artefact — normalises the
output with a fixed `sed` chain that assumes exactly this shape: drop the first
line, de-indent by two, turn `- ` at column zero into a document separator.

A YAML stream (`manifests.yaml_stream`) looks nicer and is directly usable with
`kubectl apply -f` — but put through the same chain, the first resource loses
its `apiVersion` and every `metadata` field is lifted to the top level. The
result still parses as YAML; it just deploys something else. Measured, not
assumed.

**Hence `task kcl:render` rather than `kcl run`.** The task applies the same
normalisation, so what appears locally is what ends up in the OCI artefact. Its
output is `---`-separated and usable with `kubectl apply -f`.

**A render with no profile fails**, and deliberately so: `secretsMode` is
`external`, and that needs a secret store and a path. The error names both. A
module that renders something when told nothing renders something nobody wants
to deploy.

## What is not rendered

**No Namespace.** Several Applications share a workload namespace. When more
than one of them ships a Namespace resource, ArgoCD marks it as a
SharedResource — and one Application's prune cycle deletes the namespace out
from under the others. The consuming Application sets `CreateNamespace=true`
instead.

**No CloudNativePG Cluster.** For a related reason with worse consequences: the
database outlives every revision of the application. A base that carries the
`Cluster` is a base whose removal can take the data with it. The `Cluster` is
its own Argo Application from `infra/cloudnative-pg/cluster` in the catalogue,
into the same namespace, at a lower sync-wave — so it exists before the
migration initContainer runs.

## Two things that are not what they look like

### The second HTTPRoute is not decoration

The shared gateway serves the same wildcard on two listeners: HTTPS on 443 and
plain HTTP on 80. A route with no `sectionName` attaches to **both**. Over
plain HTTP a browser never returns the `Secure` session cookie — a player
joins, is shown their recovery code once, and arrives at the next request as a
stranger. That is issue #70 rebuilt by a listener, and it would look like an
application bug.

So the application binds to the TLS listener by name, and the plain listener
gets a route of its own that only redirects.

### No field can hold a secret

There is no place in the schema for `SP_SESSION_KEY`, and no default that could
invent one. What the schema accepts is a **path** into the secret store.

The reason is not tidiness: a changed session key does not fail, it silently
forgets every player — `TestARestartWithADifferentKeyForgetsEverybody` exists
to prove it. A value in a profile file is a value in git, and a rendered Secret
with a generated default would log the whole office out on every render. A
wrong Vault path, by contrast, fails loudly at sync time. Loud beats silent
here.

Two Secrets rather than one, and the split is least privilege: the migration
initContainer gets only the database Secret and never sees the cookie secret.
`config.Load` and `ValidateForServe` being separate makes that free.

| Secret | Keys | Read by |
|---|---|---|
| `<name>-db` | `username`, `password`, `SP_DATABASE_URL` | initContainer **and** app |
| `<name>-app` | `SP_SESSION_KEY`, optionally `SP_KIOSK_TOKEN` | the app only |

`<name>-db` is of type `kubernetes.io/basic-auth` so CloudNativePG can take it
through `appSecretName` as the owner's credentials at bootstrap. The `username`
in the Vault entry has to match the `owner` of the CNPG Application — two
repositories, one value.

The Vault entry has three properties, and a fourth only with the kiosk:

| Property | Becomes | Requirement |
|---|---|---|
| `session-key` | `SP_SESSION_KEY` | at least 32 characters, `openssl rand -base64 32` |
| `username` | DB owner and DSN | equal to the CNPG Application's `owner` |
| `password` | DB password and DSN | alphanumeric, `openssl rand -hex 24` |
| `kiosk-token` | `SP_KIOSK_TOKEN` | only with `kioskEnabled: true` |

The ESO template assembles `SP_DATABASE_URL` from these: the password comes
from Vault, while host, port, database and `sslmode` come from this module.

> **Keep the Vault password alphanumeric.** It is interpolated into the URL
> without escaping. A password containing `@`, `/`, `:` or `?` produces a DSN
> that parses as something else.

`config.vaultPath` is the entry name **under** the store's mount, and nothing
else. The store already carries the mount and `version: v2`, so ESO composes
`<mount>/data/<path>` itself. Both ways of getting this wrong resolve to
something that exists nowhere — and report no error at apply time:

```
schmetterpause                   correct
schmetterpause/data/cicd-test2   -> cicd-test2/data/schmetterpause/data/cicd-test2
schmetterpause-cicd-test2        -> the cluster twice; the mount IS the cluster
```

The store is named per cluster as `vault-<cluster>`, assigned by the Backstage
template — never plain `vault`. The reliable source:

```sh
kubectl get clustersecretstore
```

### When a secret does not arrive

Three things worth knowing once:

**A missing property is not reported by the store.** The ExternalSecret goes to
`SecretSyncedError` and everything around it stays green. `kubectl get
externalsecret -A` is the only place it shows.

**A store reporting `Valid`/`Ready=True` may still read nothing.** Green means
the login works — not that the policy opens the path. The proof is a synced
Secret and nothing else.

For this application that is milder than it sounds: both Secrets are mounted
through `envFrom`/`secretKeyRef` **without** `optional`. If one is missing the
pod does not start — `CreateContainerConfigError` rather than an application
running on half a configuration. The silent failure in the ExternalSecret
becomes loud one line further on.

One exception to remember: `kioskEnabled: true` adds `kiosk-token` to the same
ExternalSecret as the session key. If the property is missing from the Vault
entry, **the whole** Secret stops syncing — and then the application is down,
not just the kiosk.

## Values that matter

The full list, with reasoning, is in `schema.k`. What one actually sets:

| Value | Default | Meaning |
|---|---|---|
| `config.image` | `…:latest` | Pin a digest for a real deploy |
| `config.namespace` | `schmetterpause` | |
| `config.clusterDomain` | *(empty)* | Combines with `name` into the hostname |
| `config.host` | *(empty)* | Overrides that composition |
| `config.httpRouteEnabled` | `false` | |
| `config.gatewayName` / `…Namespace` | *(empty)* | Both required once routes are on |
| `config.httpRedirectEnabled` | `true` | Route on port 80 redirecting to https |
| `config.kioskEnabled` | `false` | Without `SP_KIOSK_TOKEN` there is no kiosk |
| `config.secretsMode` | `external` | `existing` = render no ExternalSecrets; something else creates the Secrets |
| `config.secretStoreName` | *(empty)* | e.g. `vault-cicd-test2` |
| `config.vaultPath` | *(empty)* | A path, never a value |
| `config.replicas` | `1` | A `check:` holds it there, see below |
| `config.bootstrapAdmin` | *(empty)* | Display name, takes effect at startup |

### Why `replicas` is pinned at 1

`internal/repository/postgres/migrate.go` calls `goose.UpContext` through the
package API, and that takes **no** session lock — only goose's provider API
does. Two pods migrating at once is unsafe in practice, not in theory. A
`check:` in the schema rejects anything else. Going higher means building an
advisory lock or a migration Job first.

For the same reason the strategy is `Recreate` and not `RollingUpdate`: a
rolling update would run `migrate up` in the new pod while the old one still
serves the old schema.

## The profile format

Flat, `key: value` — **not** KCL's own `kcl_options` format:

```yaml
config.namespace: schmetterpause
config.httpRouteEnabled: true
```

This is the shape `stuttgart-things/dagger/kcl` reads. It converts the file
inside its container with

```sh
yq eval -o=json params.yaml | jq 'to_entries | map(.key + "=" + (.value|tostring))'
```

and turns that into one `-D` per entry. A file in `kcl_options` format survives
that conversion as **a single** `-D kcl_options=[…]` which KCL does not know —
and every value silently falls back to its default. Here the `check:` on
`secretStoreName` caught it; without it we would have published an artefact of
pure defaults that looks like a deploy.

`task kcl:render` performs the same conversion locally. Hence `task kcl:render`
rather than `kcl run -Y` — the latter wants the `kcl_options` format and would
accept a file that does not work in the publish path.

## Generic in the repo, adapted per environment

There is **no cluster name** in this repository. The module is generic and so
is the published artefact: `kcl:publish` renders `base.yaml`, in which every
value naming a place is a placeholder.

Adaptation happens in each environment's Argo Application, the way #89 draws
it:

```
kcl/  ──▶  neutral kustomize base  ──▶  OCI  ──▶  Argo patches per environment
```

The placeholders **switch no resource off**. All eight are rendered, the
HTTPRoutes included even with no real gateway — kustomize can patch a field but
not a resource that was never there.

### What an environment has to patch

| Field | Where | Example |
|---|---|---|
| Hostname | both `HTTPRoute`, `spec.hostnames[0]` | `schmetterpause.my-cluster.example.com` |
| `SP_PUBLIC_BASE_URL` | `ConfigMap`, `data` | `https://schmetterpause.my-cluster.example.com` |
| Gateway | both `HTTPRoute`, `spec.parentRefs[0].name` / `.namespace` | `cilium-gateway` / `default` |
| Secret store | both `ExternalSecret`, `spec.secretStoreRef.name` | `vault-backend` |
| Vault entry | both `ExternalSecret`, `spec.data[*].remoteRef.key` | `schmetterpause` |
| Namespace | kustomize `namespace:` | `schmetterpause` |

**Hostname and `SP_PUBLIC_BASE_URL` are one decision in two places.** Patch
only the route and you get a reachable application whose printed QR code points
at `cluster.example.com`. That is the seam of this design; it is the price of
keeping the base neutral.

```yaml
kustomize:
  namespace: schmetterpause
  patches:
    - target: { kind: HTTPRoute }
      patch: |-
        - op: replace
          path: /spec/hostnames/0
          value: schmetterpause.my-cluster.example.com
        - op: replace
          path: /spec/parentRefs/0/name
          value: cilium-gateway
        - op: replace
          path: /spec/parentRefs/0/namespace
          value: default
    - target: { kind: ConfigMap, name: schmetterpause-config }
      patch: |-
        - op: replace
          path: /data/SP_PUBLIC_BASE_URL
          value: https://schmetterpause.my-cluster.example.com
    - target: { kind: ExternalSecret }
      patch: |-
        - op: replace
          path: /spec/secretStoreRef/name
          value: vault-backend
```

How the Argo Application consumes the OCI artefact belongs to #81 — the above
is the part that comes from here.

### Rendering locally for a real cluster

Without putting a profile of real values into the repo:

```sh
task kcl:render -- \
  -D config.clusterDomain=my-cluster.example.com \
  -D config.gatewayName=cilium-gateway \
  -D config.secretStoreName=vault-backend \
  -D config.vaultPath=schmetterpause
```

## The chain behind it

`kcl:kustomize` and `kcl:publish` call a shared Dagger module rather than
building something of our own — the path #80 describes:

```
kcl/  ──render-kustomize-base──▶  kustomize base
      ──push-kustomize-base────▶  ghcr.io/…/schmetterpause-kustomize:<tag>
                                          │
                                    Argo CD points at it (#81)
```

The tag is **the same as the image's**, deliberately: a pair of artefacts that
can drift apart is a deploy nobody reconstructs afterwards.

The module is pinned at `@v0.82.0`. Without the pin, `task kcl:publish` could
render something different tomorrow than it does today, and that is the one
property a deploy artefact must not have.

## When Dagger will not start

```
failed to select internal socket: failed to get SSH auth socket fingerprints:
failed to list SSH agent identities: agent: client error: EOF
```

This is not a problem with the manifests — it happens while loading the module.
Dagger asks the SSH agent for identities, and in a VS Code remote session
`SSH_AUTH_SOCK` often points at a forwarded socket whose target no longer
exists. Check with `ssh-add -l`; work around it by emptying the variable for
the call:

```sh
SSH_AUTH_SOCK= task kcl:kustomize
```

Not hard-wired into the Taskfile: anyone who needs SSH for private Go
dependencies would lose it.

## Related

- `docs/deployment.md` — bringing up an environment, with and without ESO
- `docs/adr/` — decisions on the data model, auth and deployment
- Issue #78 — what was decided here and why
- Issue #89 — phase 3 as a whole
- `infra/cloudnative-pg` in `stuttgart-things/argocd` — operator and cluster
