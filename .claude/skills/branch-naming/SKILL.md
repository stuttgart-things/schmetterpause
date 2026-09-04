---
name: branch-naming
description: Name a git branch so it reads well and stays out of release-please's way. Use when starting work on a change, creating a branch, or when a branch name needs correcting before a pull request is opened.
---

Branch names carry the conventional-commit type of the change they hold, so the name says what is in the branch rather than who made it.

## The shape

```
<type>/<short-kebab-case-description>
```

`feat/ttr-package`, `fix/readyz-timeout`, `ci/publish-to-ghcr`, `docs/adr-slot-model`.

Not `wip`, not `patrick`, not `claude/whatever` — those say who or when, which is what `git log` is for.

## Types

The same set the commit messages use, so a branch and the commits on it agree:

| Type | For |
|---|---|
| `feat` | a new capability |
| `fix` | a bug fix |
| `docs` | documentation only |
| `refactor` | neither fixes a bug nor adds a capability |
| `perf` | a change that improves performance |
| `test` | adding or correcting tests |
| `build` | build system or dependencies |
| `ci` | CI configuration and pipeline |
| `chore` | anything else that touches neither src nor test |

Pick the type of the change that defines the branch. A branch that adds a feature and drags along a lint fix is still `feat/`.

## Names that are reserved

Almost none, and that is a deliberate consequence of the tooling. **Releases are cut by release-please** (issue #39), which keeps its own branch:

```
release-please--branches--main
```

It creates and force-pushes that branch itself. Do not work on it, and do not create it by hand — everything else is yours.

`main` is the release branch: what lands there is what a release is derived from.

This section used to list `beta`, `alpha`, `next`, `1.x`, `1.2.x` and a maintenance glob, because semantic-release treats those as prerelease and maintenance channels and would start publishing from a working branch that happened to be called `beta`. **That was decided against** — see #39 for why the release-PR model was preferred — so those names are no longer dangerous here. They are still poor branch names, but for the ordinary reason: they do not say what is in the branch.

The `<type>/<description>` shape collides with nothing under either tool, which is the point of insisting on it.

## What actually decides the version

**Commit messages, not branch names.** release-please reads the commits that reach `main` and derives the bump:

| Commit | Release |
|---|---|
| `feat: ...` | minor |
| `fix: ...` | patch |
| `perf: ...` | patch |
| `feat!: ...` or a `BREAKING CHANGE:` footer | major |
| `docs:`, `refactor:`, `test:`, `ci:`, `chore:`, `build:` | none |

The branch name is for humans. It is worth getting right so the pull request list reads like a changelog, but it never moves a version number on its own.

What release-please then does with that is open a **release pull request** which accumulates the changes and maintains `CHANGELOG.md`. Merging it is what tags. So the bump is derived automatically and taken deliberately — nothing releases because a commit landed.

## The merge strategy changes which messages count

This matters more than it looks:

- **Merge commit** — every commit on the branch reaches `main`, so every commit's type counts. A branch with three `chore:` commits and one `feat:` produces a minor bump.
- **Squash merge** — only the squashed subject counts, and GitHub fills it from the pull request title. The **PR title must then be a valid conventional commit**, or the release is silently wrong. A perfect set of commits squashed under "Update stuff" releases nothing.
- **Rebase merge** — like a merge commit: all of them count.

Check which one the repository uses before assuming the commits will speak for themselves.

**In this repository all three are enabled, and two are in active use** — recent
pull requests landed as merge commits, #165 as a squash. So neither rule can be
relied on alone, and the safe habit is the one that satisfies both: write
conventional-commit messages on the branch *and* a conventional-commit pull
request title. Under a merge commit the title costs nothing; under a squash it
is the only thing release-please sees.

## Pull request titles

Write them as conventional commits too, whatever the merge strategy. Under squash merges it is load-bearing; under merge commits it costs nothing and keeps the pull request list readable.

## Before opening a pull request

- Does the branch name start with a type, and is it the right type for the change?
- Is it free of the reserved release-branch names above?
- Does the pull request title parse as a conventional commit?
- If a breaking change is in there, does at least one commit carry `!` or a `BREAKING CHANGE:` footer? Missing that ships a breaking change as a minor bump.
