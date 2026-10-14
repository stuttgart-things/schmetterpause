package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"

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

func (r checkinRepo) Lock(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.Checkin, error) {
	// Ordered, so two transactions locking the same pair take the rows in the
	// same order and wait for each other instead of deadlocking.
	const lock = `
		select id, player_id, half, method, created_at
		from checkins
		where id = any($1)
		order by id
		for update`

	rows, err := r.q.Query(ctx, lock, ids)
	if err != nil {
		return nil, fmt.Errorf("lock check-ins: %w", err)
	}
	defer rows.Close()

	out := make(map[uuid.UUID]domain.Checkin, len(ids))
	for rows.Next() {
		var (
			c            domain.Checkin
			half, method string
		)
		if err := rows.Scan(&c.ID, &c.PlayerID, &half, &method, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("read check-in: %w", err)
		}
		c.Half, c.Method = domain.Half(half), domain.CheckinMethod(method)
		out[c.ID] = c
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read check-ins: %w", err)
	}

	// A second statement, not a subquery in the first. Under read committed a
	// statement that waited for the lock keeps the snapshot it started with,
	// and would not see the match the other transaction just committed.
	const used = `
		select home_checkin_id from matches where home_checkin_id = any($1)
		union all
		select away_checkin_id from matches where away_checkin_id = any($1)`

	urows, err := r.q.Query(ctx, used, ids)
	if err != nil {
		return nil, fmt.Errorf("look up used check-ins: %w", err)
	}
	defer urows.Close()
	for urows.Next() {
		var id uuid.UUID
		if err := urows.Scan(&id); err != nil {
			return nil, fmt.Errorf("read used check-in: %w", err)
		}
		if c, ok := out[id]; ok {
			c.Used = true
			out[id] = c
		}
	}
	if err := urows.Err(); err != nil {
		return nil, fmt.Errorf("read used check-ins: %w", err)
	}
	return out, nil
}
