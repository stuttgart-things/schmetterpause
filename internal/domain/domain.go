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
}

// Identity links a provider's proof to a player. A player can hold several
// identities.
type Identity struct {
	Provider  Provider
	Subject   string
	PlayerID  uuid.UUID
	CreatedAt time.Time
}

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
	Sets        []MatchSet
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
