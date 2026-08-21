package postgres_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/repository"
	"github.com/stuttgart-things/schmetterpause/internal/repository/postgres"
)

// Diese Tests brauchen eine echte Datenbank und laufen nur, wenn
// SP_TEST_DATABASE_URL gesetzt ist:
//
//	task test:integration
//
// Sie leeren alle Tabellen. Die Variable muss auf eine Wegwerf-Datenbank
// zeigen, niemals auf eine mit echten Daten.
const testDSNEnv = "SP_TEST_DATABASE_URL"

func newStore(t *testing.T) (*postgres.Store, context.Context) {
	t.Helper()

	dsn := os.Getenv(testDSNEnv)
	if dsn == "" {
		t.Skipf("%s nicht gesetzt — Integrationstest übersprungen", testDSNEnv)
	}

	ctx := t.Context()

	if err := postgres.Migrate(ctx, dsn); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}

	store, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	t.Cleanup(store.Close)

	truncate(ctx, t, store)
	return store, ctx
}

// truncate leert alle Tabellen ueber die Repository-Ebene hinweg. Der einzige
// Ort im Projekt, an dem Testcode SQL absetzt.
func truncate(ctx context.Context, t *testing.T, store *postgres.Store) {
	t.Helper()

	err := store.InTx(ctx, func(repository.Store) error { return nil })
	if err != nil {
		t.Fatalf("Datenbank nicht nutzbar: %v", err)
	}
	if err := postgres.TruncateAll(ctx, store); err != nil {
		t.Fatalf("Tabellen leeren: %v", err)
	}
}

func mustPlayer(ctx context.Context, t *testing.T, store *postgres.Store, name string, ttr int) domain.Player {
	t.Helper()

	p, err := store.Players().Create(ctx, name, ttr)
	if err != nil {
		t.Fatalf("Spieler %q anlegen: %v", name, err)
	}
	return p
}

func TestPlayerRepository(t *testing.T) {
	store, ctx := newStore(t)
	players := store.Players()

	anna := mustPlayer(ctx, t, store, "Anna", 1100)
	bodo := mustPlayer(ctx, t, store, "Bodo", 900)

	got, err := players.ByID(ctx, anna.ID)
	if err != nil {
		t.Fatalf("ByID(): %v", err)
	}
	if got.DisplayName != "Anna" || got.TTR != 1100 {
		t.Errorf("ByID() = %+v, erwartet Anna/1100", got)
	}

	n, err := players.Count(ctx)
	if err != nil {
		t.Fatalf("Count(): %v", err)
	}
	if n != 2 {
		t.Errorf("Count() = %d, erwartet 2", n)
	}

	// Die Liste ist die Reihenfolge der Rangliste: bestes TTR zuerst.
	list, err := players.List(ctx)
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if len(list) != 2 || list[0].ID != anna.ID || list[1].ID != bodo.ID {
		t.Errorf("List() liefert falsche Reihenfolge: %+v", list)
	}

	if err := players.UpdateTTR(ctx, bodo.ID, 1250); err != nil {
		t.Fatalf("UpdateTTR(): %v", err)
	}
	list, err = players.List(ctx)
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if list[0].ID != bodo.ID {
		t.Errorf("Bodo steht nach dem TTR-Update nicht vorn: %+v", list)
	}

	if _, err := players.ByID(ctx, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("ByID() für unbekannten Spieler = %v, erwartet domain.ErrNotFound", err)
	}
	if err := players.UpdateTTR(ctx, uuid.New(), 1000); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("UpdateTTR() für unbekannten Spieler = %v, erwartet domain.ErrNotFound", err)
	}
}

