package postgres

import (
	"context"
	"fmt"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

type checkinRepo struct{ q queryer }

func (r checkinRepo) Create(ctx context.Context, c domain.Checkin) (domain.Checkin, error) {
	const q = `
		insert into checkins (player_id, half, method, created_at)
		values ($1, $2, $3, $4)
		returning id`

	if err := r.q.QueryRow(ctx, q, c.PlayerID, string(c.Half), string(c.Method), c.CreatedAt).
		Scan(&c.ID); err != nil {
		return domain.Checkin{}, fmt.Errorf("record check-in of player %s: %w", c.PlayerID, err)
	}
	return c, nil
}
