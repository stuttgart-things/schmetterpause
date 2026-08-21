// Package auth keeps authentication behind an interface.
//
// Invariant 4 in CLAUDE.md: no handler knows about GitLab, GitHub or WebAuthn
// directly. Everything outside this package works with a player_id passed
// through the request context. Which provider produced it is invisible beyond
// that boundary (docs/adr/0003).
package auth

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// Authenticator determines the player behind a request.
//
// The MVP has exactly one implementation: recognition through a signed cookie
// (AP2). OIDC and WebAuthn arrive as further implementations without any
// handler changing.
type Authenticator interface {
	// Identify returns the player_id for a request. When nobody is signed in,
	// the second return value is false — which is not an error.
	Identify(r *http.Request) (uuid.UUID, bool)
}

// Anonymous recognises nobody. Placeholder for the scaffolding, so the server
// runs without a login implementation.
type Anonymous struct{}

// Identify recognises nobody.
func (Anonymous) Identify(*http.Request) (uuid.UUID, bool) { return uuid.Nil, false }

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
func Middleware(a Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if id, ok := a.Identify(r); ok {
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
