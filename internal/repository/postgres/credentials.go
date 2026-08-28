package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

type credentialRepo struct{ q queryer }

func (r credentialRepo) Put(ctx context.Context, playerID uuid.UUID, kind domain.CredentialKind, hash string) error {
	// One statement rather than a delete and an insert: between the two there
	// would be a moment in which the player has no credential of this kind at
	// all, and a crash in that moment leaves them with none.
	const q = `
		insert into player_credentials (player_id, kind, hash)
		values ($1, $2, $3)
		on conflict (player_id, kind) do update
			set hash = excluded.hash, updated_at = now()`

	if _, err := r.q.Exec(ctx, q, playerID, string(kind), hash); err != nil {
		return fmt.Errorf("store %s credential of player %s: %w", kind, playerID, err)
	}
	return nil
}

func (r credentialRepo) ForPlayer(ctx context.Context, playerID uuid.UUID, kind domain.CredentialKind) (domain.Credential, error) {
	const q = `
		select player_id, kind, hash, updated_at
		from player_credentials
		where player_id = $1 and kind = $2`

	var (
		c        domain.Credential
		kindText string
	)
	err := r.q.QueryRow(ctx, q, playerID, string(kind)).
		Scan(&c.PlayerID, &kindText, &c.Hash, &c.UpdatedAt)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.Credential{}, fmt.Errorf("%s credential of player %s: %w", kind, playerID, domain.ErrNotFound)
	case err != nil:
		return domain.Credential{}, fmt.Errorf("load %s credential of player %s: %w", kind, playerID, err)
	}
	c.Kind = domain.CredentialKind(kindText)
	return c, nil
}
