package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/stuttgart-things/schmetterpause/internal/avatar"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

type avatarRepo struct{ q queryer }

func (r avatarRepo) Get(ctx context.Context, playerID uuid.UUID) (avatar.Avatar, error) {
	const q = `
		select fur, headdress, handle, blade, bling
		from player_avatars
		where player_id = $1`

	var a avatar.Avatar
	err := r.q.QueryRow(ctx, q, playerID).Scan(&a.Fur, &a.Headdress, &a.Handle, &a.Blade, &a.Bling)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return avatar.Avatar{}, fmt.Errorf("avatar of player %s: %w", playerID, domain.ErrNotFound)
	case err != nil:
		return avatar.Avatar{}, fmt.Errorf("load avatar of player %s: %w", playerID, err)
	}
	return a, nil
}

func (r avatarRepo) Put(ctx context.Context, playerID uuid.UUID, a avatar.Avatar) error {
	const q = `
		insert into player_avatars (player_id, fur, headdress, handle, blade, bling, updated_at)
		values ($1, $2, $3, $4, $5, $6, now())
		on conflict (player_id) do update
			set fur = excluded.fur,
			    headdress = excluded.headdress,
			    handle = excluded.handle,
			    blade = excluded.blade,
			    bling = excluded.bling,
			    updated_at = excluded.updated_at`

	_, err := r.q.Exec(ctx, q, playerID, a.Fur, a.Headdress, a.Handle, a.Blade, a.Bling)
	switch {
	case isForeignKeyViolation(err):
		// Nobody to dress: the player is gone, or never was.
		return fmt.Errorf("avatar of player %s: %w", playerID, domain.ErrNotFound)
	case err != nil:
		return fmt.Errorf("store avatar of player %s: %w", playerID, err)
	}
	return nil
}

func (r avatarRepo) Delete(ctx context.Context, playerID uuid.UUID) error {
	if _, err := r.q.Exec(ctx, `delete from player_avatars where player_id = $1`, playerID); err != nil {
		return fmt.Errorf("delete avatar of player %s: %w", playerID, err)
	}
	return nil
}
