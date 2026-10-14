package checkin_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/checkin"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

var now = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

func pair() (home, away domain.Checkin) {
	home = domain.Checkin{ID: uuid.New(), PlayerID: uuid.New(), Half: domain.HalfLeft,
		Method: domain.CheckinRFID, CreatedAt: now.Add(-10 * time.Minute)}
	away = domain.Checkin{ID: uuid.New(), PlayerID: uuid.New(), Half: domain.HalfRight,
		Method: domain.CheckinPhone, CreatedAt: now.Add(-9 * time.Minute)}
	return home, away
}

func TestVerifyAcceptsTwoFreshCheckinsAtDifferentHalves(t *testing.T) {
	home, away := pair()
	if err := checkin.Verify(home, away, home.PlayerID, away.PlayerID, now); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

// The halves only have to differ. The Zählwerk decides which player is home,
// and home on the right is as good as home on the left.
func TestVerifyDoesNotCareWhichHalfHomeStoodAt(t *testing.T) {
	home, away := pair()
	home.Half, away.Half = domain.HalfRight, domain.HalfLeft
	if err := checkin.Verify(home, away, home.PlayerID, away.PlayerID, now); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

// Exactly Validity old still counts; the rule is "older than".
func TestVerifyAcceptsACheckinExactlyValidityOld(t *testing.T) {
	home, away := pair()
	home.CreatedAt = now.Add(-checkin.Validity)
	if err := checkin.Verify(home, away, home.PlayerID, away.PlayerID, now); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func TestVerifyRejects(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(home, away *domain.Checkin, homeID, awayID *uuid.UUID)
		kind   checkin.Kind
		// offender picks which check-in the rejection has to name.
		offender func(home, away domain.Checkin) uuid.UUID
	}{
		{
			name:     "home check-in of somebody else",
			change:   func(_, _ *domain.Checkin, homeID, _ *uuid.UUID) { *homeID = uuid.New() },
			kind:     checkin.WrongPlayer,
			offender: func(h, _ domain.Checkin) uuid.UUID { return h.ID },
		},
		{
			name:     "away check-in of somebody else",
			change:   func(_, _ *domain.Checkin, _, awayID *uuid.UUID) { *awayID = uuid.New() },
			kind:     checkin.WrongPlayer,
			offender: func(_, a domain.Checkin) uuid.UUID { return a.ID },
		},
		{
			// The sender swapped the two ids. Each check-in is still real,
			// but it is the other side's.
			name: "check-ins swapped",
			change: func(_, _ *domain.Checkin, homeID, awayID *uuid.UUID) {
				*homeID, *awayID = *awayID, *homeID
			},
			kind:     checkin.WrongPlayer,
			offender: func(h, _ domain.Checkin) uuid.UUID { return h.ID },
		},
		{
			name:     "same half",
			change:   func(_, away *domain.Checkin, _, _ *uuid.UUID) { away.Half = domain.HalfLeft },
			kind:     checkin.SameHalf,
			offender: func(_, a domain.Checkin) uuid.UUID { return a.ID },
		},
		{
			name:     "home already used",
			change:   func(home, _ *domain.Checkin, _, _ *uuid.UUID) { home.Used = true },
			kind:     checkin.Used,
			offender: func(h, _ domain.Checkin) uuid.UUID { return h.ID },
		},
		{
			name:     "away already used",
			change:   func(_, away *domain.Checkin, _, _ *uuid.UUID) { away.Used = true },
			kind:     checkin.Used,
			offender: func(_, a domain.Checkin) uuid.UUID { return a.ID },
		},
		{
			name: "away expired",
			change: func(_, away *domain.Checkin, _, _ *uuid.UUID) {
				away.CreatedAt = now.Add(-checkin.Validity - time.Second)
			},
			kind:     checkin.Expired,
			offender: func(_, a domain.Checkin) uuid.UUID { return a.ID },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home, away := pair()
			homeID, awayID := home.PlayerID, away.PlayerID
			tc.change(&home, &away, &homeID, &awayID)

			err := checkin.Verify(home, away, homeID, awayID, now)
			var rej *checkin.Rejection
			if !errors.As(err, &rej) {
				t.Fatalf("Verify = %v, want a *Rejection", err)
			}
			if rej.Kind != tc.kind {
				t.Errorf("Kind = %d, want %d (%v)", rej.Kind, tc.kind, err)
			}
			if want := tc.offender(home, away); rej.CheckinID != want {
				t.Errorf("CheckinID = %s, want %s", rej.CheckinID, want)
			}
		})
	}
}
