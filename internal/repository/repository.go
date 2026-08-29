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
	Credentials() CredentialRepository
	KioskGrants() KioskGrantRepository
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
	// ByDisplayName resolves a name to a player, matched the way the unique
	// index is: trimmed and case-folded. Its consumer is the bootstrap in
	// docs/adr/0008, which names an admin by the name people call them.
	ByDisplayName(ctx context.Context, name string) (domain.Player, error)
	// Admins returns everybody who may act for other people, by name.
	Admins(ctx context.Context) ([]domain.Player, error)
	// SetAdmin grants or withdraws the flag. A flag rather than roles, and a
	// property of the person rather than of a browser — which is what makes
	// it revocable and lets a log line name somebody (docs/adr/0008).
	SetAdmin(ctx context.Context, id uuid.UUID, isAdmin bool) error
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
	// Unlink forgets one proof. Its consumer is signing out: the browser
	// stops being recognised, and the row that recognised it stops existing
	// rather than sitting in the table as a credential nobody holds.
	//
	// A proof that is not there is not an error. Signing out twice, or after
	// a key rotation left the cookie unreadable, is an ordinary thing to do.
	Unlink(ctx context.Context, provider domain.Provider, subject string) error
}

// CredentialRepository stores the shared secrets a player proves themselves
// with. It never sees a secret in the clear — hashing happens in the
// credential package, and only the encoding it produces reaches here.
type CredentialRepository interface {
	// Put writes the hash for one kind, replacing whatever stood there. That
	// replacement is the point: issuing a new recovery code has to invalidate
	// the previous one in the same step, or the old one stays valid because
	// somebody forgot the delete (docs/adr/0006).
	Put(ctx context.Context, playerID uuid.UUID, kind domain.CredentialKind, hash string) error
	// ForPlayer returns one player's credential of one kind. A player who has
	// none of that kind is domain.ErrNotFound — the ordinary state for a PIN,
	// since setting one is optional (docs/adr/0007).
	ForPlayer(ctx context.Context, playerID uuid.UUID, kind domain.CredentialKind) (domain.Credential, error)
}

// KioskGrantRepository manages the machines unlocked as a kiosk.
//
// It never sees the secret in a cookie, only the SHA-256 of it. A plain hash
// is right here and wrong for a PIN: the secret is 32 bytes the server
// generated, so there is no guess to slow down, and a memory-hard hash on
// every kiosk request would make the page at the table unusable.
type KioskGrantRepository interface {
	// Create records a freshly unlocked machine.
	Create(ctx context.Context, secretHash []byte, expiresAt time.Time, userAgent string) (domain.KioskGrant, error)
	// BySecret finds the grant a cookie stands for. An unknown secret is
	// domain.ErrNotFound, which is the ordinary answer for a cookie somebody
	// kept past a revocation.
	BySecret(ctx context.Context, secretHash []byte) (domain.KioskGrant, error)
	// Touch records that this machine was just seen. Called sparingly rather
	// than on every request: it is a write, and the answer it produces only
	// has to be good to the minute.
	Touch(ctx context.Context, id uuid.UUID, at time.Time) error
	// Revoke takes one machine back. Revoking an already revoked grant is not
	// an error — two people pressing the same button is not a failure.
	Revoke(ctx context.Context, id uuid.UUID, at time.Time) error
	// RevokeAll takes every unlocked machine back at once and reports how
	// many there were. This is the answer to "the token has been read over
	// somebody's shoulder" that does not involve a restart.
	RevokeAll(ctx context.Context, at time.Time) (int, error)
	// Active lists the machines that are kiosks at t, newest first.
	Active(ctx context.Context, at time.Time) ([]domain.KioskGrant, error)
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
