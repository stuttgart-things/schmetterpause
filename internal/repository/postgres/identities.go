package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

type identityRepo struct{ q queryer }

func (r identityRepo) Link(ctx context.Context, provider domain.Provider, subject string, playerID uuid.UUID) error {
	// Linking the same player again is a no-op; linking a different player
	// stays a conflict on purpose: merging two players is a separate,
	// deliberate operation per docs/adr/0003.
	const q = `
		insert into identities (provider, subject, player_id)
		values ($1, $2, $3)
		on conflict (provider, subject) do nothing`

	tag, err := r.q.Exec(ctx, q, string(provider), subject, playerID)
	if err != nil {
		return fmt.Errorf("link identity %s/%s: %w", provider, subject, err)
	}
	if tag.RowsAffected() == 0 {
		existing, err := r.PlayerBy(ctx, provider, subject)
		if err != nil {
			return fmt.Errorf("check existing identity %s/%s: %w", provider, subject, err)
		}
		if existing.ID != playerID {
			return fmt.Errorf(
				"identity %s/%s already belongs to player %s", provider, subject, existing.ID)
		}
	}
	return nil
}

func (r identityRepo) PlayerBy(ctx context.Context, provider domain.Provider, subject string) (domain.Player, error) {
	const q = `
		select p.id, p.display_name, p.ttr, p.created_at
		from identities i
		join players p on p.id = i.player_id
		where i.provider = $1 and i.subject = $2`

	var p domain.Player
	err := r.q.QueryRow(ctx, q, string(provider), subject).
		Scan(&p.ID, &p.DisplayName, &p.TTR, &p.CreatedAt)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.Player{}, fmt.Errorf("identity %s/%s: %w", provider, subject, domain.ErrNotFound)
	case err != nil:
		return domain.Player{}, fmt.Errorf("load player for identity %s/%s: %w", provider, subject, err)
	}
	return p, nil
}

func (r identityRepo) ForPlayer(ctx context.Context, playerID uuid.UUID) ([]domain.Identity, error) {
	const q = `
		select provider, subject, player_id, created_at
		from identities
		where player_id = $1
		order by created_at`

	rows, err := r.q.Query(ctx, q, playerID)
	if err != nil {
		return nil, fmt.Errorf("load identities of player %s: %w", playerID, err)
	}
	defer rows.Close()

	var identities []domain.Identity
	for rows.Next() {
		var (
			id       domain.Identity
			provider string
		)
		if err := rows.Scan(&provider, &id.Subject, &id.PlayerID, &id.CreatedAt); err != nil {
			return nil, fmt.Errorf("read identity: %w", err)
		}
		id.Provider = domain.Provider(provider)
		identities = append(identities, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read identities: %w", err)
	}
	return identities, nil
}
