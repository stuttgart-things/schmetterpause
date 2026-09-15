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
#   policy/verify-image-signature.yaml  the gate at admission
#   docs/supply-chain.md                what an operator is told to expect
#
# Widen one of them and everything still passes — that is the point of the
# comparison. All four spell the signer as the same anchored regexp, so the
# comparison is string equality. The dangerous place is still the policy, for
# a reason that has nothing to do with spelling: its identities are
# alternatives, so a second entry beside the right one is a second signer the
# gate accepts, and no single line of it looks wrong.
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
	# The single value of a "key: value" line, also as the first key of a
	# YAML list item, or empty if there is not exactly one. Two disagreeing
	# copies in one file is the same defect as two disagreeing files.
	found=$(sed -n "s/^[ -]*$2: *//p" "$1" | sort -u)
	case $(printf '%s\n' "$found" | grep -c .) in
	1) printf '%s\n' "$found" ;;
	*) printf '' ;;
	esac
}

policy=policy/verify-image-signature.yaml

issuer_ci=$(one .github/workflows/ci.yml COSIGN_ISSUER)
issuer_task=$(one Taskfile.yml COSIGN_ISSUER)
issuer_policy=$(one "$policy" issuer)

identity_ci=$(one .github/workflows/ci.yml COSIGN_IDENTITY)
identity_task=$(one Taskfile.yml COSIGN_IDENTITY)
identity_policy=$(one "$policy" subjectRegExp)

# --- Every one of them was found -----------------------------------------
for pair in \
	"issuer_ci:${issuer_ci}" \
	"issuer_task:${issuer_task}" \
	"issuer_policy:${issuer_policy}" \
	"identity_ci:${identity_ci}" \
	"identity_task:${identity_task}" \
	"identity_policy:${identity_policy}"; do
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

# --- One identity ---------------------------------------------------------
if [ "${identity_ci}" != "${identity_task}" ] || [ "${identity_ci}" != "${identity_policy}" ]; then
	note "three signer identities, not one:"
	echo "  ci.yml:       ${identity_ci}" >&2
	echo "  Taskfile.yml: ${identity_task}" >&2
	echo "  policy:       ${identity_policy}" >&2
fi

# --- One signer at admission ----------------------------------------------
#
# one() reads a value that is the same everywhere it appears, so a second
# identity with the same issuer, or one spelled with subject: or
# subjectExpression: instead, would get past it. Counting the keys does not
# care about the values: one issuer and one subject is one signer.
signer_keys=$(grep -cE '^[ -]*(issuer|issuerRegExp|subject|subjectRegExp|subjectExpression):' "$policy" || true)
[ "${signer_keys}" -eq 2 ] ||
	note "${policy} has ${signer_keys} issuer and subject keys; one signer is two"

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
