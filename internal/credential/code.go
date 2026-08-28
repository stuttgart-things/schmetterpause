// Package credential generates and checks the shared secrets a player proves
// themselves with: the recovery code from docs/adr/0006 and the PIN from
// docs/adr/0007.
//
// Like the rest of the business logic it knows neither the database nor HTTP
// (CLAUDE.md, "Konventionen"). It hands out strings and verifies them; who
// stores them is somebody else's problem.
package credential

import (
	"crypto/rand"
	"strings"
)

// alphabet is Crockford's base32: the digits and the upper-case letters, less
// I, L, O and U. That is the answer to open point 1 in docs/adr/0006 — the
// code gets read aloud and typed, so the pairs that get confused when it does
// (0/O, 1/I, 1/L) cannot both be in it. U is out because leaving it out is
// what keeps a random code from spelling something.
const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// codeLen is how many characters a recovery code carries. Sixteen of this
// alphabet is 80 bits, which is far past what the rate limit already makes
// unguessable — and short enough to read down a phone line in four groups.
const codeLen = 16

// groupLen is how many characters stand together between the hyphens. Four is
// the length people read and type as one chunk; the hyphens are decoration
// and NormalizeCode throws them away again.
const groupLen = 4

// The bounds on a PIN. docs/adr/0007 sets the floor at six digits, which only
// holds up because of the brake in front of it; the ceiling is here so the
// field cannot become an essay, and nothing needs it.
//
// They live in this package rather than beside the handler because the form
// that collects a PIN needs them too, and two copies of a rule drift.
const (
	MinPINLength = 6
	MaxPINLength = 32
)

// NewCode returns a fresh recovery code, formatted for reading aloud.
//
// Generated, never chosen. docs/adr/0006 is explicit about why: a field
// somebody may type their own secret into becomes a field somebody types
// their company password into.
func NewCode() string {
	var b [codeLen]byte
	if _, err := rand.Read(b[:]); err != nil {
		// The process has no usable entropy source. Handing out guessable
		// codes would be worse than stopping. Same trade as auth.NewSubject.
		panic("crypto/rand unavailable: " + err.Error())
	}

	var sb strings.Builder
	sb.Grow(codeLen + codeLen/groupLen)
	for i, v := range b {
		if i > 0 && i%groupLen == 0 {
			sb.WriteByte('-')
		}
		// len(alphabet) is 32 and 256 is a multiple of it, so the remainder
		// is uniform — no modulo bias to work around.
		sb.WriteByte(alphabet[int(v)%len(alphabet)])
	}
	return sb.String()
}

// NewRecoveryCode returns a fresh code to show and the hash to store.
//
// The two come from one call because they must not be derived apart. The hash
// covers the *normalized* code, which is what a sign-in compares against after
// running what somebody typed through NormalizeCode. Hashing the formatted
// string instead would reject every code the moment it was typed back without
// its hyphens — which is how most people would type it.
func NewRecoveryCode() (code, hash string) {
	code = NewCode()
	return code, Hash(NormalizeCode(code))
}

// NormalizeCode turns what somebody typed into what was generated.
//
// It folds case, drops the hyphens and anything else that is not part of the
// alphabet, and maps the characters the alphabet leaves out onto the ones it
// kept: O to zero, I and L to one. Somebody reading a code off a screen and
// typing "O" where a "0" stands is the expected case, not an attack, and a
// sign-in that refuses it for that is refusing the right person.
//
// The result is not checked for length or plausibility. That belongs to
// whoever compares it against a hash, where a wrong code and a mistyped one
// have to look the same anyway.
func NormalizeCode(raw string) string {
	var sb strings.Builder
	sb.Grow(len(raw))

	for _, r := range strings.ToUpper(raw) {
		switch r {
		case 'O':
			r = '0'
		case 'I', 'L':
			r = '1'
		}
		if strings.ContainsRune(alphabet, r) {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}
