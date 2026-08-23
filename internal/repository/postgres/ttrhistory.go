package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

type ttrHistoryRepo struct{ q queryer }

func (r ttrHistoryRepo) Append(ctx context.Context, changes []domain.TTRChange) error {
	const q = `
		insert into ttr_history (player_id, match_id, ttr_before, ttr_after)
		values ($1, $2, $3, $4)`

	for _, c := range changes {
		if _, err := r.q.Exec(ctx, q, c.PlayerID, c.MatchID, c.TTRBefore, c.TTRAfter); err != nil {
			return fmt.Errorf("write ttr history for player %s, match %s: %w",
				c.PlayerID, c.MatchID, err)
		}
	}
	return nil
}

func (r ttrHistoryRepo) ForPlayer(ctx context.Context, playerID uuid.UUID, limit int) ([]domain.TTRChange, error) {
	const q = `
		select id, player_id, match_id, ttr_before, ttr_after, created_at
		from ttr_history
		where player_id = $1
		order by created_at desc
		limit $2`

	rows, err := r.q.Query(ctx, q, playerID, limit)
	if err != nil {
		return nil, fmt.Errorf("load ttr history of player %s: %w", playerID, err)
	}
	defer rows.Close()

	var changes []domain.TTRChange
	for rows.Next() {
		var c domain.TTRChange
		if err := rows.Scan(&c.ID, &c.PlayerID, &c.MatchID, &c.TTRBefore, &c.TTRAfter, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("read ttr entry: %w", err)
		}
		changes = append(changes, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read ttr history: %w", err)
	}
	return changes, nil
}

func (r ttrHistoryRepo) ForMatch(ctx context.Context, matchID uuid.UUID) ([]domain.TTRChange, error) {
	const q = `
		select id, player_id, match_id, ttr_before, ttr_after, created_at
		from ttr_history
		where match_id = $1
		order by created_at`

	rows, err := r.q.Query(ctx, q, matchID)
	if err != nil {
		return nil, fmt.Errorf("load ttr history of match %s: %w", matchID, err)
	}
	defer rows.Close()

	var changes []domain.TTRChange
	for rows.Next() {
		var c domain.TTRChange
		if err := rows.Scan(&c.ID, &c.PlayerID, &c.MatchID, &c.TTRBefore, &c.TTRAfter, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("read ttr entry: %w", err)
		}
		changes = append(changes, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read ttr history of match %s: %w", matchID, err)
	}
	return changes, nil
}
