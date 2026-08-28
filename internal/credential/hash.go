package credential

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// The Argon2id cost. docs/adr/0007 decided the algorithm and says why a keyed
// hash was not an option: key and database live in the same environment and
// land in the same backup, and six digits is a million values to anyone who
// holds both. Argon2id keeps a price per guess even then.
//
// 64 MiB with two passes lands around a tenth of a second on the laptop under
// the table, which is the machine that matters — sign-in is a rare request
// and the rate limit bounds how often it can be asked for at all.
const (
	argonTime    = 2
	argonMemory  = 64 * 1024 // KiB
	argonThreads = 4
	argonKeyLen  = 32
	saltLen      = 16
)

// argonVersion is the Argon2 version the parameters above are written for.
// It travels in the encoded hash so a future version can be told apart rather
// than silently mis-verified.
const argonVersion = argon2.Version

// ErrMalformedHash reports a stored hash this package cannot read. It means
// the row is broken, not that the secret was wrong, and callers have to keep
// the two apart: answering "wrong code" to a corrupted row would send
// somebody looking for a code that was right all along.
var ErrMalformedHash = errors.New("stored credential hash is malformed")

// b64 is the encoding both halves of the hash are written in: standard
// alphabet, no padding, as every other Argon2 implementation writes it.
var b64 = base64.RawStdEncoding

// Hash derives the digest that gets stored for secret.
//
// The result carries its own parameters and salt, in the format Argon2
// implementations agree on:
//
//	$argon2id$v=19$m=65536,t=2,p=4$<salt>$<digest>
//
// Self-describing on purpose. Raising the cost later has to leave the rows
// written before it verifiable, or the change locks out everybody who has not
// signed in since.
func Hash(secret string) string {
	var salt [saltLen]byte
	if _, err := rand.Read(salt[:]); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}

	digest := argon2.IDKey([]byte(secret), salt[:], argonTime, argonMemory, argonThreads, argonKeyLen)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argonVersion, argonMemory, argonTime, argonThreads,
		b64.EncodeToString(salt[:]), b64.EncodeToString(digest))
}

// Verify reports whether secret is the one encoded stands for.
//
// A wrong secret is (false, nil) — an ordinary answer, not a failure. Only an
// unreadable encoded hash returns an error.
func Verify(encoded, secret string) (bool, error) {
	params, salt, want, err := parse(encoded)
	if err != nil {
		return false, err
	}

	got := argon2.IDKey([]byte(secret), salt, params.time, params.memory, params.threads, uint32(len(want)))

	// Constant time: a byte-by-byte comparison would leak how much of a
	// guessed secret was right, one request at a time.
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

type params struct {
	memory  uint32
	time    uint32
	threads uint8
}

// parse pulls the parameters, the salt and the digest back out of an encoded
// hash.
func parse(encoded string) (params, []byte, []byte, error) {
	// The leading "$" makes the first field empty, so there are six.
	fields := strings.Split(encoded, "$")
	if len(fields) != 6 || fields[0] != "" {
		return params{}, nil, nil, fmt.Errorf("%w: expected six fields, got %d", ErrMalformedHash, len(fields))
	}

	if fields[1] != "argon2id" {
		// Deliberately not a fallback to another algorithm. Nothing in this
		// application ever wrote one, so a different name means a row that
		// came from somewhere it should not have.
		return params{}, nil, nil, fmt.Errorf("%w: algorithm %q is not argon2id", ErrMalformedHash, fields[1])
	}

	var version int
	if _, err := fmt.Sscanf(fields[2], "v=%d", &version); err != nil {
		return params{}, nil, nil, fmt.Errorf("%w: unreadable version %q", ErrMalformedHash, fields[2])
	}
	if version != argonVersion {
		return params{}, nil, nil, fmt.Errorf("%w: argon2 version %d, this build speaks %d",
			ErrMalformedHash, version, argonVersion)
	}

	var p params
	if _, err := fmt.Sscanf(fields[3], "m=%d,t=%d,p=%d", &p.memory, &p.time, &p.threads); err != nil {
		return params{}, nil, nil, fmt.Errorf("%w: unreadable parameters %q", ErrMalformedHash, fields[3])
	}
	if p.memory == 0 || p.time == 0 || p.threads == 0 {
		return params{}, nil, nil, fmt.Errorf("%w: parameters %q include a zero", ErrMalformedHash, fields[3])
	}

	salt, err := b64.DecodeString(fields[4])
	if err != nil {
		return params{}, nil, nil, fmt.Errorf("%w: salt is not base64: %w", ErrMalformedHash, err)
	}

	digest, err := b64.DecodeString(fields[5])
	if err != nil {
		return params{}, nil, nil, fmt.Errorf("%w: digest is not base64: %w", ErrMalformedHash, err)
	}
	if len(salt) == 0 || len(digest) == 0 {
		return params{}, nil, nil, fmt.Errorf("%w: salt or digest is empty", ErrMalformedHash)
	}

	return p, salt, digest, nil
}
