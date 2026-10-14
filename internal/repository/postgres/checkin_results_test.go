package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/repository"
	"github.com/stuttgart-things/schmetterpause/internal/repository/postgres"
)

func mustCheckin(ctx context.Context, t *testing.T, store *postgres.Store, player uuid.UUID, half domain.Half) domain.Checkin {
	t.Helper()

	c, err := store.Checkins().Create(ctx, domain.Checkin{
		PlayerID: player, Half: half, Method: domain.CheckinRFID,
		CreatedAt: time.Now().Truncate(time.Microsecond),
	})
	if err != nil {
		t.Fatalf("Checkins().Create(): %v", err)
	}
	return c
}

var twoSets = []domain.MatchSet{{SetNo: 1, HomePoints: 11, AwayPoints: 9}, {SetNo: 2, HomePoints: 11, AwayPoints: 7}}

func checkedInMatch(home, away uuid.UUID, hc, ac *uuid.UUID) domain.Match {
	return domain.Match{
		HomeID: home, AwayID: away, BestOf: 3, PointsToWin: 11,
		EnteredVia: domain.EnteredViaCheckin, HomeCheckinID: hc, AwayCheckinID: ac,
		Sets: twoSets,
	}
}

// docs/adr/0024: a checked-in match has no reporter and cites its two
// check-ins, and the schema holds every other path to a reporter.
func TestACheckedInMatchHasNoReporterAndCitesItsCheckins(t *testing.T) {
	store, ctx := newStore(t)
	anna := mustPlayer(ctx, t, store, "Anna", domain.DefaultTTR)
	bodo := mustPlayer(ctx, t, store, "Bodo", domain.DefaultTTR)
	hc := mustCheckin(ctx, t, store, anna.ID, domain.HalfLeft)
	ac := mustCheckin(ctx, t, store, bodo.ID, domain.HalfRight)

	created, err := store.Matches().Create(ctx, checkedInMatch(anna.ID, bodo.ID, &hc.ID, &ac.ID))
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	back, err := store.Matches().ByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("ByID(): %v", err)
	}
	if back.ReportedBy != uuid.Nil || back.EnteredVia != domain.EnteredViaCheckin {
		t.Errorf("stored reporter %s via %q, want nobody via checkin", back.ReportedBy, back.EnteredVia)
	}
	if back.HomeCheckinID == nil || *back.HomeCheckinID != hc.ID ||
		back.AwayCheckinID == nil || *back.AwayCheckinID != ac.ID {
		t.Errorf("stored check-ins %v / %v, want %s / %s", back.HomeCheckinID, back.AwayCheckinID, hc.ID, ac.ID)
	}

	// Both players have it waiting on them. "reported_by <> $1" would have
	// been null for both, and the match would have reached nobody.
	for _, p := range []domain.Player{anna, bodo} {
		pending, err := store.Matches().PendingFor(ctx, p.ID)
		if err != nil {
			t.Fatalf("PendingFor(%s): %v", p.DisplayName, err)
		}
		if len(pending) != 1 || pending[0].ID != created.ID {
			t.Errorf("PendingFor(%s) = %d matches, want the checked-in one", p.DisplayName, len(pending))
		}
		n, err := store.Matches().PendingCountFor(ctx, p.ID)
		if err != nil || n != 1 {
			t.Errorf("PendingCountFor(%s) = %d, %v; want 1", p.DisplayName, n, err)
		}
		waiting, err := store.Matches().WaitingOnOpponentFor(ctx, p.ID)
		if err != nil || len(waiting) != 0 {
			t.Errorf("WaitingOnOpponentFor(%s) = %d, %v; want none, nobody reported it", p.DisplayName, len(waiting), err)
		}
	}

	// The schema keeps the other paths honest.
	cases := []struct {
		name string
		m    domain.Match
	}{
		{"a player's entry without a reporter", domain.Match{
			HomeID: anna.ID, AwayID: bodo.ID, BestOf: 3, PointsToWin: 11, Sets: twoSets}},
		{"a checked-in entry without check-ins", checkedInMatch(anna.ID, bodo.ID, nil, nil)},
		{"one check-in on a player's entry", domain.Match{
			HomeID: anna.ID, AwayID: bodo.ID, BestOf: 3, PointsToWin: 11, ReportedBy: anna.ID,
			HomeCheckinID: &hc.ID, Sets: twoSets}},
		{"check-ins on a scoreboard entry", domain.Match{
			HomeID: anna.ID, AwayID: bodo.ID, BestOf: 3, PointsToWin: 11, ReportedBy: anna.ID,
			EnteredVia: domain.EnteredViaScoreboard, HomeCheckinID: &hc.ID, AwayCheckinID: &ac.ID,
			Sets: twoSets}},
	}
	for _, tc := range cases {
		if _, err := store.Matches().Create(ctx, tc.m); err == nil {
			t.Errorf("%s: Create() succeeded, want the schema to refuse it", tc.name)
		}
	}
}

