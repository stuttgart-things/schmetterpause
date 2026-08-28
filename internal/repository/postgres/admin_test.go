package postgres_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// The admin flag, per ADR-0008: a property of the person rather than of a
// browser, which is what makes it revocable and lets a log line name
// somebody.
func TestTheAdminFlag(t *testing.T) {
	store, ctx := newStore(t)
	players := store.Players()

	anna := mustPlayer(ctx, t, store, "Anna", domain.DefaultTTR)
	mustPlayer(ctx, t, store, "Bodo", domain.DefaultTTR)

	// Nobody is one by default. The flag is granted, never inherited.
	admins, err := players.Admins(ctx)
	if err != nil {
		t.Fatalf("Admins(): %v", err)
	}
	if len(admins) != 0 {
		t.Fatalf("Admins() = %v on a fresh table, want nobody", admins)
	}

	if err := players.SetAdmin(ctx, anna.ID, true); err != nil {
		t.Fatalf("SetAdmin(): %v", err)
	}

	admins, err = players.Admins(ctx)
	if err != nil {
		t.Fatalf("Admins(): %v", err)
	}
	if len(admins) != 1 || admins[0].ID != anna.ID {
		t.Fatalf("Admins() = %v, want only Anna", admins)
	}

	// And it has to come back on the ordinary reads, or nothing outside this
	// list can tell.
	back, err := players.ByID(ctx, anna.ID)
	if err != nil || !back.IsAdmin {
		t.Errorf("ByID() = %v, %v, want an admin", back.IsAdmin, err)
	}
	all, err := players.List(ctx)
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	for _, p := range all {
		if (p.ID == anna.ID) != p.IsAdmin {
			t.Errorf("List() reports %q as admin=%v", p.DisplayName, p.IsAdmin)
		}
	}

	// Withdrawing works, which is the property the kiosk's constant cookie
	// does not have (#77).
	if err := players.SetAdmin(ctx, anna.ID, false); err != nil {
		t.Fatalf("SetAdmin(false): %v", err)
	}
	admins, _ = players.Admins(ctx)
	if len(admins) != 0 {
		t.Errorf("Admins() = %v after withdrawing, want nobody", admins)
	}

	if err := players.SetAdmin(ctx, uuid.New(), true); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("SetAdmin() for an unknown player = %v, want domain.ErrNotFound", err)
	}
}

// ByDisplayName is what SP_BOOTSTRAP_ADMIN resolves through, so it has to
// match the way players_display_name_key is unique: trimmed and folded.
func TestByDisplayName(t *testing.T) {
	store, ctx := newStore(t)

	anna := mustPlayer(ctx, t, store, "Anna", domain.DefaultTTR)

	for _, name := range []string{"Anna", "anna", "  ANNA  ", "aNnA"} {
		got, err := store.Players().ByDisplayName(ctx, name)
		if err != nil {
			t.Errorf("ByDisplayName(%q): %v", name, err)
			continue
		}
		if got.ID != anna.ID {
			t.Errorf("ByDisplayName(%q) = %s, want %s", name, got.ID, anna.ID)
		}
	}

	if _, err := store.Players().ByDisplayName(ctx, "Niemand"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("ByDisplayName() for an unknown name = %v, want domain.ErrNotFound", err)
	}
}
