<!--
The title is the squashed commit subject, so it has to be a valid conventional
commit: `type(scope): description`, imperative. Merge commits and squash merges
are both in use here, and under a squash the title is the only message that
reaches main — a title that is not a conventional commit releases nothing, and
release-please says so by staying silent rather than by failing.
See `.claude/skills/branch-naming`.

English, per CLAUDE.md: code, operations and pull requests are English; the
product and what a player reads are German.

These headings are what this repository's pull requests already write. They are
prompts, not a form. Delete a heading you have nothing to put under rather than
answering it with "n/a" — an empty section says less than no section.
-->

Why this change exists: what was wrong or missing, and what a reviewer would
otherwise have to reconstruct from the diff.

## Changes

<!--
What moved, grouped so a reviewer knows where to look first — not a list of
files, which the diff already is.
-->

## Verified

<!--
What was actually run, and what it said. Not what ought to pass.

And what was *not* verified, and why. A pull request that names the check it
could not run locally is worth more than one that lets the reader assume all of
them did — this repository has been bitten twice by something that looked right
and was never checked (#177, #179).
-->

## Left open, deliberately

<!--
What this does not do, and what that leaves for whoever picks it up next.

If the change touches the data model, auth or deployment: which invariant in
CLAUDE.md and which decision in `docs/adr/` does it rest on? A change that
contradicts an ADR needs a new ADR with a `supersedes` reference — not a quiet
rebuild.
-->

<!--
Close the issue this finishes (`Closes #123`), or reference the one it belongs
to without closing it (`Refs #123`) when it is one step of several.
-->