func TestIdentityRepository(t *testing.T) {
	store, ctx := newStore(t)
	ids := store.Identities()

	anna := mustPlayer(ctx, t, store, "Anna", domain.DefaultTTR)
	bodo := mustPlayer(ctx, t, store, "Bodo", domain.DefaultTTR)

	if err := ids.Link(ctx, domain.ProviderLocal, "cookie-anna", anna.ID); err != nil {
		t.Fatalf("Link(): %v", err)
	}

	got, err := ids.PlayerBy(ctx, domain.ProviderLocal, "cookie-anna")
	if err != nil {
		t.Fatalf("PlayerBy(): %v", err)
	}
	if got.ID != anna.ID {
		t.Errorf("PlayerBy() = %s, erwartet %s", got.ID, anna.ID)
	}

	// Erneutes Verknüpfen auf denselben Spieler ist ein No-op.
	if err := ids.Link(ctx, domain.ProviderLocal, "cookie-anna", anna.ID); err != nil {
		t.Errorf("wiederholtes Link() = %v, erwartet nil", err)
	}

	// Auf einen anderen Spieler bleibt es ein Konflikt: das Zusammenführen
	// zweier Spieler ist laut ADR-0003 eine eigene, bewusste Operation.
	if err := ids.Link(ctx, domain.ProviderLocal, "cookie-anna", bodo.ID); err == nil {
		t.Error("Link() auf einen anderen Spieler lieferte keinen Fehler")
	}

	// Ein Spieler kann mehrere Identitäten haben.
	if err := ids.Link(ctx, domain.ProviderPasskey, "credential-1", anna.ID); err != nil {
		t.Fatalf("zweite Identität verknüpfen: %v", err)
	}
	list, err := ids.ForPlayer(ctx, anna.ID)
	if err != nil {
		t.Fatalf("ForPlayer(): %v", err)
	}
	if len(list) != 2 {
		t.Errorf("ForPlayer() = %d Identitäten, erwartet 2", len(list))
	}

	if _, err := ids.PlayerBy(ctx, domain.ProviderGitLab, "unbekannt"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("PlayerBy() für unbekannte Identität = %v, erwartet domain.ErrNotFound", err)
	}
}

func TestMatchRepository(t *testing.T) {
	store, ctx := newStore(t)
	matches := store.Matches()

	anna := mustPlayer(ctx, t, store, "Anna", domain.DefaultTTR)
	bodo := mustPlayer(ctx, t, store, "Bodo", domain.DefaultTTR)

	created, err := matches.Create(ctx, domain.Match{
		HomeID:      anna.ID,
		AwayID:      bodo.ID,
		BestOf:      5,
		PointsToWin: 11,
		ReportedBy:  anna.ID,
		Sets: []domain.MatchSet{
			{SetNo: 1, HomePoints: 11, AwayPoints: 7},
			{SetNo: 2, HomePoints: 9, AwayPoints: 11},
			{SetNo: 3, HomePoints: 11, AwayPoints: 13},
			{SetNo: 4, HomePoints: 11, AwayPoints: 8},
			{SetNo: 5, HomePoints: 12, AwayPoints: 10},
		},
	})
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	if created.Status != domain.MatchPending {
		t.Errorf("Status = %q, erwartet pending", created.Status)
	}

	got, err := matches.ByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("ByID(): %v", err)
	}
	if len(got.Sets) != 5 {
		t.Fatalf("ByID() liefert %d Sätze, erwartet 5", len(got.Sets))
	}
	if got.Sets[2].AwayPoints != 13 {
		t.Errorf("dritter Satz = %+v, erwartet 11:13", got.Sets[2])
	}

	// Bestätigen muss der Gegner: Anna hat eingetragen, also wartet das Match
	// auf Bodo und nicht auf Anna.
	pendingForBodo, err := matches.PendingFor(ctx, bodo.ID)
	if err != nil {
		t.Fatalf("PendingFor(bodo): %v", err)
	}
	if len(pendingForBodo) != 1 {
		t.Errorf("PendingFor(bodo) = %d, erwartet 1", len(pendingForBodo))
	}
	pendingForAnna, err := matches.PendingFor(ctx, anna.ID)
	if err != nil {
		t.Fatalf("PendingFor(anna): %v", err)
	}
	if len(pendingForAnna) != 0 {
		t.Errorf("PendingFor(anna) = %d, erwartet 0", len(pendingForAnna))
	}

	now := time.Now()
	if err := matches.SetStatus(ctx, created.ID, domain.MatchConfirmed, &now); err != nil {
		t.Fatalf("SetStatus(): %v", err)
	}
	got, err = matches.ByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("ByID() nach Bestätigung: %v", err)
	}
	if got.Status != domain.MatchConfirmed || got.ConfirmedAt == nil {
		t.Errorf("nach Bestätigung: Status=%q, ConfirmedAt=%v", got.Status, got.ConfirmedAt)
	}

	recent, err := matches.RecentFor(ctx, anna.ID, 10)
	if err != nil {
		t.Fatalf("RecentFor(): %v", err)
	}
	if len(recent) != 1 || len(recent[0].Sets) != 5 {
		t.Errorf("RecentFor() = %d Matches mit %d Sätzen", len(recent), len(recent[0].Sets))
	}

	if _, err := matches.ByID(ctx, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("ByID() für unbekanntes Match = %v, erwartet domain.ErrNotFound", err)
	}
}

