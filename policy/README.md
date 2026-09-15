# Cluster policies

Kyverno policies this project ships for the cluster it runs on. They are not
part of the kustomize base and are not rendered by KCL.

| File | What it asserts |
| --- | --- |
| `verify-image-signature.yaml` | the application image carries a keyless cosign signature made by this repository's CI workflow |

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

## Applying it

```sh
task policy:apply            # kubectl apply -f policy/
task policy:status           # is it ready, and what has it reported?
```

Applying it needs cluster-admin, because the policy is cluster-scoped, and a
Kyverno that serves `policies.kyverno.io/v1` — 1.19.1 on `homerun2-test1` does.
On that cluster the policies the platform ships live in the argocd catalog
(`infra/kyverno/install`); this one is applied by hand until somebody decides
it should be reconciled, which is an open point in ADR-0020.

The drill that proves the policy can refuse something, and the condition for
moving it from `Audit` to `Deny`, are in
[`docs/supply-chain.md`](../docs/supply-chain.md).
