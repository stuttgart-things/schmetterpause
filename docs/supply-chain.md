# Signatures, SBOM and what checks them

What runs should verifiably be what CI built, and something other than a human
should be the one checking. This page is the operator's side of that: what is
signed, how to check it by hand, what refuses an artefact that is not ours, and
what is deliberately not covered.

The decision behind it is [ADR-0020](adr/0020-signieren-und-pruefen.md); the
issue is [#230](https://github.com/stuttgart-things/schmetterpause/issues/230).

## At a glance

| | |
| --- | --- |
| Signature | keyless cosign — GitHub OIDC → Fulcio, recorded in Rekor |
| Signed by | `https://github.com/stuttgart-things/schmetterpause/.github/workflows/ci.yml@refs/*` |
| Issuer | `https://token.actions.githubusercontent.com` |
| Image | `ghcr.io/stuttgart-things/schmetterpause` — signature and CycloneDX SBOM attestation, both on the digest |
| Manifest artefact | `ghcr.io/stuttgart-things/schmetterpause-kustomize` — signature only |
| Checked by | a Kyverno `ClusterPolicy` at admission, and the `verify-artefacts` job in CI |
| Not checked | Azure Container Apps, and every image that is not ours |

There is no key. Nothing to rotate, nothing in Vault, nothing to leak — and a
public record in the transparency log of every signature ever made, which for a
public package is the trade the project wanted. ADR-0020 has the reasoning.

## Checking an artefact by hand

```sh
task verify:signature                    # the current VERSION, both artefacts
task verify:signature TAG=v0.8.0         # a particular release
```

That wraps cosign. The long form, so that the two strings above are readable
rather than buried in a Taskfile:

```sh
cosign verify \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github\.com/stuttgart-things/schmetterpause/\.github/workflows/ci\.yml@refs/' \
  ghcr.io/stuttgart-things/schmetterpause:v0.8.0
```

Both flags are needed and neither is enough on its own. Anybody can get a
certificate from that issuer; the identity says which workflow file, in which
repository, asked for it. A `cosign verify` without `--certificate-identity*`
refuses to run for exactly this reason.

### The inventory

```sh
task verify:sbom                         # build/sbom.cdx.json
task verify:sbom TAG=v0.8.0 DEST=/tmp/sbom.json
```

Which is this, and it goes through `verify-attestation` rather than
`cosign download sbom` on purpose: an inventory that cannot be traced back to
the pipeline is a list, not evidence.

```sh
cosign verify-attestation \
  --type cyclonedx \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github\.com/stuttgart-things/schmetterpause/\.github/workflows/ci\.yml@refs/' \
  ghcr.io/stuttgart-things/schmetterpause:v0.8.0 \
  | jq -r '.payload | @base64d | fromjson | .predicate' > sbom.cdx.json
```

It is a CycloneDX document written by Trivy — the same scanner the pipeline
already runs against the same digest, which is why there is no second tool
here pulling the image again.

**It describes `linux/amd64`.** The release is one manifest covering two
architectures, and an SBOM is per image, not per index. The arm64 image is
built from the same source on the same base, so the inventories should differ
only in the architecture of their packages — but that is a claim nobody has
checked, and a second attestation for arm64 is an open point in ADR-0020.

## What checks it without being asked

### On the cluster: Kyverno

`policy/verify-image-signature.yaml` is a `ClusterPolicy` with a `verifyImages`
rule, scoped to `schmetterpause` and `schmetterpause-pr-*`. It refuses — or, for
now, reports — a pod whose application image carries no signature from the
identity above, and rewrites the tag to the digest it verified so the kubelet
cannot pull something else afterwards.

```sh
task policy:apply
task policy:status
```

**It is in `Audit` today.** It moves to `Enforce` once it has seen three clean
releases, the posture Trivy took in this repository for the same reason: a rule
that has never run over a real release is a measurement, not a verdict.

| Release | Policy report | Date |
| --- | --- | --- |
| _(none yet — the policy has not been applied)_ | | |

Fill that in as the releases come, and when the third line is clean change
`validationFailureAction` in the policy from `Audit` to `Enforce` and say so
here. A gate that cannot fail is a measurement, and the day it stops being one
is worth a line.

Two things to know before flipping it:

- `failurePolicy: Ignore`. If Kyverno cannot reach the registry or Rekor, the
  pod is admitted rather than refused. That is the right default for a cluster
  the office plays on and the wrong one for a claim of coverage, so it is
  named here rather than left to be discovered. Changing it to `Fail` makes
  every pod in those namespaces depend on Rekor being up.
- The rule covers the application image only. Postgres, and anything else in
  the same namespace, is not checked by it and is not claimed to be.

### In the delivery path: the `verify-artefacts` job

The kustomize artefact is what Argo consumes, and admission never sees it — the
kubelet does not pull it. So for that half, the check is a CI job:
`verify-artefacts` in `.github/workflows/ci.yml` verifies both artefacts after
they are signed, and fails the build if either does not verify.

It is weaker than admission by construction: it checks the thing we published,
not the thing the cluster pulled. Closing that would mean a policy engine in
front of Argo's OCI pull, which nothing in this org runs today — an open point
in ADR-0020.

## Proving it can refuse

> A verification that has never refused anything is indistinguishable from one
> that cannot.

Two halves, because one of them needs a cluster and one does not.

**Every build.** The last step of `verify-artefacts` runs the same `cosign
verify` against an identity that must not match, and fails the build if it
passes. It catches the failure mode the drill below is for: a misspelled flag,
or an identity pattern that quietly matches everything, would sail through the
step above it and keep doing so forever.

And the four places that name the signer — this page, the pipeline, the
Taskfile and the policy — are compared against each other by
`scripts/signing_test.sh`, which runs in the pipeline's lint stage
(`task signing:check` locally). Widening one of them is the quiet way this
stops being a gate: the Kyverno subject is a glob and the cosign identity is an
anchored regexp, so "the same string" is not something a reader checks by
looking.

**By hand, against the cluster.** This is the drill DoD point 5 asks for, and
it has not been run yet — it needs the policy applied.

1. Build and push an unsigned image under a scratch tag:
   ```sh
   docker pull alpine:3 && \
   docker tag alpine:3 ghcr.io/stuttgart-things/schmetterpause:unsigned-drill && \
   docker push ghcr.io/stuttgart-things/schmetterpause:unsigned-drill
   ```
2. Try to run it in the namespace the policy covers:
   ```sh
   kubectl -n schmetterpause run signature-drill \
     --image ghcr.io/stuttgart-things/schmetterpause:unsigned-drill \
     --restart=Never --command -- sleep 30
   ```
3. Read what happened. Under `Audit` the pod starts and the refusal is a
   report:
   ```sh
   kubectl -n schmetterpause get policyreport -o yaml | grep -A5 signature
   ```
   Under `Enforce` the `kubectl run` itself fails, and the message names the
   policy and the rule.
4. Clean up, and delete the scratch tag from the package. A tag on the release
   repository that names something that is not a release is exactly the kind of
   thing #177 and #179 were about.
5. **Write down how long it took and what the refusal looked like** — in this
   section, replacing this list. A drill nobody recorded is a drill nobody can
   tell was run.

## Where this does not reach

- **Azure Container Apps.** No admission control exists there at all (#206),
  so nothing on that side checks a signature before starting a container. The
  image is verified in CI and nowhere else. The environment is temporary
  (ADR-0016) and this is the reason it is named rather than quietly left out:
  covering one of two environments while sounding like coverage is the failure
  this whole piece of work exists to avoid.
- **Argo's pull of the kustomize artefact.** See above.
- **Everything below the signature.** A signature says the artefact came from
  this workflow. It says nothing about whether what the workflow built was
  right, which is what the tests, the scans and the reviews are for. It also
  says nothing about the base image, which is somebody else's signature to
  check and is not checked here.
- **The signatures of pull-request builds are not swept.**
  `cleanup-pr-artifacts.yml` deletes versions tagged `pr-<n>-*`; cosign stores
  a signature under `sha256-<digest>.sig`, which does not match that pattern.
  So a closed pull request leaves two small objects behind per build. Noted
  rather than fixed — it needs a change in a shared workflow this repository
  does not own, and it is an open point in ADR-0020.
