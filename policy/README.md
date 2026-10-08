# Cluster policies

Kyverno policies this project ships for the cluster it runs on. They are not
part of the kustomize base and are not rendered by KCL.

| File | What it asserts |
| --- | --- |
| `verify-image-signature.yaml` | the application image carries a keyless cosign signature made by this repository's CI workflow |
| `tests/` | what that policy must refuse and what it must admit, checked with `kyverno test` |

## Why not in `kcl/`

`kcl/` renders the kustomize base, and the base is namespaced: one Argo
Application installs it into `schmetterpause`, and the preview ApplicationSet
installs a copy of it into every `schmetterpause-pr-<n>`. An
`ImageValidatingPolicy` is cluster-scoped, so shipping it there would mean every
preview namespace installing and pruning the same cluster-wide object — the
`SharedResource` problem `kcl/main.k` already describes for the Namespace, with
a policy engine on the other end of it.

Same reasoning as ADR-0019 gives for the backup configuration: what belongs to
the cluster rather than to a revision of the application does not travel in the
deploy artefact.

## Testing it

```sh
task policy:test             # does it still refuse and admit what tests/ says?
```

The same call the `policy-test` job makes in CI: the kyverno Dagger module,
with the Kyverno release pinned as `KYVERNO_VERSION` in the Taskfile and with
warnings as errors. It needs the network, because the policy verifies
signatures while it is tested. `tests/kyverno-test.yaml` says what each fixture
is for and what cannot be tested this way.

## Applying it

**On `homerun2-dev2`, the office's cluster, nobody applies it by hand.** The
argocd catalog entry `apps/schmetterpause/install` has `policy.enabled`, and its
child Application reads `verify-image-signature.yaml` from this repository at
the same tag as the deployed release. `directory.include` names that one file,
so `tests/` never reaches the cluster. On homerun2-dev2 the switch is the
annotation `tabletennis-platform.stuttgart-things.com/policy-enabled: 'true'`
in the cluster's ClusterStack order
(`clusters/labda/vsphere/machinery-xrs/homerun2-dev2.yaml` in
`stuttgart-things`), which the `tabletennis` ApplicationSet maps onto
`policy.enabled`. It was first switched on on homerun2-test1 on 2026-09-15
(stuttgart-things/argocd#446, stuttgart-things/stuttgart-things#2982). A change to
the policy therefore reaches the cluster with the next release that carries it,
not with a merge to `main`.

With `monitoring-enabled` on the same order as well, a refusal raises
`SchmetterpauseUnsignedImageRefused` in the Teams alert channel
(stuttgart-things/argocd#448, #449, renamed from
`SchmetterpauseUnsignedImageAdmitted` with the move to `Deny`).

For a cluster outside the catalog path:

```sh
task policy:apply            # kubectl apply -f policy/ (not recursive, so tests/ stays out)
task policy:status           # is it ready, and what has it reported?
```

Applying it by hand needs cluster-admin, because the policy is cluster-scoped,
and a Kyverno that serves `policies.kyverno.io/v1`. 1.19.1, on `homerun2-dev2`
(as on `homerun2-test1` before it), does.

The policy is in `Deny` from v0.16.0 (#262). The ledger of clean releases that
move rests on, and the drill that proves the policy refuses something on the
cluster, are in [`docs/supply-chain.md`](../docs/supply-chain.md).
