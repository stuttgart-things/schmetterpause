// Package checkin decides whether two check-ins prove a match
// (docs/adr/0024).
//
// Pure domain logic, as the "Konventionen" section of CLAUDE.md asks: the
// caller reads the rows, under a lock, and this package says whether they
// carry the result. Like internal/match it returns a Kind rather than a
// sentence, because the sentence belongs to whoever answers the request.
package checkin

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// Validity is how long a check-in can carry a result.
//
// ADR-0024 asks for "a few hours" and a constant in the code. Three hours is a
// long lunch with a tournament in it. Much longer and a check-in from the
// morning still proves who stands at the table in the evening.
const Validity = 3 * time.Hour

// Kind is why two check-ins do not prove a match.
type Kind int

const (
	// WrongPlayer is a check-in that belongs to neither side, or both
	// check-ins belong to the same side.
	WrongPlayer Kind = iota + 1
	// SameHalf is both players checked in at the same end of the table.
	SameHalf
	// Used is a check-in that already carries a result.
	Used
	// Expired is a check-in older than Validity.
	Expired
)

// Rejection says which rule two check-ins broke, and which check-in broke it.
type Rejection struct {
	Kind Kind
	// CheckinID is the offending check-in. For SameHalf it is the away one,
	// which is as good as either.
	CheckinID uuid.UUID
}

func (r *Rejection) Error() string {
	switch r.Kind {
	case WrongPlayer:
		return fmt.Sprintf("check-in %s belongs to the wrong player", r.CheckinID)
	case SameHalf:
		return "both check-ins are at the same half"
	case Used:
		return fmt.Sprintf("check-in %s already carries a result", r.CheckinID)
	case Expired:
		return fmt.Sprintf("check-in %s has expired", r.CheckinID)
	}
	return "the check-ins do not prove the match"
}

// Verify reports whether home and away prove a match of homeID against
// awayID at now. It returns nil or a *Rejection.
//
// Each check-in has to belong to its own side, the two have to be at
// different halves, neither may carry a result yet, and neither may be older
// than Validity. Which half home stood at does not matter: the Zählwerk names
// home and away, the halves only have to differ.
func Verify(home, away domain.Checkin, homeID, awayID uuid.UUID, now time.Time) error {
	if home.PlayerID != homeID {
		return &Rejection{Kind: WrongPlayer, CheckinID: home.ID}
	}
	if away.PlayerID != awayID {
		return &Rejection{Kind: WrongPlayer, CheckinID: away.ID}
	}
	if home.Half == away.Half {
		return &Rejection{Kind: SameHalf, CheckinID: away.ID}
	}
	for _, c := range []domain.Checkin{home, away} {
		if c.Used {
			return &Rejection{Kind: Used, CheckinID: c.ID}
		}
	}
	for _, c := range []domain.Checkin{home, away} {
		if now.Sub(c.CreatedAt) > Validity {
			return &Rejection{Kind: Expired, CheckinID: c.ID}
		}
	}
	return nil
}
