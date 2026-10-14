package postgres_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// docs/adr/0025: at most one window, replaced only once it has expired or by
// the player who holds it — decided by one upsert, so it holds across
// replicas.
func TestFobWindowRepository(t *testing.T) {
	store, ctx := newStore(t)
	windows := store.FobWindow()

	anna, err := store.Players().Create(ctx, "Anna", domain.DefaultTTR)
	if err != nil {
		t.Fatal(err)
	}
	bea, err := store.Players().Create(ctx, "Bea", domain.DefaultTTR)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().Truncate(time.Microsecond)
	minute := now.Add(time.Minute)

	if _, err := windows.Current(ctx, now); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Current() with no window = %v, want domain.ErrNotFound", err)
	}

	w, err := windows.Open(ctx, anna.ID, now, minute)
	if err != nil || w.PlayerID != anna.ID || !w.ExpiresAt.Equal(minute) {
		t.Fatalf("Open() = %+v, %v; want Anna's window until %s", w, err, minute)
	}

	// Somebody else, while it is open: refused, and told whose and until when.
	other, err := windows.Open(ctx, bea.ID, now.Add(time.Second), minute.Add(time.Second))
	if !errors.Is(err, domain.ErrConflict) || other.PlayerID != anna.ID {
		t.Errorf("Open() for Bea during Anna's window = %+v, %v; want ErrConflict with Anna's", other, err)
	}

	// The same player again extends their own.
	if w, err := windows.Open(ctx, anna.ID, now.Add(time.Second), minute.Add(time.Second)); err != nil ||
		!w.ExpiresAt.Equal(minute.Add(time.Second)) {
		t.Errorf("Open() for Anna again = %+v, %v; want her window extended", w, err)
	}

	// Once expired, it is anybody's.
	later := minute.Add(time.Hour)
	if w, err := windows.Open(ctx, bea.ID, later, later.Add(time.Minute)); err != nil || w.PlayerID != bea.ID {
		t.Fatalf("Open() for Bea after expiry = %+v, %v; want Bea's window", w, err)
	}

	// Consume takes it once.
	if w, err := windows.Consume(ctx, later.Add(time.Second)); err != nil || w.PlayerID != bea.ID {
		t.Errorf("Consume() = %+v, %v; want Bea's window", w, err)
	}
	if _, err := windows.Consume(ctx, later.Add(time.Second)); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("a second Consume() = %v, want domain.ErrNotFound", err)
	}

	// An expired window cannot be consumed.
	if _, err := windows.Open(ctx, anna.ID, later, later.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := windows.Consume(ctx, later.Add(2*time.Minute)); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Consume() after expiry = %v, want domain.ErrNotFound", err)
	}

	// Close removes only your own.
	if _, err := windows.Open(ctx, anna.ID, later.Add(time.Hour), later.Add(time.Hour+time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := windows.Close(ctx, bea.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := windows.Current(ctx, later.Add(time.Hour)); err != nil {
		t.Errorf("Bea closed Anna's window: Current() = %v", err)
	}
	if err := windows.Close(ctx, anna.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := windows.Current(ctx, later.Add(time.Hour)); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Current() after Anna closed hers = %v, want domain.ErrNotFound", err)
	}
}

func TestCheckinsAndFobIdentities(t *testing.T) {
	store, ctx := newStore(t)

	anna, err := store.Players().Create(ctx, "Anna", domain.DefaultTTR)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Truncate(time.Microsecond)

	c, err := store.Checkins().Create(ctx, domain.Checkin{
		PlayerID: anna.ID, Half: domain.HalfLeft, Method: domain.CheckinRFID, CreatedAt: now,
	})
	if err != nil || c.ID.String() == "" {
		t.Fatalf("Create() = %+v, %v", c, err)
	}
	if _, err := store.Checkins().Create(ctx, domain.Checkin{
		PlayerID: anna.ID, Half: "middle", Method: domain.CheckinRFID, CreatedAt: now,
	}); err == nil {
		t.Error("a check-in at half \"middle\" was accepted; the constraint names left and right")
	}

	ids := store.Identities()
	if err := ids.Touch(ctx, domain.ProviderRFID, "nobody", now); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Touch() of an unknown fob = %v, want domain.ErrNotFound", err)
	}
	if err := ids.Link(ctx, domain.ProviderRFID, "abc", anna.ID); err != nil {
		t.Fatal(err)
	}
	if err := ids.Touch(ctx, domain.ProviderRFID, "abc", now); err != nil {
		t.Fatal(err)
	}
	list, err := ids.ForPlayer(ctx, anna.ID)
	if err != nil || len(list) != 1 || list[0].LastUsedAt == nil || !list[0].LastUsedAt.Equal(now) {
		t.Errorf("ForPlayer() = %+v, %v; want one fob last used at %s", list, err, now)
	}

	// A player who never played can be deleted, and their check-ins and
	// window go with them.
	if _, err := store.FobWindow().Open(ctx, anna.ID, now, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := store.Players().Delete(ctx, anna.ID); err != nil {
		t.Errorf("deleting a player with a check-in and a window: %v", err)
	}
}
