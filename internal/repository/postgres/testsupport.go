package postgres

import (
	"context"
	"fmt"
)

// TruncateAll empties every table. Intended exclusively for integration
// tests — which is why callers reach it only behind a separately configured
// test DSN, never through the normal configuration.
func TruncateAll(ctx context.Context, s *Store) error {
	const q = `truncate ttr_history, match_sets, matches, tournament_players, tournaments,
		player_credentials, kiosk_grants, identities, players cascade`

	if _, err := s.q.Exec(ctx, q); err != nil {
		return fmt.Errorf("truncate tables: %w", err)
	}
	return nil
}
