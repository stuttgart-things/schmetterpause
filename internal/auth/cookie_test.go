package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/repository"
)

var testKey = []byte("0123456789abcdef0123456789abcdef")

// fakeIdentities resolves exactly one subject. Everything else is unknown.
type fakeIdentities struct {
	repository.IdentityRepository
	subject string
	player  domain.Player
	err     error
}

func (f fakeIdentities) PlayerBy(_ context.Context, provider domain.Provider, subject string) (domain.Player, error) {
	switch {
	case f.err != nil:
		return domain.Player{}, f.err
	case provider == domain.ProviderLocal && subject == f.subject:
		return f.player, nil
	}
	return domain.Player{}, domain.ErrNotFound
}

func requestWithCookie(value string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if value != "" {
		r.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: value})
	}
	return r
}

// signedCookieFor runs a subject through SetCookie and returns the value a
// browser would send back, so the tests exercise the real signing path
// instead of a reimplementation of it.
func signedCookieFor(t *testing.T, a *auth.CookieAuthenticator, subject string) string {
	t.Helper()

	rec := httptest.NewRecorder()
	a.SetCookie(rec, subject)

	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.SessionCookieName {
			return c.Value
		}
	}
	t.Fatal("SetCookie wrote no session cookie")
	return ""
}

func TestIdentifyRecognisesItsOwnCookie(t *testing.T) {
	player := domain.Player{ID: uuid.New(), DisplayName: "Anna"}
	subject := auth.NewSubject()
	a := auth.NewCookieAuthenticator(fakeIdentities{subject: subject, player: player}, testKey, true)

	got, err := a.Identify(requestWithCookie(signedCookieFor(t, a, subject)))

	if err != nil {
		t.Fatalf("Identify(): %v", err)
	}
	if got != player.ID {
		t.Errorf("Identify() = %s, want %s", got, player.ID)
	}
}

// TestIdentifyRejectsTampering is the point of signing the cookie at all: the
// subject is visible to whoever holds it, so it has to be unusable once
// changed.
func TestIdentifyRejectsTampering(t *testing.T) {
	player := domain.Player{ID: uuid.New(), DisplayName: "Anna"}
	subject := auth.NewSubject()
	a := auth.NewCookieAuthenticator(fakeIdentities{subject: subject, player: player}, testKey, true)
	valid := signedCookieFor(t, a, subject)

	tests := []struct {
		name  string
		value string
	}{
		{"empty", ""},
		{"no signature", subject},
		{"signature dropped", subject + "."},
		{"signature replaced", subject + ".AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
		{"signature not base64", subject + ".not base64"},
		{"subject swapped, signature kept", auth.NewSubject() + valid[len(subject):]},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := a.Identify(requestWithCookie(tc.value))

			if err != nil {
				t.Fatalf("Identify() = %v, want no error — a bad cookie is not a failure", err)
			}
			if got != uuid.Nil {
				t.Errorf("Identify() = %s, want uuid.Nil", got)
			}
		})
	}
}

// TestIdentifyRejectsAnotherKeysCookie covers key rotation: cookies signed
// with the old key stop working, they do not become someone else's session.
func TestIdentifyRejectsAnotherKeysCookie(t *testing.T) {
	player := domain.Player{ID: uuid.New(), DisplayName: "Anna"}
	subject := auth.NewSubject()
	identities := fakeIdentities{subject: subject, player: player}

	old := auth.NewCookieAuthenticator(identities, []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), true)
	current := auth.NewCookieAuthenticator(identities, []byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"), true)

	got, err := current.Identify(requestWithCookie(signedCookieFor(t, old, subject)))

	if err != nil {
		t.Fatalf("Identify(): %v", err)
	}
	if got != uuid.Nil {
		t.Errorf("Identify() = %s, want uuid.Nil", got)
	}
}

// TestIdentifyWithNoCookie is the ordinary case for a first visit.
func TestIdentifyWithNoCookie(t *testing.T) {
	a := auth.NewCookieAuthenticator(fakeIdentities{}, testKey, true)

	got, err := a.Identify(httptest.NewRequest(http.MethodGet, "/", nil))

	if err != nil || got != uuid.Nil {
		t.Errorf("Identify() = (%s, %v), want (uuid.Nil, nil)", got, err)
	}
}

// TestIdentifyForwardsDatabaseErrors separates "nobody is signed in" from
// "the lookup failed" — the middleware treats them differently.
func TestIdentifyForwardsDatabaseErrors(t *testing.T) {
	subject := auth.NewSubject()
	boom := errors.New("database gone")
	a := auth.NewCookieAuthenticator(fakeIdentities{err: boom}, testKey, true)

	_, err := a.Identify(requestWithCookie(signedCookieFor(t, a, subject)))

	if !errors.Is(err, boom) {
		t.Errorf("Identify() = %v, want the database error wrapped", err)
	}
}

// TestIdentifyWithAnUnknownSubject covers a valid cookie for a player that no
// longer exists — a deleted account, or a database reset in development.
func TestIdentifyWithAnUnknownSubject(t *testing.T) {
	a := auth.NewCookieAuthenticator(fakeIdentities{subject: "someone-else"}, testKey, true)

	got, err := a.Identify(requestWithCookie(signedCookieFor(t, a, auth.NewSubject())))

	if err != nil || got != uuid.Nil {
		t.Errorf("Identify() = (%s, %v), want (uuid.Nil, nil)", got, err)
	}
}

func TestSetCookieFlags(t *testing.T) {
	rec := httptest.NewRecorder()
	auth.NewCookieAuthenticator(fakeIdentities{}, testKey, true).SetCookie(rec, auth.NewSubject())

	c := rec.Result().Cookies()[0]

	if !c.HttpOnly {
		t.Error("HttpOnly = false; script must not be able to read the session")
	}
	if !c.Secure {
		t.Error("Secure = false although the authenticator was built for HTTPS")
	}
	// Lax, not Strict: the QR code at the table is a cross-site navigation,
	// and Strict would drop the cookie on exactly that path (AP7).
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", c.SameSite)
	}
	if c.MaxAge <= 0 {
		t.Errorf("MaxAge = %d; a session cookie would not survive closing the browser", c.MaxAge)
	}
}

func TestSetCookieWithoutHTTPS(t *testing.T) {
	rec := httptest.NewRecorder()
	auth.NewCookieAuthenticator(fakeIdentities{}, testKey, false).SetCookie(rec, auth.NewSubject())

	if rec.Result().Cookies()[0].Secure {
		t.Error("Secure = true although the authenticator was built for plain HTTP")
	}
}

func TestClearCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	auth.NewCookieAuthenticator(fakeIdentities{}, testKey, true).ClearCookie(rec)

	c := rec.Result().Cookies()[0]

	if c.Value != "" || c.MaxAge >= 0 {
		t.Errorf("ClearCookie() wrote %q with MaxAge %d, want an expiring empty cookie", c.Value, c.MaxAge)
	}
}

func TestNewSubjectIsUnique(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for range 1000 {
		s := auth.NewSubject()
		if seen[s] {
			t.Fatalf("NewSubject() repeated %q", s)
		}
		seen[s] = true
	}
}
