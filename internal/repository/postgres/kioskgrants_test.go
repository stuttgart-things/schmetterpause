package postgres_test

import (
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

func hashOf(secret string) []byte {
	h := sha256.Sum256([]byte(secret))
	return h[:]
}

// Issue #77: a row per machine answers the two questions a derived constant
// cannot — which machines are kiosks right now, and how do I take one back.
func TestKioskGrantRepository(t *testing.T) {
	store, ctx := newStore(t)
	grants := store.KioskGrants()

	now := time.Now()
	later := now.Add(12 * time.Hour)

	table, err := grants.Create(ctx, hashOf("table"), later, "Turnier-Laptop")
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	if table.UserAgent != "Turnier-Laptop" || table.RevokedAt != nil {
		t.Errorf("Create() = %+v, want an active grant with its label", table)
	}
	if !table.Active(now) {
		t.Error("a fresh grant is not active")
	}

	shoulder, err := grants.Create(ctx, hashOf("shoulder"), later, "Fremdes Handy")
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}

	found, err := grants.BySecret(ctx, hashOf("table"))
	if err != nil {
		t.Fatalf("BySecret(): %v", err)
	}
	if found.ID != table.ID {
		t.Errorf("BySecret() = %s, want %s", found.ID, table.ID)
	}

	// A cookie nothing stands for is the ordinary answer for a machine
	// somebody took back, not a failure.
	if _, err := grants.BySecret(ctx, hashOf("never-issued")); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("BySecret() for an unknown secret = %v, want domain.ErrNotFound", err)
	}

	active, err := grants.Active(ctx, now)
	if err != nil {
		t.Fatalf("Active(): %v", err)
	}
	if len(active) != 2 {
		t.Fatalf("Active() = %d, want 2", len(active))
	}

	// Taking one back leaves the other alone. Under the old constant this was
	// impossible: one value, so revoking meant revoking everybody.
	if err := grants.Revoke(ctx, shoulder.ID, now); err != nil {
		t.Fatalf("Revoke(): %v", err)
	}
	active, _ = grants.Active(ctx, now)
	if len(active) != 1 || active[0].ID != table.ID {
		t.Fatalf("Active() = %v, want only the table", active)
	}

	// The revoked one stops resolving, so its cookie is worth nothing.
	back, err := grants.BySecret(ctx, hashOf("shoulder"))
	if err != nil {
		t.Fatalf("BySecret() after revoking: %v", err)
	}
	if back.Active(now) {
		t.Error("a revoked grant still reports itself active")
	}

	// Two people pressing the same button is not a failure, and the second
	// press must not move the timestamp.
	firstRevocation := *back.RevokedAt
	if err := grants.Revoke(ctx, shoulder.ID, now.Add(time.Hour)); err != nil {
		t.Errorf("revoking twice = %v, want nil", err)
	}
	again, _ := grants.BySecret(ctx, hashOf("shoulder"))
	if !again.RevokedAt.Equal(firstRevocation) {
		t.Errorf("the second revocation moved the timestamp: %s then %s",
			firstRevocation, again.RevokedAt)
	}

	if err := grants.Revoke(ctx, uuid.New(), now); err != nil {
		t.Errorf("revoking something that does not exist = %v, want nil", err)
	}
}

