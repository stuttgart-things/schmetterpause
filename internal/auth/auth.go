// Package auth keeps authentication behind an interface.
//
// Invariant 4 in CLAUDE.md: no handler knows about GitLab, GitHub or WebAuthn
// directly. Everything outside this package works with a player_id passed
// through the request context. Which provider produced it is invisible beyond
// that boundary (docs/adr/0003).
package auth

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
)

// Authenticator determines the player behind a request.
//
// The MVP has exactly one implementation: recognition through a signed cookie
// (AP2). OIDC and WebAuthn arrive as further implementations without any
// handler changing.
type Authenticator interface {
	// Identify returns the player_id for a request. Nobody being signed in is
	// an ordinary state, reported as uuid.Nil with a nil error — only a
	// genuine failure, such as an unreachable database, returns an error.
	Identify(r *http.Request) (uuid.UUID, error)
}

// Anonymous recognises nobody. Useful in tests and wherever a server should
// run without a login implementation.
type Anonymous struct{}

// Identify recognises nobody.
func (Anonymous) Identify(*http.Request) (uuid.UUID, error) { return uuid.Nil, nil }

// SetCookie remembers nobody.
func (Anonymous) SetCookie(http.ResponseWriter, string) {}

// ClearCookie has nothing to clear.
func (Anonymous) ClearCookie(http.ResponseWriter) {}

// SessionAuthenticator is an Authenticator that can also start and end a
// session. Handlers that sign somebody in need this; everything else is
// happier with the narrower Authenticator.
type SessionAuthenticator interface {
	Authenticator
	// SetCookie writes the recognition cookie for subject.
	SetCookie(w http.ResponseWriter, subject string)
	// ClearCookie removes it.
	ClearCookie(w http.ResponseWriter)
}

var (
	_ SessionAuthenticator = Anonymous{}
	_ SessionAuthenticator = (*CookieAuthenticator)(nil)
)

type contextKey struct{}

// WithPlayerID stores the player_id in the context.
func WithPlayerID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

// PlayerID reads the player_id from the context. The second return value is
// false when the request is not attributed to any player.
func PlayerID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(contextKey{}).(uuid.UUID)
	return id, ok && id != uuid.Nil
}

// Middleware puts the identified player_id into the context. It turns nobody
// away — that is each handler's decision, via RequirePlayer.
//
// A failure to identify is logged and treated as anonymous rather than
// answered with a 500. If the database is down, the pages that need no player
// should still render, and /healthz must answer regardless.
func Middleware(a Authenticator, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, err := a.Identify(r)
			switch {
			case err != nil:
				log.WarnContext(r.Context(), "identifying the player failed",
					"path", r.URL.Path, "error", err)
			case id != uuid.Nil:
				r = r.WithContext(WithPlayerID(r.Context(), id))
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequirePlayer lets through only requests attributed to a player.
func RequirePlayer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := PlayerID(r.Context()); !ok {
			// User-facing text stays German; see CLAUDE.md.
			http.Error(w, "Kein Spieler zugeordnet", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
