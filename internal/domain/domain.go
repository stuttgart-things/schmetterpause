// Package domain holds the application's business types.
//
// The package deliberately depends on neither the database nor HTTP, so that
// domain logic (rating calculation, tables, brackets) stays testable without
// infrastructure — see the "Konventionen" section in CLAUDE.md.
package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound reports that a requested record does not exist. Repository
// implementations wrap it; callers check with errors.Is.
var ErrNotFound = errors.New("not found")

// ErrConflict reports that a record collides with one that already exists —
// a display name someone else has taken, for instance. Repositories wrap it
// so handlers can answer with a readable message instead of a 500, without
// learning anything about SQL error codes.
var ErrConflict = errors.New("already exists")

// DefaultTTR is the starting rating for newly created players.
//
// Whether 1000 spreads well across an office group is an open question in the
// MVP plan and will only be answered with real data.
const DefaultTTR = 1000

// Provider names the method by which an identity was proven. New methods are
// a row, not a schema change (docs/adr/0003).
type Provider string

const (
	// ProviderLocal is recognition through a signed cookie without real
	// authentication. The only provider in the MVP.
	ProviderLocal Provider = "local"
	// ProviderGitLab is OIDC against the company GitLab. Not implemented yet.
	ProviderGitLab Provider = "gitlab"
	// ProviderGitHub is OIDC against GitHub. Not implemented yet.
	ProviderGitHub Provider = "github"
	// ProviderPasskey is WebAuthn. Stores only a public key and a signature
	// counter, never biometric data (docs/adr/0004).
	ProviderPasskey Provider = "passkey"
)

// Player is a player. The struct carries business data only; how a player was
// authenticated is recorded in Identity.
type Player struct {
	ID          uuid.UUID
	DisplayName string
	TTR         int
	CreatedAt   time.Time
	// IsAdmin marks somebody who may act for other people: correcting a
	// counted result, merging two players, closing a tournament
	// (docs/adr/0008). One level, not roles.
	//
	// A property of the person rather than of a browser, which is what makes
	// it revocable and what makes a log line name somebody.
	IsAdmin bool
}

// PlayerRecord is a player with their confirmed match tally. Only confirmed
// matches count — a pending or disputed one changes nothing until somebody
// agrees it happened.
type PlayerRecord struct {
	Player Player
	// Played, Won and Lost cover confirmed matches only.
	Played, Won, Lost int
}

// Identity links a provider's proof to a player. A player can hold several
// identities.
type Identity struct {
	Provider  Provider
	Subject   string
	PlayerID  uuid.UUID
	CreatedAt time.Time
}

// CredentialKind names a sort of shared secret a player can prove themselves
// with. Both kinds are bearer credentials: whoever holds one is the player,
// and the blast radius stays the one docs/adr/0004 accepts.
type CredentialKind string

const (
	// CredentialRecovery is the generated recovery code. Everybody gets one
	// at join without doing anything, and it is the only kind a third party
	// may issue — that is what the kiosk does (docs/adr/0006).
	CredentialRecovery CredentialKind = "recovery"
	// CredentialPIN is the digits a player chose. Optional, memorable, and
	// nobody else can set it (docs/adr/0007).
	CredentialPIN CredentialKind = "pin"
)

// Credential is one player's secret of one kind, stored as a hash and never
// in the clear. A player holds at most one per kind: a new secret replaces
// the old one, which is what makes a new recovery code invalidate the
// previous one immediately.
type Credential struct {
	PlayerID uuid.UUID
	Kind     CredentialKind
	// Hash is the encoded Argon2id digest, parameters and salt included.
	Hash      string
	UpdatedAt time.Time
}

// KioskGrant is one machine that has been unlocked as a kiosk.
//
// A row per machine rather than a value every unlocked browser shares
// (issue #77). That is what makes two questions answerable that a derived
// constant cannot answer: which machines are kiosks right now, and how does
// somebody take one of them back without restarting the application and
// logging out the table.
type KioskGrant struct {
	ID        uuid.UUID
	CreatedAt time.Time
	// LastSeenAt is when this machine last showed its cookie. It is what
	// makes a list of grants readable: one nobody has used since Tuesday is
	// a laptop somebody took home.
	LastSeenAt time.Time
	ExpiresAt  time.Time
	// UserAgent is what the browser said it was. A label, not a fact and not
	// identity — it exists so the list reads as machines rather than as a
	// column of identifiers.
	UserAgent string
	// RevokedAt is set when somebody took this grant back.
	RevokedAt *time.Time
}

// Active reports whether this grant still unlocks anything at t.
func (g KioskGrant) Active(t time.Time) bool {
	return g.RevokedAt == nil && t.Before(g.ExpiresAt)
}

// TournamentStatus is where a tournament is in its short life.
type TournamentStatus string

const (
	// TournamentOpen is being played. Results can still arrive.
	TournamentOpen TournamentStatus = "open"
	// TournamentClosed is over. Nothing about the rating depends on this —
	// tournament matches settle one at a time (docs/adr/0009) — so closing
	// is bookkeeping: it takes the tournament off the list of things still
	// happening.
	TournamentClosed TournamentStatus = "closed"
)

// TournamentFormat names how the draw is built.
type TournamentFormat string

