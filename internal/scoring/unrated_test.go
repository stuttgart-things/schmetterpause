package scoring_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/match"
	"github.com/stuttgart-things/schmetterpause/internal/repository/postgres"
	"github.com/stuttgart-things/schmetterpause/internal/scoring"
)

// A tournament that does not count moves nothing (docs/adr/0012).
//
// Against the real database rather than a fake, because the whole mechanism
// is that settle skips two writes: the ratings and the history. A fake that
// answers "no history" without ever having been asked to write one proves
// nothing about either.
func TestATournamentWithoutRatingMovesNoRating(t *testing.T) {
	store, ctx := newStore(t)

	anna, bodo, tour := unratedTournament(ctx, t, store)
	before := anna.TTR

	settlement, err := scoring.Record(ctx, store, anna.ID, bodo.ID,
		result(t), domain.EnteredViaKiosk, &tour.ID, nil, time.Now())
	if err != nil {
		t.Fatalf("Record(): %v", err)
	}

	if settlement.Rated {
		t.Error("the settlement claims the match was rated")
	}
	// The match happened: it is confirmed, it belongs in the table and in
	// the statistics.
	if settlement.Match.Status != domain.MatchConfirmed {
		t.Errorf("the match is %s, want confirmed", settlement.Match.Status)
	}

	for name, id := range map[string]domain.Player{"Anna": anna, "Bodo": bodo} {
		after, err := store.Players().ByID(ctx, id.ID)
		if err != nil {
			t.Fatalf("reload %s: %v", name, err)
		}
		if after.TTR != before {
			t.Errorf("%s moved from %d to %d in a tournament that does not count",
				name, before, after.TTR)
		}
	}

	history, err := store.TTRHistory().ForMatch(ctx, settlement.Match.ID)
	if err != nil {
		t.Fatalf("ForMatch(): %v", err)
	}
	if len(history) != 0 {
		t.Errorf("%d history rows written for an unrated match", len(history))
	}
}

// The same evening in a tournament that counts, so the test above cannot pass
// because something else broke.
func TestATournamentWithRatingStillRates(t *testing.T) {
	store, ctx := newStore(t)

	anna, bodo, tour := ratedTournament(ctx, t, store)

	settlement, err := scoring.Record(ctx, store, anna.ID, bodo.ID,
		result(t), domain.EnteredViaKiosk, &tour.ID, nil, time.Now())
	if err != nil {
		t.Fatalf("Record(): %v", err)
	}

	if !settlement.Rated {
		t.Fatal("a tournament that counts did not rate")
	}
	if settlement.HomeChange.Delta() <= 0 {
		t.Errorf("the winner gained %d", settlement.HomeChange.Delta())
	}
	_ = bodo
}

// Taking back an unrated result used to be impossible: no history meant the
// match had been confirmed by something that never settled it, which is
// exactly what an unrated match now looks like.
func TestAnUnratedMatchCanBeTakenBack(t *testing.T) {
	store, ctx := newStore(t)

	anna, bodo, tour := unratedTournament(ctx, t, store)

	settlement, err := scoring.Record(ctx, store, anna.ID, bodo.ID,
		result(t), domain.EnteredViaKiosk, &tour.ID, nil, time.Now())
	if err != nil {
		t.Fatalf("Record(): %v", err)
	}

	if _, err := scoring.Undo(ctx, store, settlement.Match.ID, time.Now()); err != nil {
		t.Fatalf("Undo(): %v", err)
	}
	if _, err := store.Matches().ByID(ctx, settlement.Match.ID); err == nil {
		t.Error("the match is still there after being taken back")
	}
	_ = bodo
}

func result(t *testing.T) match.Result {
	t.Helper()

	return match.Result{
		Mode: match.Mode{BestOf: 3, PointsToWin: 11},
		Sets: []match.Set{{Home: 11, Away: 9}, {Home: 11, Away: 7}},
	}
}

func unratedTournament(ctx context.Context, t *testing.T, store *postgres.Store) (domain.Player, domain.Player, domain.Tournament) {
	t.Helper()

	return seedTournamentRated(ctx, t, store, false)
}

func ratedTournament(ctx context.Context, t *testing.T, store *postgres.Store) (domain.Player, domain.Player, domain.Tournament) {
	t.Helper()

	return seedTournamentRated(ctx, t, store, true)
}

func seedTournamentRated(ctx context.Context, t *testing.T, store *postgres.Store, rated bool) (domain.Player, domain.Player, domain.Tournament) {
	t.Helper()

	anna, err := store.Players().Create(ctx, "Anna", domain.DefaultTTR)
	if err != nil {
		t.Fatalf("create Anna: %v", err)
	}
	bodo, err := store.Players().Create(ctx, "Bodo", domain.DefaultTTR)
	if err != nil {
		t.Fatalf("create Bodo: %v", err)
	}

	tour, err := store.Tournaments().Create(ctx, domain.Tournament{
		Name: "Freitag", Format: domain.TournamentRoundRobin,
		Status: domain.TournamentOpen, CreatedBy: anna.ID,
		BestOf: 3, PointsToWin: 11, Rated: rated,
		Players: []uuid.UUID{anna.ID, bodo.ID},
	})
	if err != nil {
		t.Fatalf("create the tournament: %v", err)
	}
	return anna, bodo, tour
}
