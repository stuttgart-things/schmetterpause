package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/repository"
)

// SessionCookieName is the cookie a returning player is recognised by.
const SessionCookieName = "schmetterpause_session"

// subjectBytes is the length of a freshly minted subject before encoding.
// 32 bytes is well past the point where guessing one is worth anyone's time.
const subjectBytes = 32

// ErrInvalidCookie reports a cookie whose signature does not match. It is not
// an error the player caused — an old cookie after a key rotation looks the
// same — so callers treat it as "nobody is signed in" and move on.
var ErrInvalidCookie = errors.New("session cookie is not valid")

// CookieAuthenticator recognises a player by a signed cookie.
//
// The cookie carries an opaque subject, not the player_id. Resolving it goes
// through the identities table, which is the same path OIDC and WebAuthn will
// take later (docs/adr/0003) — so adding those changes this package and
// nothing else.
type CookieAuthenticator struct {
	identities repository.IdentityRepository
	key        []byte
	// secure marks the cookie so browsers only send it over HTTPS. Off for
	// plain localhost, where there is no HTTPS to send it over.
	secure bool
}

var _ Authenticator = (*CookieAuthenticator)(nil)

// NewCookieAuthenticator builds the authenticator. The key signs cookies and
// comes from the environment; it must survive restarts, or every player is a
// stranger after a deployment.
func NewCookieAuthenticator(identities repository.IdentityRepository, key []byte, secure bool) *CookieAuthenticator {
	return &CookieAuthenticator{identities: identities, key: key, secure: secure}
}

// Identify resolves the signed cookie to a player.
//
// A missing, malformed or unknown cookie yields uuid.Nil and no error: not
// being signed in is an ordinary state, not a failure. Only a database
// problem produces an error.
func (a *CookieAuthenticator) Identify(r *http.Request) (uuid.UUID, error) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return uuid.Nil, nil
	}

	subject, err := a.verify(cookie.Value)
	if err != nil {
		return uuid.Nil, nil
	}

	player, err := a.identities.PlayerBy(r.Context(), domain.ProviderLocal, subject)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		// A validly signed cookie for a player that no longer exists. Same
		// outcome as no cookie at all.
		return uuid.Nil, nil
	case err != nil:
		return uuid.Nil, fmt.Errorf("resolve session cookie: %w", err)
	}
	return player.ID, nil
}

// SetCookie signs subject and writes the session cookie.
func (a *CookieAuthenticator) SetCookie(w http.ResponseWriter, subject string) {
	http.SetCookie(w, &http.Cookie{
		Name:  SessionCookieName,
		Value: a.sign(subject),
		Path:  "/",
		// A year. The whole point is that the player is not asked again;
		// a session that expires over a holiday defeats the exercise.
		MaxAge:   365 * 24 * 60 * 60,
		HttpOnly: true,
		Secure:   a.secure,
		// Lax, not Strict: a QR code at the table is a cross-site
		// navigation, and Strict would drop the cookie on exactly the
		// entry path the MVP is measuring (AP7).
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearCookie removes the session cookie.
func (a *CookieAuthenticator) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   a.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// sign returns "<subject>.<signature>".
func (a *CookieAuthenticator) sign(subject string) string {
	return subject + "." + base64.RawURLEncoding.EncodeToString(a.mac(subject))
}

// verify checks the signature and returns the subject it covers.
func (a *CookieAuthenticator) verify(value string) (string, error) {
	subject, signature, found := strings.Cut(value, ".")
	if !found || subject == "" {
		return "", ErrInvalidCookie
	}

	got, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return "", ErrInvalidCookie
	}

	// Constant time: a byte-by-byte comparison would leak how much of a
	// forged signature was right.
	if !hmac.Equal(got, a.mac(subject)) {
		return "", ErrInvalidCookie
	}
	return subject, nil
}

func (a *CookieAuthenticator) mac(subject string) []byte {
	m := hmac.New(sha256.New, a.key)
	m.Write([]byte(subject))
	return m.Sum(nil)
}

// NewSubject mints an opaque subject for the local provider.
//
// Deliberately not the player_id: the cookie should say nothing about the
// record behind it, and a later provider supplies its own subject anyway.
func NewSubject() string {
	var b [subjectBytes]byte
	if _, err := rand.Read(b[:]); err != nil {
		// The process has no usable entropy source. Handing out guessable
		// sessions would be worse than stopping.
		panic("crypto/rand unavailable: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}