const (
	// TournamentRoundRobin is everybody against everybody, once. Swiss is
	// the named successor for a field past ten (#41), and a type that cannot
	// say which format it holds cannot hold two.
	TournamentRoundRobin TournamentFormat = "round_robin"
	// TournamentDoubleRoundRobin is the same draw twice, with the sides
	// swapped in the second half (docs/adr/0011). Twice the matches: eight
	// players is 56, which is a number worth seeing before agreeing to it.
	TournamentDoubleRoundRobin TournamentFormat = "double_round_robin"
)

// Legs is how many times each pair meets in this format.
func (f TournamentFormat) Legs() int {
	if f == TournamentDoubleRoundRobin {
		return 2
	}
	return 1
}

// Tournament is a bracket around matches.
//
// It is deliberately not a scoring event: a tournament match settles through
// the ordinary confirmation path, one at a time (docs/adr/0009). What a
// tournament owns is who is in it and which matches belong to it.
type Tournament struct {
	ID     uuid.UUID
	Name   string
	Format TournamentFormat
	Status TournamentStatus
	// CreatedBy is whoever set it up. Not a permission — anybody may start
	// one — but a tournament with no author is one nobody will admit to
	// having made when the pairings are wrong.
	CreatedBy uuid.UUID
	CreatedAt time.Time
	// ClosedAt is set exactly when Status is TournamentClosed.
	ClosedAt *time.Time
	// BestOf and PointsToWin are the mode every match in the draw is played
	// under. They sit on the tournament rather than on each pairing because
	// a quick tournament is one agreement about how the evening is played,
	// and a control per pairing would ask it twenty-eight times.
	BestOf      int
	PointsToWin int
	// WithFinal is whether the two best of the group play a decider
	// afterwards. A separate flag rather than a fourth format name: four
	// names for four combinations grows quadratically at the next variant
	// (docs/adr/0011).
	WithFinal bool
	// Players are the participants in draw order. The order is the draw:
	// the circle method is deterministic over it, so the pairings are a
	// function of this slice rather than a stored copy that could drift.
	Players []uuid.UUID
}

// Open reports whether results can still arrive.
func (t Tournament) Open() bool { return t.Status == TournamentOpen }

// MatchStatus is a match's confirmation state. Only a match in state
// MatchConfirmed enters the rating.
type MatchStatus string

const (
	// MatchPending has been recorded but not yet confirmed by the opponent.
	MatchPending MatchStatus = "pending"
	// MatchConfirmed has been confirmed by the opponent and counts.
	MatchConfirmed MatchStatus = "confirmed"
	// MatchDisputed has been contested by the opponent and blocks rating.
	// Resolvable only by hand in the MVP.
	MatchDisputed MatchStatus = "disputed"
)

// EnteredVia records how a result reached the database.
//
// It answers a question the measurement has to be able to ask and could not
// (issue #71): whether a row is somebody logging their own match, which is
// what the Definition of Done counts, or a scorekeeper typing in an evening,
// which is not. Eight players round robin is twenty-eight matches — counted in
// by accident, the measurement passes and proves nothing.
type EnteredVia string

const (
	// EnteredViaPlayer is a player entering a result themselves. The default,
	// and the only kind that answers the question the MVP asks.
	EnteredViaPlayer EnteredVia = "player"
	// EnteredViaKiosk is the machine at the table, where one person enters
	// for everybody.
	EnteredViaKiosk EnteredVia = "kiosk"
)

// Match is a singles encounter between two players. Doubles do not count
// towards TTR and would need a rating of their own if they ever arrive.
type Match struct {
	ID          uuid.UUID
	HomeID      uuid.UUID
	AwayID      uuid.UUID
	BestOf      int
	PointsToWin int
	Status      MatchStatus
	ReportedBy  uuid.UUID
	PlayedAt    time.Time
	// ConfirmedAt is set exactly when Status is MatchConfirmed.
	ConfirmedAt *time.Time
	// EnteredVia is how the result reached the database. Empty means
	// EnteredViaPlayer, which is what the column defaults to.
	EnteredVia EnteredVia
	// TournamentID is the bracket this match belongs to, or nil for one
	// played outside a tournament — which is every match that existed
	// before docs/adr/0009. It changes nothing about how the match is
	// rated; it is what lets a table be built from the results.
	TournamentID *uuid.UUID
	// TournamentRound is which slot of the draw this match fills, counting
	// from 1. Nil outside a tournament, and nil for every row written before
	// docs/adr/0011 — which is unambiguous rather than a gap: those are all
	// single round robins, where a pair occurs exactly once and (round, pair)
	// and (pair) are the same key.
	//
	// It is not a stored copy of a derived value. The slots are computed from
	// the draw; this says which of them a result belongs to, which is the
	// thing that cannot be derived once a pair can meet more than once.
	TournamentRound *int
	Sets            []MatchSet
}

// MatchSet is a single set within a match.
type MatchSet struct {
	SetNo      int
	HomePoints int
	AwayPoints int
}

// TTRChange is one entry in the rating history: the change to a player's
// rating caused by exactly one rated match.
type TTRChange struct {
	ID        uuid.UUID
	PlayerID  uuid.UUID
	MatchID   uuid.UUID
	TTRBefore int
	TTRAfter  int
	CreatedAt time.Time
}

// Delta is the rating change recorded by this entry.
func (c TTRChange) Delta() int { return c.TTRAfter - c.TTRBefore }
