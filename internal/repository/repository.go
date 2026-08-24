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
	// PendingFor returns the matches waiting on this player: the pending ones
	// somebody else recorded, and the contested ones they played in, which
	// are waiting on either side to say what the result really was.
	PendingFor(ctx context.Context, playerID uuid.UUID) ([]domain.Match, error)
	// PendingCountFor is how many of those there are. The badge in the top
	// bar needs a number and not the matches behind it, and asking for the
	// list on every page render would load their sets along with it.
	PendingCountFor(ctx context.Context, playerID uuid.UUID) (int, error)
	// RecentFor returns a player's most recent matches.
	RecentFor(ctx context.Context, playerID uuid.UUID, limit int) ([]domain.Match, error)
	// Recent returns the most recent matches of everybody, newest first, at
	// most limit of them. Every status, because a list that quietly left out
	// what is still waiting would answer "where is my match" with silence.
	Recent(ctx context.Context, limit int) ([]domain.Match, error)
	// Delete removes a confirmed match along with its sets and its rating
	// history, which the schema cascades. It exists for one case: a result
	// entered at the kiosk counts at once, so a mistyped one has to be
	// removable rather than corrected — there is nothing left to correct.
	//
	// It refuses anything that is not confirmed, so it cannot be used as a
	// way around the confirmation path.
	Delete(ctx context.Context, id uuid.UUID) error
	// SetStatus writes the confirmation state. confirmedAt is set exactly
	// when status is domain.MatchConfirmed.
	SetStatus(ctx context.Context, id uuid.UUID, status domain.MatchStatus, confirmedAt *time.Time) error
	// ReplaceResult swaps the result of a contested match and hands it back
	// for confirmation: mode, sets and reporter come from corrected, and the
	// status returns to pending.
	//
	// It refuses anything that is not currently disputed, with
	// domain.ErrConflict — a settled result must not be quietly rewritten,
	// and that guard belongs where the write happens rather than only in the
	// caller that remembered to check.
	ReplaceResult(ctx context.Context, id uuid.UUID, corrected domain.Match) error
}

// TTRHistoryRepository writes and reads the rating history.
type TTRHistoryRepository interface {
	// Append writes the entries of one rating event. Callers use InTx so the
	// history and the player rating cannot drift apart.
	Append(ctx context.Context, changes []domain.TTRChange) error
	// ForPlayer returns a player's entries, **newest first**, at most limit
	// of them. The order is part of the contract: callers take the first
	// entry to mean the most recent rating change, and a store that returns
	// them the other way round answers a different question.
	ForPlayer(ctx context.Context, playerID uuid.UUID, limit int) ([]domain.TTRChange, error)
	// ForMatch returns the entries one match produced — one per player.
	// Taking a match back means writing ttr_before straight back, and that
	// is where the number lives.
	ForMatch(ctx context.Context, matchID uuid.UUID) ([]domain.TTRChange, error)
	// ForMatches is ForMatch for many, in one query. A list of matches wants
	// what each one was worth, and asking per match turns one page into one
	// round trip per row.
	ForMatches(ctx context.Context, matchIDs []uuid.UUID) ([]domain.TTRChange, error)
}
