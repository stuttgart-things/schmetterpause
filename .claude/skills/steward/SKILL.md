---
name: steward
description: How to open and drive a pull request in this repository — which checks to run before pushing, how to regenerate templ output without churn, what the pull request body has to say, and how fast things get merged here. Use when opening a pull request, when watching one, or when reacting to CI or review comments on one.
---

This is what a generic pull-request routine gets wrong about this repository. It does not replace the review rules an agent already has; it corrects them where this repository does things its own way.

## Before you push

The pipeline runs `dagger call ci`, which needs a container runtime. **An agent sandbox has none**, so `task ci` is not available and running it is not the check. This is:

```sh
go generate ./...        # never `go tool templ generate` — see below
gofmt -l cmd internal web db
go build ./...
go test -race ./...      # what `task test` runs
```

Budget four minutes for the last one and do not take a long silence for a hang: the server suite runs Argon2id at its real cost, which is 35 seconds plain and about 230 under the race detector. `go test ./...` without `-race` is the fast loop while iterating; run the race pass once before pushing.

`task fmt` also runs `go mod tidy` and `templ fmt .`, which is worth doing when a `.templ` file changed.

There is no separate lint step to run locally; the pipeline's `lint / Repository Linting` job has not disagreed with `gofmt` yet.

## Regenerating templ output

**`go generate ./...`, always.** The `//go:generate` line lives in `internal/templates/view.go`, so it runs with that directory as its working directory and the generated files carry `FileName: 'layout.templ'`.

Running `go tool templ generate` from the repository root instead rewrites that field in *every* generated file to `internal/templates/layout.templ` — sixteen files of pure churn on top of the two you meant to change. It builds and it passes, and it makes the diff unreadable.

## The pull request body

There is a template at `.github/pull_request_template.md` and it is prompts rather than a form: delete a heading you have nothing to put under. Two of them carry more weight here than the wording suggests.

**"Verified" must say what was *not* verified.** The template names #177 and #179 as the two times something looked right and was never checked. A pull request that admits it could not run a check is worth more than one that lets the reader assume everything passed — and if the admission is "I could not see how this looks", fix that instead: see the `screenshots` skill, it takes a minute and needs no database.

**"Left open, deliberately" is where a decision you did not take goes.** This repository writes its reasoning down — in ADRs, in code comments, in test names. A thing you noticed and left alone belongs in the body with the reason, not in silence. If the reason is that a decision is the maintainer's (product copy, a visible design change), say that and name the options.

The title is load-bearing: both merge commits and squashes are in use here, so it has to parse as a conventional commit or a squash releases nothing. The `branch-naming` skill has the detail.

## How fast things merge here

Recent pull requests were squash-merged **within half an hour of opening**, before any review. Two consequences:

- Do not plan a long review conversation into the pull request. Put the reasoning in the body, because the body may be the only thing anybody reads.
- **Check whether it merged before you push follow-up work.** A merged pull request cannot take more commits, and its branch has been squashed into `main` — stacking on top replays the whole change. Restart from `main` (`git fetch origin main && git checkout -B <branch> origin/main`) and open a new pull request. If unmerged work is already on the branch, rebase it onto the new base rather than throwing it away.

## Branch names

The `branch-naming` skill asks for `<type>/<description>` and explicitly rejects `claude/whatever`. Agent sessions are pinned to a `claude/<name>` branch by their environment and cannot choose. That conflict is known and unresolved: follow the assigned branch, and make sure the **pull request title** carries the conventional-commit type, which is what release-please reads anyway.

## What review will be about

Not style — `gofmt` settles that. The things this repository sends back:

- **A comment that no longer matches the code.** Comments here carry reasoning and are load-bearing; a stale one is a defect. If a change makes a comment false, rewrite the comment in the same commit.
- **A decision taken quietly.** Anything that contradicts an ADR needs a new ADR with a `supersedes` reference. Anything that contradicts a *test's* stated reasoning needs the maintainer, not a test edit.
- **Language.** Code, comments, commits, issues, pull requests and operations docs in English; anything a player reads, plus `CLAUDE.md`, the ADRs and the plan docs, in German. CLAUDE.md has the full split.
- **Invariants.** The eight in CLAUDE.md are not guidance. A change that violates one is a question to ask before it is a diff to write.

## Guards rather than promises

When a bug turns out to have been invisible to the build, the fix here comes in two parts: the fix, and a mechanical test that would have caught it. There are three already — `TestNoTableCellWearsAClassThatSetsDisplay`, `TestEveryClassInAMarkupFileHasARule`, `TestEveryPageOpensWithExactlyOneH1` — and each names the bug it guards in its doc comment and explains why nothing else could see it. Write the next one the same way, and check it in both directions: reintroduce the bug, watch it fail by name, then put it back.
