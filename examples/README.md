# Examples

Three ways to adapt this application to an environment. They differ in *when*
they act, and that decides which one is right.

| | Acts | Needs kcl | Deploys the published artefact |
|---|---|---|---|
| `-D` overrides | before the artefact | yes | no |
| a profile file | before the artefact | yes | no |
| kustomize patches | after the artefact | no | yes |

## Before the artefact: overrides and profiles

Everything after `--` goes to `kcl`, behind the profile's own values, and a
later `-D` wins:

```sh
task kcl:apply -- -D config.gatewayName=cilium-gateway
```

For more than a handful of values, put them in a file. `PROFILE` takes either a
name from `kcl/profiles` or a path to a file anywhere:

```sh
task kcl:apply PROFILE=~/environments/cicd-test2.yaml
```

The path form is the one an environment uses. A profile that names a cluster
has no business in this repository — keeping it outside is what lets the
published base stay neutral. The format is flat `key: value`, the same one
`kcl/profiles/base.yaml` uses.

Both render locally and apply the result. That is right for a test cluster and
for fault-finding, and wrong as the answer to "what is actually running there":
what gets applied never existed as an artefact.

## After the artefact: kustomize patches

The base is rendered once, pushed once, and every cluster deploys that exact
artefact plus a visible set of patches.

- [`argocd-application.yaml`](argocd-application.yaml) — an ArgoCD Application
  with the OCI artefact as its source. Four values marked `CHANGE ME`.
- [`kustomize/`](kustomize/) — the same patches without Argo, for seeing what
  the patched artefact becomes before a controller applies it.

```sh
cd examples/kustomize
mkdir -p base && (cd base && oras pull ghcr.io/stuttgart-things/schmetterpause-kustomize:3382ad1)
kubectl kustomize .
```

### What has to be patched

| Field | Where |
|---|---|
| Hostname | both `HTTPRoute`, `spec.hostnames[0]` |
| `SP_PUBLIC_BASE_URL` | `ConfigMap`, `data` |
| Gateway | both `HTTPRoute`, `spec.parentRefs[0].name` and `.namespace` |
| Secret store | both `ExternalSecret`, `spec.secretStoreRef.name` |
| Namespace | kustomize `namespace:` |

Two of these are worth stating plainly, because both fail quietly:

**Hostname and `SP_PUBLIC_BASE_URL` are one decision in two places.** Patch only
the route and the application is reachable while its printed QR code points at
`cluster.example.com`.

**`parentRefs[0].namespace` is not optional.** Left out, the parentRef resolves
against the route's own namespace, and a route pointing at a Gateway that is
not there gets no status at all — not `Accepted=False`, nothing — while the
host answers 404 from the gateway.

There is no image patch. The kustomize base and the container image are
published under the same tag on purpose, and `kcl:publish` pins the image to
the tag the artefact itself carries. Setting `targetRevision` moves both.

## Related

- [`../docs/deployment.md`](../docs/deployment.md) — bringing up an environment
  from nothing, with and without External Secrets
- [`../kcl/README.md`](../kcl/README.md) — the module itself
