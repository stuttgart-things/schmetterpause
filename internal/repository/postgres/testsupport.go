package postgres

import (
	"context"
	"fmt"
)

// TruncateAll leert alle Tabellen. Ausschliesslich fuer Integrationstests
// gedacht — deshalb steht der Aufruf unter dem Vorbehalt einer eigens
// gesetzten Test-DSN und nicht unter der normalen Konfiguration.
func TruncateAll(ctx context.Context, s *Store) error {
	const q = `truncate ttr_history, match_sets, matches, identities, players cascade`

	if _, err := s.q.Exec(ctx, q); err != nil {
		return fmt.Errorf("tabellen leeren: %w", err)
	}
	return nil
}
