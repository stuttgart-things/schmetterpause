package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

type playerRepo struct{ q queryer }

func (r playerRepo) Create(ctx context.Context, displayName string, initialTTR int) (domain.Player, error) {
	const q = `
		insert into players (display_name, ttr)
		values ($1, $2)
		returning id, display_name, ttr, created_at`

	var p domain.Player
	err := r.q.QueryRow(ctx, q, displayName, initialTTR).
		Scan(&p.ID, &p.DisplayName, &p.TTR, &p.CreatedAt)
	if err != nil {
		return domain.Player{}, fmt.Errorf("create player %q: %w", displayName, err)
	}
	return p, nil
}

func (r playerRepo) ByID(ctx context.Context, id uuid.UUID) (domain.Player, error) {
	const q = `select id, display_name, ttr, created_at from players where id = $1`

	var p domain.Player
	err := r.q.QueryRow(ctx, q, id).Scan(&p.ID, &p.DisplayName, &p.TTR, &p.CreatedAt)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.Player{}, fmt.Errorf("player %s: %w", id, domain.ErrNotFound)
	case err != nil:
		return domain.Player{}, fmt.Errorf("load player %s: %w", id, err)
	}
	return p, nil
}

func (r playerRepo) List(ctx context.Context) ([]domain.Player, error) {
	const q = `
		select id, display_name, ttr, created_at
		from players
		order by ttr desc, lower(display_name)`

	rows, err := r.q.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("load player list: %w", err)
	}
	defer rows.Close()

	var players []domain.Player
	for rows.Next() {
		var p domain.Player
		if err := rows.Scan(&p.ID, &p.DisplayName, &p.TTR, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("read player: %w", err)
		}
		players = append(players, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read player list: %w", err)
	}
	return players, nil
}

func (r playerRepo) Count(ctx context.Context) (int, error) {
	var n int
	if err := r.q.QueryRow(ctx, `select count(*) from players`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count players: %w", err)
	}
	return n, nil
}

func (r playerRepo) UpdateTTR(ctx context.Context, id uuid.UUID, ttr int) error {
	tag, err := r.q.Exec(ctx, `update players set ttr = $2 where id = $1`, id, ttr)
	if err != nil {
		return fmt.Errorf("set ttr of player %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("player %s: %w", id, domain.ErrNotFound)
	}
	return nil
}
