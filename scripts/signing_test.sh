#!/bin/sh
# Checks that the four places naming the signing identity still name the same
# one.
#
# Issue #230 asks for a verification that can refuse something. The way that
# claim dies quietly is not a missing check — it is four copies of "who may
# have signed this" that stopped agreeing:
#
#   .github/workflows/ci.yml            the gate in the delivery path
#   Taskfile.yml                        the same check, run by a person
#   policy/verify-image-signature.yaml  the gate at admission, as a glob
#   docs/supply-chain.md                what an operator is told to expect
#
# Widen one of them and everything still passes — that is the point of the
# comparison. The dangerous direction is the Kyverno subject: it is a glob and
# the others are an anchored regexp, so "the same string" is not something a
# reader checks by looking. `*` where the regexp has none accepts a workflow
# file nobody wrote.
#
# Runs in the Lint stage of the pipeline, so it uses nothing the Go image does
# not have — no YAML parser, sed only, the same constraint scripts/docs_test.sh
# works under.
set -eu

cd "$(dirname "$0")/.."

fail=0

note() {
	echo "FAIL: $1" >&2
	fail=1
}

one() {
	# The single value of a "key: value" line, or empty if there is not
	# exactly one. Two disagreeing copies in one file is the same defect as
	# two disagreeing files.
	found=$(sed -n "s/^ *$2: *//p" "$1" | sort -u)
	case $(printf '%s\n' "$found" | grep -c .) in
	1) printf '%s\n' "$found" ;;
	*) printf '' ;;
	esac
}

issuer_ci=$(one .github/workflows/ci.yml COSIGN_ISSUER)
issuer_task=$(one Taskfile.yml COSIGN_ISSUER)
issuer_policy=$(one policy/verify-image-signature.yaml issuer)

identity_ci=$(one .github/workflows/ci.yml COSIGN_IDENTITY)
identity_task=$(one Taskfile.yml COSIGN_IDENTITY)

# The Kyverno subject is a folded scalar, so it is the indented line that
# carries it rather than a "key: value" pair. Matched as loosely as the file
# allows — any bare URL on its own line — so that a subject somebody widened
# is read and compared rather than missed, which would fail with a message
# about a renamed key instead of the one that is true.
subject_policy=$(sed -n \
	's|^ *\(https://github\.com/[^ ]*\) *$|\1|p' \
	policy/verify-image-signature.yaml)

# --- Every one of them was found -----------------------------------------
for pair in \
	"issuer_ci:${issuer_ci}" \
	"issuer_task:${issuer_task}" \
	"issuer_policy:${issuer_policy}" \
	"identity_ci:${identity_ci}" \
	"identity_task:${identity_task}" \
	"subject_policy:${subject_policy}"; do
	[ -n "${pair#*:}" ] ||
		note "could not read ${pair%%:*} — has a key been renamed, or is it there twice with two values?"
done

[ "$fail" -eq 0 ] || exit 1

# --- One issuer -----------------------------------------------------------
if [ "${issuer_ci}" != "${issuer_task}" ] || [ "${issuer_ci}" != "${issuer_policy}" ]; then
	note "three issuers, not one:"
	echo "  ci.yml:       ${issuer_ci}" >&2
	echo "  Taskfile.yml: ${issuer_task}" >&2
	echo "  policy:       ${issuer_policy}" >&2
fi

# --- One identity, in both spellings --------------------------------------
[ "${identity_ci}" = "${identity_task}" ] ||
	note "ci.yml verifies ${identity_ci}, Taskfile.yml verifies ${identity_task}"

# The glob without its trailing "*", with every "." escaped and anchored at
# the front, is what the regexp has to be. Derived rather than written out, so
# this file is not a fifth copy to keep in step.
expected="^$(printf '%s' "${subject_policy%\*}" | sed 's/\./\\./g')"
if [ "${identity_ci}" != "${expected}" ]; then
	note "the admission policy and the cosign check do not describe the same signer:"
	echo "  policy subject: ${subject_policy}" >&2
	echo "  which means:    ${expected}" >&2
	echo "  ci.yml says:    ${identity_ci}" >&2
fi

# --- The operator is told the same thing ----------------------------------
grep -qF "${issuer_ci}" docs/supply-chain.md ||
	note "docs/supply-chain.md does not name the issuer ${issuer_ci}"
grep -qF "${identity_ci}" docs/supply-chain.md ||
	note "docs/supply-chain.md does not name the identity ${identity_ci}"

# --- The gate still proves it can refuse ----------------------------------
#
# A verification that has never refused anything is indistinguishable from one
# that cannot, so ci.yml runs the same check against an identity that must not
# match and fails if it passes. Deleting that step leaves a green pipeline and
# no way to tell the gate stopped being one.
grep -q 'not-this-repository' .github/workflows/ci.yml ||
	note "ci.yml no longer checks that verification refuses a wrong identity"

[ "$fail" -eq 0 ] || exit 1

echo "signing: one issuer, one signer identity, and a gate that still refuses"
