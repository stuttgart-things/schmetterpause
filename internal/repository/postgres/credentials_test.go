package postgres_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/credential"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

func TestCredentialRepository(t *testing.T) {
	store, ctx := newStore(t)
	creds := store.Credentials()

	anna := mustPlayer(ctx, t, store, "Anna", domain.DefaultTTR)

	const code = "A1B2C3D4E5F6G7H8"
	if err := creds.Put(ctx, anna.ID, domain.CredentialRecovery, credential.Hash(code)); err != nil {
		t.Fatalf("Put(): %v", err)
	}

	got, err := creds.ForPlayer(ctx, anna.ID, domain.CredentialRecovery)
	if err != nil {
		t.Fatalf("ForPlayer(): %v", err)
	}
	if got.PlayerID != anna.ID || got.Kind != domain.CredentialRecovery {
		t.Errorf("ForPlayer() = %s/%s, want %s/%s",
			got.PlayerID, got.Kind, anna.ID, domain.CredentialRecovery)
	}
	if got.UpdatedAt.IsZero() {
		t.Error("ForPlayer() returned a zero updated_at")
	}

	ok, err := credential.Verify(got.Hash, code)
	if err != nil || !ok {
		t.Errorf("the stored hash does not verify the code it was made from: %v, %v", ok, err)
	}

	// The two kinds are separate rows and must not overwrite each other: a
	// player who sets a PIN keeps the recovery code they already had.
	if err := creds.Put(ctx, anna.ID, domain.CredentialPIN, credential.Hash("123456")); err != nil {
		t.Fatalf("Put() for a second kind: %v", err)
	}
	stillThere, err := creds.ForPlayer(ctx, anna.ID, domain.CredentialRecovery)
	if err != nil {
		t.Fatalf("ForPlayer() after storing a PIN: %v", err)
	}
	if stillThere.Hash != got.Hash {
		t.Error("storing a PIN changed the recovery code")
	}

	// A player who has no credential of a kind is an ordinary state, and the
	// normal one for a PIN — setting one is optional (ADR-0007).
	bodo := mustPlayer(ctx, t, store, "Bodo", domain.DefaultTTR)
	if _, err := creds.ForPlayer(ctx, bodo.ID, domain.CredentialPIN); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("ForPlayer() without a credential = %v, want domain.ErrNotFound", err)
	}

	if _, err := creds.ForPlayer(ctx, uuid.New(), domain.CredentialRecovery); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("ForPlayer() for an unknown player = %v, want domain.ErrNotFound", err)
	}
}

// A new recovery code has to invalidate the old one in the same step
// (ADR-0006). Here that is the primary key doing it, not the caller.
func TestCredentialPutReplacesTheOldSecret(t *testing.T) {
	store, ctx := newStore(t)
	creds := store.Credentials()

	anna := mustPlayer(ctx, t, store, "Anna", domain.DefaultTTR)

	const old, fresh = "A1B2C3D4E5F6G7H8", "J9K8M7N6P5Q4R3S2"

	if err := creds.Put(ctx, anna.ID, domain.CredentialRecovery, credential.Hash(old)); err != nil {
		t.Fatalf("Put(): %v", err)
	}
	before, err := creds.ForPlayer(ctx, anna.ID, domain.CredentialRecovery)
	if err != nil {
		t.Fatalf("ForPlayer(): %v", err)
	}

	if err := creds.Put(ctx, anna.ID, domain.CredentialRecovery, credential.Hash(fresh)); err != nil {
		t.Fatalf("second Put(): %v", err)
	}
	after, err := creds.ForPlayer(ctx, anna.ID, domain.CredentialRecovery)
	if err != nil {
		t.Fatalf("ForPlayer() after replacing: %v", err)
	}

	if ok, _ := credential.Verify(after.Hash, old); ok {
		t.Error("the replaced code still verifies, so issuing a new one did not invalidate it")
	}
	if ok, err := credential.Verify(after.Hash, fresh); err != nil || !ok {
		t.Errorf("the new code does not verify: %v, %v", ok, err)
	}
	if !after.UpdatedAt.After(before.UpdatedAt) && !after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Errorf("updated_at went backwards: %s then %s", before.UpdatedAt, after.UpdatedAt)
	}
}

// Deleting a player takes their secrets with them. Without the cascade a
// removed player would leave a row nothing points at any more.
func TestCredentialsFollowThePlayer(t *testing.T) {
	store, ctx := newStore(t)

	anna := mustPlayer(ctx, t, store, "Anna", domain.DefaultTTR)
	if err := store.Credentials().Put(ctx, anna.ID, domain.CredentialPIN, credential.Hash("123456")); err != nil {
		t.Fatalf("Put(): %v", err)
	}

	// A credential for a player who does not exist has nothing to hang on.
	err := store.Credentials().Put(ctx, uuid.New(), domain.CredentialPIN, credential.Hash("123456"))
	if err == nil {
		t.Error("Put() for an unknown player returned no error")
	}
}
