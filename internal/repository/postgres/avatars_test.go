package postgres_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/avatar"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// docs/adr/0029: one stored figure per player, replaced on change, gone on
// reset and with the player. No row is "has not chosen".
func TestAvatarRepository(t *testing.T) {
	store, ctx := newStore(t)
	avatars := store.Avatars()

	anna, err := store.Players().Create(ctx, "Anna", domain.DefaultTTR)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := avatars.Get(ctx, anna.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Get() before choosing = %v, want domain.ErrNotFound", err)
	}

	first := avatar.Avatar{Fur: "cream", Headdress: "lapis", Handle: "wood-dark", Blade: "pink", Bling: "chain"}
	if err := avatars.Put(ctx, anna.ID, first); err != nil {
		t.Fatalf("Put() = %v", err)
	}
	if got, err := avatars.Get(ctx, anna.ID); err != nil || got != first {
		t.Errorf("Get() = %+v, %v; want %+v", got, err, first)
	}

	// A change replaces the row, bling taken off included.
	second := avatar.Avatar{Fur: "white", Headdress: avatar.SameAsFur, Handle: "red", Blade: "steel", Bling: avatar.NoBling}
	if err := avatars.Put(ctx, anna.ID, second); err != nil {
		t.Fatalf("Put() again = %v", err)
	}
	if got, err := avatars.Get(ctx, anna.ID); err != nil || got != second {
		t.Errorf("Get() after a change = %+v, %v; want %+v", got, err, second)
	}

	// Reset, twice: the second one finds nothing and still succeeds.
	for range 2 {
		if err := avatars.Delete(ctx, anna.ID); err != nil {
			t.Fatalf("Delete() = %v", err)
		}
	}
	if _, err := avatars.Get(ctx, anna.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Get() after reset = %v, want domain.ErrNotFound", err)
	}

	// Somebody who does not exist cannot be dressed.
	if err := avatars.Put(ctx, uuid.New(), first); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Put() for nobody = %v, want domain.ErrNotFound", err)
	}

	// The figure goes with the player.
	if err := avatars.Put(ctx, anna.ID, first); err != nil {
		t.Fatal(err)
	}
	if err := store.Players().Delete(ctx, anna.ID); err != nil {
		t.Fatalf("deleting Anna: %v", err)
	}
	if _, err := avatars.Get(ctx, anna.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Get() after the player was deleted = %v, want domain.ErrNotFound", err)
	}
}

// A stored key the code no longer knows is kept as it is. Falling back is
// avatar.Fill's job, at read time, so taking a colour out of a palette needs
// no migration (docs/adr/0029).
func TestAvatarRepositoryKeepsUnknownKeys(t *testing.T) {
	store, ctx := newStore(t)
	bea, err := store.Players().Create(ctx, "Bea", domain.DefaultTTR)
	if err != nil {
		t.Fatal(err)
	}
	stale := avatar.Avatar{Fur: "plaid", Headdress: "gold", Handle: avatar.SameAsFur, Blade: "blue", Bling: "crown"}
	if err := store.Avatars().Put(ctx, bea.ID, stale); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Avatars().Get(ctx, bea.ID); err != nil || got != stale {
		t.Errorf("Get() = %+v, %v; want the stored keys untouched", got, err)
	}
}
