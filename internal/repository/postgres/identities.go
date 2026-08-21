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
	// Ein erneuter Link auf denselben Spieler ist ein No-op; ein Link auf einen
	// anderen Spieler bleibt bewusst ein Konflikt: das Zusammenfuehren zweier
	// Spieler ist laut docs/adr/0003 eine eigene, bewusste Operation.
	const q = `
		insert into identities (provider, subject, player_id)
		values ($1, $2, $3)
		on conflict (provider, subject) do nothing`

	tag, err := r.q.Exec(ctx, q, string(provider), subject, playerID)
	if err != nil {
		return fmt.Errorf("identitaet %s/%s verknuepfen: %w", provider, subject, err)
	}
	if tag.RowsAffected() == 0 {
		existing, err := r.PlayerBy(ctx, provider, subject)
		if err != nil {
			return fmt.Errorf("bestehende identitaet %s/%s pruefen: %w", provider, subject, err)
		}
		if existing.ID != playerID {
			return fmt.Errorf(
				"identitaet %s/%s gehoert bereits zu spieler %s", provider, subject, existing.ID)
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
		return domain.Player{}, fmt.Errorf("identitaet %s/%s: %w", provider, subject, domain.ErrNotFound)
	case err != nil:
		return domain.Player{}, fmt.Errorf("spieler zu identitaet %s/%s laden: %w", provider, subject, err)
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
		return nil, fmt.Errorf("identitaeten von spieler %s laden: %w", playerID, err)
	}
	defer rows.Close()

	var identities []domain.Identity
	for rows.Next() {
		var (
			id       domain.Identity
			provider string
		)
		if err := rows.Scan(&provider, &id.Subject, &id.PlayerID, &id.CreatedAt); err != nil {
			return nil, fmt.Errorf("identitaet lesen: %w", err)
		}
		id.Provider = domain.Provider(provider)
		identities = append(identities, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("identitaeten lesen: %w", err)
	}
	return identities, nil
}
