package postgres_test

import (
	"testing"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// Issue #71: a tournament evening and a normal Tuesday were the same row
// shape, so the Definition of Done counted a scorekeeper's typing as people
// logging their own results.
func TestMatchesRecordHowTheyWereEntered(t *testing.T) {
	store, ctx := newStore(t)

	anna := mustPlayer(ctx, t, store, "Anna", domain.DefaultTTR)
	bodo := mustPlayer(ctx, t, store, "Bodo", domain.DefaultTTR)

	sets := []domain.MatchSet{{SetNo: 1, HomePoints: 11, AwayPoints: 9}, {SetNo: 2, HomePoints: 11, AwayPoints: 7}}

	kiosk, err := store.Matches().Create(ctx, domain.Match{
		HomeID: anna.ID, AwayID: bodo.ID, BestOf: 3, PointsToWin: 11,
		ReportedBy: anna.ID, EnteredVia: domain.EnteredViaKiosk, Sets: sets,
	})
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	if kiosk.EnteredVia != domain.EnteredViaKiosk {
		t.Errorf("Create() returned %q, want %q", kiosk.EnteredVia, domain.EnteredViaKiosk)
	}

	// It has to survive the round trip, not only the returning clause.
	back, err := store.Matches().ByID(ctx, kiosk.ID)
	if err != nil {
		t.Fatalf("ByID(): %v", err)
	}
	if back.EnteredVia != domain.EnteredViaKiosk {
		t.Errorf("ByID() = %q, want %q", back.EnteredVia, domain.EnteredViaKiosk)
	}

	// An unset value is a player's own entry, matching the column default. A
	// caller that forgets gets the truthful answer for the path almost every
	// row takes, not a constraint violation.
	own, err := store.Matches().Create(ctx, domain.Match{
		HomeID: bodo.ID, AwayID: anna.ID, BestOf: 3, PointsToWin: 11,
		ReportedBy: bodo.ID, Sets: sets,
	})
	if err != nil {
		t.Fatalf("Create() without EnteredVia: %v", err)
	}
	if own.EnteredVia != domain.EnteredViaPlayer {
		t.Errorf("an unset EnteredVia stored %q, want %q", own.EnteredVia, domain.EnteredViaPlayer)
	}

	// And the list readers carry it too, or the measurement query is the only
	// thing that can see it.
	recent, err := store.Matches().Recent(ctx, 10)
	if err != nil {
		t.Fatalf("Recent(): %v", err)
	}
	seen := map[domain.EnteredVia]int{}
	for _, m := range recent {
		seen[m.EnteredVia]++
	}
	if seen[domain.EnteredViaKiosk] != 1 || seen[domain.EnteredViaPlayer] != 1 {
		t.Errorf("Recent() reports %v, want one of each", seen)
	}
}
