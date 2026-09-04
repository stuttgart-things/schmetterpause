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

type kioskGrantRepo struct{ q queryer }

const kioskGrantColumns = `id, created_at, last_seen_at, expires_at, user_agent, revoked_at, operator_id`

func (r kioskGrantRepo) Create(
	ctx context.Context, secretHash []byte, expiresAt time.Time, userAgent string,
) (domain.KioskGrant, error) {
	const q = `
		insert into kiosk_grants (secret_hash, expires_at, user_agent)
		values ($1, $2, $3)
		returning ` + kioskGrantColumns

	g, err := scanKioskGrant(r.q.QueryRow(ctx, q, secretHash, expiresAt, userAgent))
	if err != nil {
		return domain.KioskGrant{}, fmt.Errorf("create kiosk grant: %w", err)
	}
	return g, nil
}

func (r kioskGrantRepo) BySecret(ctx context.Context, secretHash []byte) (domain.KioskGrant, error) {
	const q = `select ` + kioskGrantColumns + ` from kiosk_grants where secret_hash = $1`

	g, err := scanKioskGrant(r.q.QueryRow(ctx, q, secretHash))
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// A cookie nothing stands for. The ordinary answer for a machine
		// somebody kept past a revocation, not a failure.
		return domain.KioskGrant{}, fmt.Errorf("kiosk grant: %w", domain.ErrNotFound)
	case err != nil:
		return domain.KioskGrant{}, fmt.Errorf("load kiosk grant: %w", err)
	}
	return g, nil
}

func (r kioskGrantRepo) Touch(ctx context.Context, id uuid.UUID, at time.Time) error {
	const q = `update kiosk_grants set last_seen_at = $2 where id = $1`

	if _, err := r.q.Exec(ctx, q, id, at); err != nil {
		return fmt.Errorf("touch kiosk grant %s: %w", id, err)
	}
	return nil
}

// SetOperator names who is typing at this machine (issue #90).
//
// Writable more than once on purpose: the person at the laptop changes during
// an evening, and the alternative is a machine that has to be unlocked again
// to hand over. Only an active grant can be named, so a revoked machine
// cannot be quietly given an operator and put back to work.
func (r kioskGrantRepo) SetOperator(
	ctx context.Context, id, playerID uuid.UUID, at time.Time,
) error {
	const q = `
		update kiosk_grants
		   set operator_id = $2
		 where id = $1 and revoked_at is null and expires_at > $3`

	tag, err := r.q.Exec(ctx, q, id, playerID, at)
	if err != nil {
		return fmt.Errorf("set operator of kiosk grant %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("set operator of kiosk grant %s: %w", id, domain.ErrNotFound)
	}
	return nil
}

func (r kioskGrantRepo) Revoke(ctx context.Context, id uuid.UUID, at time.Time) error {
	// Only the first revocation counts, so pressing the button twice does not
	// move the timestamp and make it look like it happened later than it did.
	const q = `update kiosk_grants set revoked_at = $2 where id = $1 and revoked_at is null`

	if _, err := r.q.Exec(ctx, q, id, at); err != nil {
		return fmt.Errorf("revoke kiosk grant %s: %w", id, err)
	}
	return nil
}

func (r kioskGrantRepo) RevokeAll(ctx context.Context, at time.Time) (int, error) {
	const q = `
		update kiosk_grants
		   set revoked_at = $1
		 where revoked_at is null and expires_at > $1`

	tag, err := r.q.Exec(ctx, q, at)
	if err != nil {
		return 0, fmt.Errorf("revoke all kiosk grants: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

func (r kioskGrantRepo) Active(ctx context.Context, at time.Time) ([]domain.KioskGrant, error) {
	const q = `
		select ` + kioskGrantColumns + `
		from kiosk_grants
		where revoked_at is null and expires_at > $1
		order by last_seen_at desc`

	rows, err := r.q.Query(ctx, q, at)
	if err != nil {
		return nil, fmt.Errorf("load active kiosk grants: %w", err)
	}
	defer rows.Close()

	var grants []domain.KioskGrant
	for rows.Next() {
		g, err := scanKioskGrant(rows)
		if err != nil {
			return nil, fmt.Errorf("read kiosk grant: %w", err)
		}
		grants = append(grants, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read kiosk grants: %w", err)
	}
	return grants, nil
}

func scanKioskGrant(row pgx.Row) (domain.KioskGrant, error) {
	var g domain.KioskGrant
	err := row.Scan(&g.ID, &g.CreatedAt, &g.LastSeenAt, &g.ExpiresAt, &g.UserAgent,
		&g.RevokedAt, &g.OperatorID)
	if err != nil {
		return domain.KioskGrant{}, err
	}
	return g, nil
}