// Expiry is the other way a grant stops counting, and it needs no button.
//
// Asked by moving the clock forward rather than by writing a row that has
// already expired: the schema forbids that (kiosk_grants_expires_after_creation),
// and rightly — a grant is issued to last some time. Active takes the moment
// as a parameter for exactly this reason.
func TestKioskGrantsExpire(t *testing.T) {
	store, ctx := newStore(t)
	grants := store.KioskGrants()

	now := time.Now()

	if _, err := grants.Create(ctx, hashOf("short"), now.Add(time.Hour), "Kurz"); err != nil {
		t.Fatalf("Create(): %v", err)
	}
	if _, err := grants.Create(ctx, hashOf("long"), now.Add(12*time.Hour), "Lang"); err != nil {
		t.Fatalf("Create(): %v", err)
	}

	// Two hours later, one of them has run out.
	later := now.Add(2 * time.Hour)

	active, err := grants.Active(ctx, later)
	if err != nil {
		t.Fatalf("Active(): %v", err)
	}
	if len(active) != 1 || active[0].UserAgent != "Lang" {
		t.Errorf("Active() = %v, want only the one that still runs", active)
	}

	// The row is still there and still resolves — it just no longer unlocks
	// anything, which is what the guard in the handler reads.
	expired, err := grants.BySecret(ctx, hashOf("short"))
	if err != nil {
		t.Fatalf("BySecret(): %v", err)
	}
	if expired.Active(later) {
		t.Error("an expired grant reports itself active")
	}
	if !expired.Active(now) {
		t.Error("the same grant was not active before it ran out")
	}
}

// The answer to "somebody read the token over a shoulder" that does not
// involve a restart.
func TestKioskGrantsRevokeAll(t *testing.T) {
	store, ctx := newStore(t)
	grants := store.KioskGrants()

	now := time.Now()
	for _, s := range []string{"a", "b", "c"} {
		if _, err := grants.Create(ctx, hashOf(s), now.Add(time.Hour), s); err != nil {
			t.Fatalf("Create(%q): %v", s, err)
		}
	}

	n, err := grants.RevokeAll(ctx, now)
	if err != nil {
		t.Fatalf("RevokeAll(): %v", err)
	}
	if n != 3 {
		t.Errorf("RevokeAll() = %d, want 3", n)
	}

	active, _ := grants.Active(ctx, now)
	if len(active) != 0 {
		t.Errorf("Active() = %v, want none", active)
	}

	// Nothing left to take back.
	n, err = grants.RevokeAll(ctx, now)
	if err != nil || n != 0 {
		t.Errorf("RevokeAll() again = %d, %v, want 0, nil", n, err)
	}

	// An expired grant is not "taken back" either — it was already gone, and
	// counting it would tell somebody they revoked more than they did.
	if _, err := grants.Create(ctx, hashOf("expiring"), now.Add(time.Hour), "läuft ab"); err != nil {
		t.Fatalf("Create(): %v", err)
	}
	n, err = grants.RevokeAll(ctx, now.Add(2*time.Hour))
	if err != nil || n != 0 {
		t.Errorf("RevokeAll() after it ran out = %d, %v, want 0, nil", n, err)
	}
}

// Touch is what makes the list readable: a grant nobody has used since
// Tuesday is a laptop somebody took home.
func TestKioskGrantTouch(t *testing.T) {
	store, ctx := newStore(t)
	grants := store.KioskGrants()

	now := time.Now()
	g, err := grants.Create(ctx, hashOf("laptop"), now.Add(time.Hour), "Laptop")
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}

	seen := now.Add(30 * time.Minute)
	if err := grants.Touch(ctx, g.ID, seen); err != nil {
		t.Fatalf("Touch(): %v", err)
	}

	back, err := grants.BySecret(ctx, hashOf("laptop"))
	if err != nil {
		t.Fatalf("BySecret(): %v", err)
	}
	if !back.LastSeenAt.Truncate(time.Second).Equal(seen.UTC().Truncate(time.Second)) {
		t.Errorf("LastSeenAt = %s, want %s", back.LastSeenAt, seen)
	}
}

// The unique index is what makes a secret name exactly one machine.
func TestKioskGrantSecretsAreUnique(t *testing.T) {
	store, ctx := newStore(t)
	grants := store.KioskGrants()

	now := time.Now()
	if _, err := grants.Create(ctx, hashOf("same"), now.Add(time.Hour), "erst"); err != nil {
		t.Fatalf("Create(): %v", err)
	}
	if _, err := grants.Create(ctx, hashOf("same"), now.Add(time.Hour), "dann"); err == nil {
		t.Error("the same secret was accepted twice")
	}
}
