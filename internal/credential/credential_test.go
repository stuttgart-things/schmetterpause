package credential_test

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"

	"github.com/stuttgart-things/schmetterpause/internal/credential"
)

func TestNewCodeShape(t *testing.T) {
	t.Parallel()

	code := credential.NewCode()

	groups := strings.Split(code, "-")
	if len(groups) != 4 {
		t.Fatalf("NewCode() = %q, want four hyphen-separated groups", code)
	}
	for i, group := range groups {
		if len(group) != 4 {
			t.Errorf("NewCode() group %d = %q, want four characters", i, group)
		}
	}
}

// The alphabet is the half of the decision docs/adr/0006 spelled out: a code
// that gets read aloud must not contain a character somebody hears as another
// one. A test rather than a comment, because the constant is easy to widen by
// accident.
func TestNewCodeAvoidsConfusableCharacters(t *testing.T) {
	t.Parallel()

	const forbidden = "ILOU"

	for range 200 {
		code := strings.ReplaceAll(credential.NewCode(), "-", "")
		if i := strings.IndexAny(code, forbidden); i >= 0 {
			t.Fatalf("NewCode() = %q, contains %q, which is not in the alphabet",
				code, code[i])
		}
		for _, r := range code {
			if !strings.ContainsRune("0123456789ABCDEFGHJKMNPQRSTVWXYZ", r) {
				t.Fatalf("NewCode() = %q, contains %q, which is outside the alphabet", code, r)
			}
		}
	}
}

func TestNewCodeIsNotRepeated(t *testing.T) {
	t.Parallel()

	seen := make(map[string]bool, 500)
	for range 500 {
		code := credential.NewCode()
		if seen[code] {
			t.Fatalf("NewCode() returned %q twice in 500 draws", code)
		}
		seen[code] = true
	}
}

func TestNormalizeCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"as generated", "A1B2-C3D4-E5F6-G7H8", "A1B2C3D4E5F6G7H8"},
		{"lower case", "a1b2-c3d4", "A1B2C3D4"},
		{"spaces instead of hyphens", "A1B2 C3D4", "A1B2C3D4"},
		{"no separators at all", "A1B2C3D4", "A1B2C3D4"},
		// The characters the alphabet leaves out are the ones somebody types
		// when they read the ones it kept.
		{"letter O for zero", "OOOO", "0000"},
		{"letter I for one", "IIII", "1111"},
		{"letter l for one", "llll", "1111"},
		{"punctuation is dropped", "A1B2/C3D4.", "A1B2C3D4"},
		{"empty stays empty", "", ""},
		{"nothing usable", "   ---   ", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := credential.NormalizeCode(tc.in); got != tc.want {
				t.Errorf("NormalizeCode(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// Whatever NewCode hands out has to survive being typed back in, hyphens and
// all. If this ever fails, every player who kept their code is locked out.
func TestNormalizeCodeAcceptsWhatNewCodeProduces(t *testing.T) {
	t.Parallel()

	for range 50 {
		code := credential.NewCode()
		want := strings.ReplaceAll(code, "-", "")

		for _, typed := range []string{code, strings.ToLower(code), want, strings.ReplaceAll(code, "-", " ")} {
			if got := credential.NormalizeCode(typed); got != want {
				t.Errorf("NormalizeCode(%q) = %q, want %q", typed, got, want)
			}
		}
	}
}

func TestHashVerifies(t *testing.T) {
	t.Parallel()

	const secret = "A1B2C3D4E5F6G7H8"

	ok, err := credential.Verify(credential.Hash(secret), secret)
	if err != nil {
		t.Fatalf("Verify() error = %v, want nil", err)
	}
	if !ok {
		t.Error("Verify() = false for the secret that was hashed, want true")
	}
}

func TestVerifyRejectsWrongSecret(t *testing.T) {
	t.Parallel()

	encoded := credential.Hash("A1B2C3D4E5F6G7H8")

	for _, wrong := range []string{"", "A1B2C3D4E5F6G7H9", "a1b2c3d4e5f6g7h8", "123456"} {
		ok, err := credential.Verify(encoded, wrong)
		if err != nil {
			t.Errorf("Verify(_, %q) error = %v, want nil — a wrong secret is an answer, not a failure",
				wrong, err)
		}
		if ok {
			t.Errorf("Verify(_, %q) = true, want false", wrong)
		}
	}
}

// A salt per row is what docs/adr/0007 chose over a keyed hash. Two players
// with the same PIN must not end up with the same row, or one precomputation
// covers both.
func TestHashSaltsEveryCall(t *testing.T) {
	t.Parallel()

	const secret = "123456"

	first, second := credential.Hash(secret), credential.Hash(secret)
	if first == second {
		t.Fatal("Hash() returned the same encoding twice for the same secret, so it is not salting")
	}

	for _, encoded := range []string{first, second} {
		ok, err := credential.Verify(encoded, secret)
		if err != nil || !ok {
			t.Errorf("Verify(%q) = %v, %v — both encodings must verify", encoded, ok, err)
		}
	}
}

func TestHashEncoding(t *testing.T) {
	t.Parallel()

	encoded := credential.Hash("123456")

	fields := strings.Split(encoded, "$")
	if len(fields) != 6 || fields[0] != "" {
		t.Fatalf("Hash() = %q, want the six-field $argon2id$… encoding", encoded)
	}
	if fields[1] != "argon2id" {
		t.Errorf("Hash() names algorithm %q, want argon2id", fields[1])
	}
	if fields[2] != "v=19" {
		t.Errorf("Hash() names version %q, want v=19", fields[2])
	}
	if !strings.HasPrefix(fields[3], "m=") {
		t.Errorf("Hash() parameters = %q, want them spelled out", fields[3])
	}

	// The secret must not be anywhere in what gets stored, in any form.
	if strings.Contains(encoded, "123456") {
		t.Errorf("Hash() = %q contains the secret in the clear", encoded)
	}
}

// The encoding carries its own cost so that raising it later leaves the rows
// written before it verifiable. Without this, a parameter change locks out
// everybody who has not signed in since.
func TestVerifyReadsTheParametersFromTheHash(t *testing.T) {
	t.Parallel()

	const secret = "A1B2C3D4E5F6G7H8"

	// Deliberately not this build's parameters: a cheaper Argon2id, as an
	// older row would have been written with.
	var (
		salt   = []byte("sixteen-byte-sal")
		digest = argon2.IDKey([]byte(secret), salt, 1, 8*1024, 1, 32)
		b64    = base64.RawStdEncoding
	)
	encoded := fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		8*1024, 1, 1, b64.EncodeToString(salt), b64.EncodeToString(digest))

	ok, err := credential.Verify(encoded, secret)
	if err != nil {
		t.Fatalf("Verify() error = %v, want nil", err)
	}
	if !ok {
		t.Error("Verify() = false for a hash written with other parameters, want true")
	}
}

func TestVerifyRejectsMalformedHash(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		encoded string
	}{
		{"empty", ""},
		{"not the format", "just-a-string"},
		{"too few fields", "$argon2id$v=19$m=65536,t=2,p=4$c2FsdA"},
		{"another algorithm", "$argon2i$v=19$m=65536,t=2,p=4$c2FsdA$aGFzaA"},
		{"unknown version", "$argon2id$v=16$m=65536,t=2,p=4$c2FsdA$aGFzaA"},
		{"unreadable parameters", "$argon2id$v=19$m=lots,t=2,p=4$c2FsdA$aGFzaA"},
		{"zero memory", "$argon2id$v=19$m=0,t=2,p=4$c2FsdA$aGFzaA"},
		{"salt is not base64", "$argon2id$v=19$m=65536,t=2,p=4$not base64!$aGFzaA"},
		{"empty digest", "$argon2id$v=19$m=65536,t=2,p=4$c2FsdA$"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ok, err := credential.Verify(tc.encoded, "123456")
			if ok {
				t.Error("Verify() = true for a hash it cannot read")
			}
			// A broken row and a wrong secret must not look the same to the
			// caller: one of them means somebody should stop guessing, the
			// other means somebody should look at the database.
			if !errors.Is(err, credential.ErrMalformedHash) {
				t.Errorf("Verify() error = %v, want ErrMalformedHash", err)
			}
		})
	}
}

// The hash has to cover the code as it will be typed back, not as it is
// printed. Getting this backwards would leave every saved code rejected at
// the moment it matters, and nothing before then would show it.
func TestNewRecoveryCodeHashesTheNormalizedForm(t *testing.T) {
	t.Parallel()

	code, hash := credential.NewRecoveryCode()

	typed := []string{
		code,
		strings.ReplaceAll(code, "-", ""),
		strings.ToLower(code),
		strings.ReplaceAll(code, "-", " "),
	}
	for _, in := range typed {
		ok, err := credential.Verify(hash, credential.NormalizeCode(in))
		if err != nil {
			t.Fatalf("Verify() error = %v, want nil", err)
		}
		if !ok {
			t.Errorf("the code typed as %q does not verify against its own hash", in)
		}
	}

	// And the printed form must not verify on its own, or the normalization
	// step would be doing nothing.
	if ok, _ := credential.Verify(hash, code); ok {
		t.Error("the hyphenated code verifies unnormalized, so NormalizeCode is not load-bearing")
	}
}
