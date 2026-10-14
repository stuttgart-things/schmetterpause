package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

type fobWindowRepo struct{ q queryer }

func (r fobWindowRepo) Open(ctx context.Context, playerID uuid.UUID, at, expiresAt time.Time) (domain.FobWindow, error) {
	// The row is replaced only when it has expired or is the same player's.
	// Otherwise the conflict updates nothing, returns nothing, and the window
	// that stands is read back below. One statement, so two players opening
	// at once cannot both win.
	const q = `
		insert into fob_link_window (player_id, opened_at, expires_at)
		values ($1, $2, $3)
		on conflict (only_one) do update
			set player_id = excluded.player_id,
			    opened_at = excluded.opened_at,
			    expires_at = excluded.expires_at
			where fob_link_window.expires_at <= excluded.opened_at
			   or fob_link_window.player_id = excluded.player_id
		returning player_id, opened_at, expires_at`

	var w domain.FobWindow
	err := r.q.QueryRow(ctx, q, playerID, at, expiresAt).Scan(&w.PlayerID, &w.OpenedAt, &w.ExpiresAt)
	switch {
	case err == nil:
		return w, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return domain.FobWindow{}, fmt.Errorf("open fob window for player %s: %w", playerID, err)
	}

	other, err := r.Current(ctx, at)
	if errors.Is(err, domain.ErrNotFound) {
		// Expired between the two statements. Rare enough that asking the
		// player to press again is the honest answer rather than a retry loop.
		return domain.FobWindow{}, fmt.Errorf("open fob window: %w", domain.ErrConflict)
	}
	if err != nil {
		return domain.FobWindow{}, err
	}
	return other, fmt.Errorf("open fob window: %w", domain.ErrConflict)
}

func (r fobWindowRepo) Current(ctx context.Context, at time.Time) (domain.FobWindow, error) {
	const q = `
		select player_id, opened_at, expires_at
		from fob_link_window
		where expires_at > $1`

	var w domain.FobWindow
	err := r.q.QueryRow(ctx, q, at).Scan(&w.PlayerID, &w.OpenedAt, &w.ExpiresAt)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.FobWindow{}, fmt.Errorf("fob window: %w", domain.ErrNotFound)
	case err != nil:
		return domain.FobWindow{}, fmt.Errorf("load fob window: %w", err)
	}
	return w, nil
}

func (r fobWindowRepo) Consume(ctx context.Context, at time.Time) (domain.FobWindow, error) {
	// Delete-returning is the claim: of two concurrent calls, one deletes the
	// row and the other finds nothing to delete.
	const q = `
		delete from fob_link_window
		where expires_at > $1
		returning player_id, opened_at, expires_at`

	var w domain.FobWindow
	err := r.q.QueryRow(ctx, q, at).Scan(&w.PlayerID, &w.OpenedAt, &w.ExpiresAt)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.FobWindow{}, fmt.Errorf("fob window: %w", domain.ErrNotFound)
	case err != nil:
		return domain.FobWindow{}, fmt.Errorf("consume fob window: %w", err)
	}
	return w, nil
}

func (r fobWindowRepo) Close(ctx context.Context, playerID uuid.UUID) error {
	const q = `delete from fob_link_window where player_id = $1`

	if _, err := r.q.Exec(ctx, q, playerID); err != nil {
		return fmt.Errorf("close fob window of player %s: %w", playerID, err)
	}
	return nil
}
