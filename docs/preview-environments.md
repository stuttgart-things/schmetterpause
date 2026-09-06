# Preview environments

Every pull request can have its own running Schmetterpause: its own hostname,
its own database, filled with a demo field. It exists because this is a browser
application whose premise is a colleague opening it on their phone during a
break — *"does this look right on your phone?"* is a question a link answers
and a diff does not.

## Getting one

Put the label **`preview`** on the pull request. That is the whole procedure.

Within a few minutes a comment appears with the address, and roughly ten
minutes after labelling the environment answers:

```
https://schmetterpause-pr-<number>.homerun2-test1.sthings-vsphere.labul.sva.de
```

What takes the time, in order: CI publishes the image and the manifest artefact
for the head commit (~4 minutes), then the ApplicationSet notices the label on
its next poll (**up to 10 minutes** — `requeueAfterSeconds: 600`), then ArgoCD
syncs and the seed job runs (seconds).

The label decides whether an *environment* is created. It does not gate the
*build*: CI publishes a `pr-<n>-<sha>` image and artefact for every pull request
from this repository, which is also what lets the Trivy scan see an image
before a change reaches `main`. Labelling later therefore costs nothing — the
artefact is already there.

## What you get

| | |
|---|---|
| Namespace | `schmetterpause-pr-<n>` on `homerun2-test1` |
| Database | its own CloudNativePG instance, 1Gi |
| Content | six players, twelve results — ten confirmed, one waiting, one contested |
| Certificate | the gateway's `*.homerun2-test1…` wildcard; no per-preview certificate or DNS record |
| Version shown on `/info` | `pr-<n>-<head sha>`, so you can check you are looking at what you think you are |

The fixture matters more than it sounds. An empty Schmetterpause is a join form
and an empty ranking, so a change to the standings, the match list or a profile
page would show nothing at all. It is written through the application's own
rules — matches are created pending and settled by `scoring.Confirm` — so the
ratings, the history and the standings agree with the match list because they
were computed from it. Two results are left unfinished on purpose: *Zu
bestätigen* and *Wartet auf den Gegner* are invisible in an all-confirmed
fixture, and those are exactly the screens somebody reviewing a change to them
needs to see.

It is deterministic: two previews of one commit are the same preview, which is
what makes a screenshot comparable.

## Tearing one down

Closing or merging the pull request removes the environment and deletes both
GHCR packages for it. Removing the `preview` label does the same at the next
poll, which is the quicker way to free the database if a pull request is going
to stay open for a while.

A preview costs one application pod, one Postgres instance and one 1Gi volume
for as long as the label is on. That is the reason the label exists rather than
previews being automatic — a Renovate bump does not need a database.

## The setup, in three repositories

Needed only if you are reproducing this, or fixing it when it breaks. Nothing
below has to be touched to get a preview.

**This repository** — everything that produces the artefacts:

| File | What it does |
|---|---|
| `.github/workflows/ci.yml`, job `release` | publishes the image as `pr-<n>-<head sha>` |
| the same file, job `kustomize` | publishes the manifest artefact under the same tag, and **only for `pr-*` renders the seed Job into it** |
| `.github/workflows/cleanup-pr-artifacts.yml` | deletes both packages when the pull request closes |
| `.github/workflows/comment-preview-url.yml` | posts the address, on `labeled` |
| `kcl/seed.k` | the Job, off unless `config.seedEnabled` |
| `internal/seed/` | the fixture, behind `schmetterpause seed` |

**`stuttgart-things/argocd`** — `platforms/schmetterpause-pr-preview/`: an
`ApplicationSet` whose matrix is (clusters labelled `schmetterpause-pr-preview`)
× (open pull requests labelled `preview`), rendering
`apps/schmetterpause/install` per cell.

**`stuttgart-things/stuttgart-things`** — two files under
`clusters/labul/vsphere/platform-sthings/argocd/`:
`schmetterpause-pr-preview-platform.yaml` (the bootstrap Application) and
`homerun2-test1/cluster.yaml`, whose `spec.labels` carries
`schmetterpause-pr-preview: "true"`. Clusterbook propagates that label to the
Argo cluster Secret, which is what the ApplicationSet actually reads.

Four things the platform already provides, and none of them needs work per
preview: the shared PR-reader token every preview ApplicationSet in the catalog
uses, the cluster's AppProject (it permits `namespace: '*'` on its own cluster),
the `vault-schmetterpause` `ClusterSecretStore` (it authenticates as the
cluster's ESO service account rather than per namespace), and wildcard DNS plus
the wildcard certificate.

## Traps that are already paid for

Written down because each of them cost something, and because the next
application to get previews will meet them again.

**The tag must name the commit it says it does.** GitHub builds the merge
commit by default, but the ApplicationSet composes `pr-<n>-<head_sha>` from
what the GitHub API gives it. So CI checks out the head sha and asserts that
`git rev-parse HEAD` matches the sha in the tag. This repository has twice
shipped a tag that identified a different build than its name claimed
(issues #177, #179); both times it was invisible until digests were compared by
hand.

**The seed Job cannot be patched in.** The consuming chart patches what the
artefact contains, so a resource that was never rendered cannot be added by a
consumer. Whether a preview seeds itself is therefore decided where the
artefact is built — `pr-*` carries the Job, releases do not. The second guard is
in the binary: `seed` refuses a database that already has players, so there is
no flag anybody has to pass correctly.

**A Job's pod template is immutable.** A plain Job stops syncing the moment the
image tag changes, which in a preview is every push. It is an ArgoCD `PostSync`
hook with `hook-delete-policy: BeforeHookCreation`, so a new tag becomes a new
Job rather than a rejected patch — and `PostSync` also means it runs after the
migration initContainer has been through.

**Deleting a GHCR package version needs more than `packages: write`.** The
repository must have the **Admin** role on the package under *Manage Actions
access*. A package whose first version was pushed from a laptop does not grant
it, and the failure is a `404 Package not found` on the delete while the listing
that found the version worked. That is what the cleanup workflow hit on its
first run.

**"0 applications generated" has two causes that look identical.** No cluster
carries the label, or no pull request does. The ApplicationSet's
`ParametersGenerated` condition is what tells them apart — it is `True` in the
second case and the message names the failure in the first.

## Previews for another application

The shape transfers: publish `pr-<n>-<head_sha>` artefacts with a cleanup on
close, add a `platforms/<app>-pr-preview` ApplicationSet to the catalog, add
the bootstrap Application and one cluster label. What does not transfer is the
part that made this worth doing — an empty preview is a useless preview, so
budget for a fixture, and for whatever the application needs per environment
that is not a container. Here that was a database per pull request, and it is
the single largest line in the cost.