func TestTTRHistoryRepository(t *testing.T) {
	store, ctx := newStore(t)

	anna := mustPlayer(ctx, t, store, "Anna", domain.DefaultTTR)
	bodo := mustPlayer(ctx, t, store, "Bodo", domain.DefaultTTR)

	match, err := store.Matches().Create(ctx, domain.Match{
		HomeID: anna.ID, AwayID: bodo.ID, BestOf: 3, PointsToWin: 11, ReportedBy: anna.ID,
		Sets: []domain.MatchSet{{SetNo: 1, HomePoints: 11, AwayPoints: 5}, {SetNo: 2, HomePoints: 11, AwayPoints: 9}},
	})
	if err != nil {
		t.Fatalf("Match anlegen: %v", err)
	}

	changes := []domain.TTRChange{
		{PlayerID: anna.ID, MatchID: match.ID, TTRBefore: 1000, TTRAfter: 1008},
		{PlayerID: bodo.ID, MatchID: match.ID, TTRBefore: 1000, TTRAfter: 992},
	}
	if err := store.TTRHistory().Append(ctx, changes); err != nil {
		t.Fatalf("Append(): %v", err)
	}

	got, err := store.TTRHistory().ForPlayer(ctx, anna.ID, 10)
	if err != nil {
		t.Fatalf("ForPlayer(): %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ForPlayer() = %d Einträge, erwartet 1", len(got))
	}
	if got[0].Delta() != 8 {
		t.Errorf("Delta() = %d, erwartet 8", got[0].Delta())
	}

	// Pro Spieler und Match darf es nur einen Eintrag geben — sonst liesse
	// sich eine Wertung versehentlich zweimal verbuchen.
	if err := store.TTRHistory().Append(ctx, changes[:1]); err == nil {
		t.Error("doppelter Eintrag für dasselbe Match wurde akzeptiert")
	}
}

func TestInTxRollback(t *testing.T) {
	store, ctx := newStore(t)

	sentinel := errors.New("abbruch")

	err := store.InTx(ctx, func(tx repository.Store) error {
		if _, err := tx.Players().Create(ctx, "Wird verworfen", domain.DefaultTTR); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("InTx() = %v, erwartet %v", err, sentinel)
	}

	n, err := store.Players().Count(ctx)
	if err != nil {
		t.Fatalf("Count(): %v", err)
	}
	if n != 0 {
		t.Errorf("nach Rollback sind %d Spieler da, erwartet 0", n)
	}
}

func TestInTxCommit(t *testing.T) {
	store, ctx := newStore(t)

	err := store.InTx(ctx, func(tx repository.Store) error {
		_, err := tx.Players().Create(ctx, "Bleibt", domain.DefaultTTR)
		return err
	})
	if err != nil {
		t.Fatalf("InTx(): %v", err)
	}

	n, err := store.Players().Count(ctx)
	if err != nil {
		t.Fatalf("Count(): %v", err)
	}
	if n != 1 {
		t.Errorf("nach Commit sind %d Spieler da, erwartet 1", n)
	}
}
