// Package auth kapselt die Authentifizierung hinter einer Schnittstelle.
//
// Invariante 4 aus CLAUDE.md: Kein Handler kennt GitLab, GitHub oder WebAuthn
// direkt. Alles ausserhalb dieses Packages arbeitet mit einer player_id, die
// per Context weitergereicht wird. Welcher Provider sie geliefert hat, ist
// jenseits dieser Grenze unsichtbar (docs/adr/0003).
package auth

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// Authenticator ermittelt den Spieler hinter einer Anfrage.
//
// Im MVP gibt es genau eine Implementierung: die Wiedererkennung ueber ein
// signiertes Cookie (AP2). OIDC und WebAuthn kommen als weitere
// Implementierungen dazu, ohne dass ein Handler sich aendert.
type Authenticator interface {
	// Identify liefert die player_id zur Anfrage. Ist niemand angemeldet,
	// ist der zweite Rueckgabewert false — das ist kein Fehler.
	Identify(r *http.Request) (uuid.UUID, bool)
}

// Anonymous erkennt niemanden. Platzhalter fuer AP1, damit der Server ohne
// Login-Implementierung lauffaehig ist.
type Anonymous struct{}

// Identify erkennt niemanden.
func (Anonymous) Identify(*http.Request) (uuid.UUID, bool) { return uuid.Nil, false }

type contextKey struct{}

// WithPlayerID hinterlegt die player_id im Context.
func WithPlayerID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

// PlayerID liest die player_id aus dem Context. Der zweite Rueckgabewert ist
// false, wenn die Anfrage keinem Spieler zugeordnet ist.
func PlayerID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(contextKey{}).(uuid.UUID)
	return id, ok && id != uuid.Nil
}

// Middleware legt die erkannte player_id in den Context. Sie weist niemanden
// ab — das entscheidet der jeweilige Handler ueber RequirePlayer.
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

// RequirePlayer laesst nur Anfragen durch, denen ein Spieler zugeordnet ist.
func RequirePlayer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := PlayerID(r.Context()); !ok {
			http.Error(w, "Kein Spieler zugeordnet", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
