package postgres_test

import (
	"errors"
	"testing"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// Removing a player is the one action in issue #105 the schema decides rather
// than the application: matches and tournaments reference a player with no
// cascade, and everything that only exists because they do carries one. These
// tests are against the real database on purpose — the rule lives in
// db/migrations, so a fake agreeing with the handler would prove nothing.
func TestDeletingAPlayerWhoNeverPlayed(t *testing.T) {
	store, ctx := newStore(t)
	players := store.Players()

	ella := mustPlayer(ctx, t, store, "Ella", domain.DefaultTTR)

	// The things that go with her: a sign-in proof and a PIN.
	if err := store.Identities().Link(ctx, domain.ProviderLocal, "ella-subject", ella.ID); err != nil {
		t.Fatalf("Link(): %v", err)
	}
	if err := store.Credentials().Put(ctx, ella.ID, domain.CredentialPIN, "not-a-real-hash"); err != nil {
		t.Fatalf("Put(): %v", err)
	}

	if err := players.Delete(ctx, ella.ID); err != nil {
		t.Fatalf("Delete(): %v", err)
	}

	if _, err := players.ByID(ctx, ella.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("ByID() = %v, want ErrNotFound", err)
	}
	// The cascades did their work, or a browser stays signed in as somebody
	// who no longer exists.
	if _, err := store.Identities().PlayerBy(ctx, domain.ProviderLocal, "ella-subject"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("the identity survived the delete: %v", err)
	}
	if _, err := store.Credentials().ForPlayer(ctx, ella.ID, domain.CredentialPIN); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("the PIN survived the delete: %v", err)
	}

	// Deleting again is not a crash but an answer.
	if err := players.Delete(ctx, ella.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Delete() twice = %v, want ErrNotFound", err)
	}
}

// A match makes somebody permanent, and so does having reported one — the
// foreign keys say so, and this is what makes "wer einmal gespielt hat,
// bleibt" a property of the data rather than a rule a handler remembers.
func TestDeletingAPlayerWithAHistoryIsRefused(t *testing.T) {
	store, ctx := newStore(t)
	players := store.Players()

	anna := mustPlayer(ctx, t, store, "Anna", domain.DefaultTTR)
	bodo := mustPlayer(ctx, t, store, "Bodo", domain.DefaultTTR)
	cleo := mustPlayer(ctx, t, store, "Cleo", domain.DefaultTTR)

	// Cleo neither plays nor is named: she reports somebody else's result.
	if _, err := store.Matches().Create(ctx, domain.Match{
		HomeID: anna.ID, AwayID: bodo.ID, BestOf: 3, PointsToWin: 11,
		Status: domain.MatchPending, ReportedBy: cleo.ID,
		Sets: []domain.MatchSet{
			{SetNo: 1, HomePoints: 11, AwayPoints: 9},
			{SetNo: 2, HomePoints: 11, AwayPoints: 7},
		},
	}); err != nil {
		t.Fatalf("create the match: %v", err)
	}

	for _, tc := range []struct {
		name   string
		player domain.Player
	}{
		{"home", anna},
		{"away", bodo},
		{"the reporter", cleo},
	} {
		if err := players.Delete(ctx, tc.player.ID); !errors.Is(err, domain.ErrInUse) {
			t.Errorf("Delete(%s) = %v, want ErrInUse", tc.name, err)
		}
		if _, err := players.ByID(ctx, tc.player.ID); err != nil {
			t.Errorf("%s did not survive the refused delete: %v", tc.name, err)
		}
	}
}

// A pending result counts too. The page offers a shortlist built from
// confirmed matches, so this is the case where the shortlist is wrong and the
// database has to be the one that says no.
func TestDeletingAPlayerWithOnlyAPendingResultIsRefused(t *testing.T) {
	store, ctx := newStore(t)

	anna := mustPlayer(ctx, t, store, "Anna", domain.DefaultTTR)
	bodo := mustPlayer(ctx, t, store, "Bodo", domain.DefaultTTR)

	if _, err := store.Matches().Create(ctx, domain.Match{
		HomeID: anna.ID, AwayID: bodo.ID, BestOf: 1, PointsToWin: 11,
		Status: domain.MatchPending, ReportedBy: anna.ID,
		Sets: []domain.MatchSet{{SetNo: 1, HomePoints: 11, AwayPoints: 5}},
	}); err != nil {
		t.Fatalf("create the match: %v", err)
	}

	records, err := store.Players().Records(ctx)
	if err != nil {
		t.Fatalf("Records(): %v", err)
	}
	for _, r := range records {
		if r.Player.ID == bodo.ID && r.Played != 0 {
			t.Fatalf("Records() counts %d for Bodo, want 0 — the shortlist is built on this", r.Played)
		}
	}

	if err := store.Players().Delete(ctx, bodo.ID); !errors.Is(err, domain.ErrInUse) {
		t.Errorf("Delete() = %v, want ErrInUse", err)
	}
}