func TestLockReportsWhichCheckinsAreUsed(t *testing.T) {
	store, ctx := newStore(t)
	anna := mustPlayer(ctx, t, store, "Anna", domain.DefaultTTR)
	bodo := mustPlayer(ctx, t, store, "Bodo", domain.DefaultTTR)
	hc := mustCheckin(ctx, t, store, anna.ID, domain.HalfLeft)
	ac := mustCheckin(ctx, t, store, bodo.ID, domain.HalfRight)
	fresh := mustCheckin(ctx, t, store, anna.ID, domain.HalfRight)

	if _, err := store.Matches().Create(ctx, checkedInMatch(anna.ID, bodo.ID, &hc.ID, &ac.ID)); err != nil {
		t.Fatalf("Create(): %v", err)
	}

	missing := uuid.New()
	got, err := store.Checkins().Lock(ctx, []uuid.UUID{hc.ID, ac.ID, fresh.ID, missing})
	if err != nil {
		t.Fatalf("Lock(): %v", err)
	}
	if len(got) != 3 {
		t.Errorf("Lock() returned %d check-ins, want 3 — the unknown id is simply absent", len(got))
	}
	if !got[hc.ID].Used || !got[ac.ID].Used || got[fresh.ID].Used {
		t.Errorf("Used = %v / %v / %v, want true / true / false", got[hc.ID].Used, got[ac.ID].Used, got[fresh.ID].Used)
	}
	if c := got[fresh.ID]; c.PlayerID != anna.ID || c.Half != domain.HalfRight || c.Method != domain.CheckinRFID ||
		!c.CreatedAt.Equal(fresh.CreatedAt) {
		t.Errorf("Lock() read %+v, want %+v", c, fresh)
	}

	// The index behind the lock: the same check-in in the same role again.
	other := mustCheckin(ctx, t, store, bodo.ID, domain.HalfRight)
	if _, err := store.Matches().Create(ctx, checkedInMatch(anna.ID, bodo.ID, &hc.ID, &other.ID)); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("Create() reusing a check-in = %v, want domain.ErrConflict", err)
	}
}

// The reason Lock exists: two transactions that both read the check-ins
// before either writes would both find them unused. With the rows locked the
// second waits, then sees the first one's match.
func TestTwoResultsCannotShareACheckin(t *testing.T) {
	store, ctx := newStore(t)
	anna := mustPlayer(ctx, t, store, "Anna", domain.DefaultTTR)
	bodo := mustPlayer(ctx, t, store, "Bodo", domain.DefaultTTR)
	hc := mustCheckin(ctx, t, store, anna.ID, domain.HalfLeft)
	ac := mustCheckin(ctx, t, store, bodo.ID, domain.HalfRight)

	errUsed := errors.New("used")
	var (
		wg      sync.WaitGroup
		start   = make(chan struct{})
		results = make([]error, 4)
	)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[i] = store.InTx(ctx, func(tx repository.Store) error {
				// Opposite orders on purpose: Lock sorts, so they still
				// queue instead of deadlocking.
				ids := []uuid.UUID{hc.ID, ac.ID}
				if i%2 == 1 {
					ids = []uuid.UUID{ac.ID, hc.ID}
				}
				got, err := tx.Checkins().Lock(ctx, ids)
				if err != nil {
					return err
				}
				if got[hc.ID].Used || got[ac.ID].Used {
					return errUsed
				}
				// Long enough for every other transaction to have read
				// the check-ins too, if nothing made it wait. Without the
				// lock they all find them unused and the index refuses
				// three of them, which is the failure this test is for.
				time.Sleep(200 * time.Millisecond)
				_, err = tx.Matches().Create(ctx, checkedInMatch(anna.ID, bodo.ID, &hc.ID, &ac.ID))
				return err
			})
		}()
	}
	close(start)
	wg.Wait()

	var ok int
	for i, err := range results {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, errUsed):
		default:
			t.Errorf("transaction %d: %v, want success or found used", i, err)
		}
	}
	if ok != 1 {
		t.Errorf("%d transactions stored a match, want exactly 1", ok)
	}
}
