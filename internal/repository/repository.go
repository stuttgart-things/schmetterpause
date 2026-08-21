// Package repository defines data access as an interface.
//
// Invariant 5 in CLAUDE.md: no SQL in handlers. That keeps open the migration
// path described in docs/adr/0001 (Postgres to SQLite, should the portability
// goal ever be dropped) and makes handlers testable without a database.
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// Store bundles the repositories and the transaction boundary.
type Store interface {
	Players() PlayerRepository
	Identities() IdentityRepository
	Matches() MatchRepository
	TTRHistory() TTRHistoryRepository

	// InTx runs fn inside a transaction. The Store handed to fn writes on
	// that same transaction; if fn returns an error, everything rolls back.
	// Needed for AP5: writing the rating, appending the history and
	// confirming the match must all succeed or all fail.
	InTx(ctx context.Context, fn func(Store) error) error

	// Ping checks that the backend is reachable. Its consumer is /readyz.
	Ping(ctx context.Context) error
}

// PlayerRepository manages players.
type PlayerRepository interface {
	Create(ctx context.Context, displayName string, initialTTR int) (domain.Player, error)
	ByID(ctx context.Context, id uuid.UUID) (domain.Player, error)
	// List returns all players by descending rating — the order the ranking
	// in AP6 needs.
	List(ctx context.Context) ([]domain.Player, error)
	// Records returns every player with their confirmed match tally, in the
	// same order as List. The counting is the database's job: a Go loop over
	// every match to answer "how many has she won" is the shape this
	// interface exists to avoid.
	Records(ctx context.Context) ([]domain.PlayerRecord, error)
	Count(ctx context.Context) (int, error)
	UpdateTTR(ctx context.Context, id uuid.UUID, ttr int) error
}

// IdentityRepository links provider proofs to players. Outside this interface
// and the auth package, nobody knows the concrete providers (invariant 4).
type IdentityRepository interface {
	// Link records the association. If it already exists for the same player,
	// the call is a no-op.
	Link(ctx context.Context, provider domain.Provider, subject string, playerID uuid.UUID) error
	// PlayerBy returns the player behind a proof. When none is linked, the
	// error is domain.ErrNotFound.
	PlayerBy(ctx context.Context, provider domain.Provider, subject string) (domain.Player, error)
	ForPlayer(ctx context.Context, playerID uuid.UUID) ([]domain.Identity, error)
}

// MatchRepository manages encounters along with their sets.
type MatchRepository interface {
	// Create stores match and sets together and returns the persisted state,
	// including the assigned ID.
	Create(ctx context.Context, m domain.Match) (domain.Match, error)
	ByID(ctx context.Context, id uuid.UUID) (domain.Match, error)
	// PendingFor returns the matches waiting on this player's confirmation —
	// that is, the ones somebody else recorded.
	PendingFor(ctx context.Context, playerID uuid.UUID) ([]domain.Match, error)
	// RecentFor returns a player's most recent matches.
	RecentFor(ctx context.Context, playerID uuid.UUID, limit int) ([]domain.Match, error)
	// SetStatus writes the confirmation state. confirmedAt is set exactly
	// when status is domain.MatchConfirmed.
	SetStatus(ctx context.Context, id uuid.UUID, status domain.MatchStatus, confirmedAt *time.Time) error
}

// TTRHistoryRepository writes and reads the rating history.
type TTRHistoryRepository interface {
	// Append writes the entries of one rating event. Callers use InTx so the
	// history and the player rating cannot drift apart.
	Append(ctx context.Context, changes []domain.TTRChange) error
	ForPlayer(ctx context.Context, playerID uuid.UUID, limit int) ([]domain.TTRChange, error)
}
