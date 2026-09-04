package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// TestDatabaseSuffix is what a database has to be called before TruncateAll
// will empty it.
//
// A suffix rather than one fixed name, so a second checkout or a throwaway
// database for one run is not locked out — `schmetterpause_test`,
// `schmetterpause_pr162_test`, anything that says out loud what it is for.
const TestDatabaseSuffix = "_test"

// RequireTestDatabase refuses a DSN that does not name a test database.
//
// The same rule as TruncateAll, applied before anything is opened, because
// the test harness migrates before it truncates — so a wrong DSN reached a
// live database with a schema change even though the truncate that followed
// was refused. A migration is additive by invariant 8 and the blast radius is
// small, but small is not none, and the point of issue #163 is to stop
// guessing about that.
//
// Parsed with pgconn rather than net/url: a DSN may be a URL or the
// keyword/value form, and both reach the same driver.
func RequireTestDatabase(dsn string) error {
	cfg, err := pgconn.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("parse the test database URL: %w", err)
	}
	if !strings.HasSuffix(cfg.Database, TestDatabaseSuffix) {
		return fmt.Errorf(
			"refusing to use the database %q for tests: only a database whose name "+
				"ends in %q is a test database (issue #163)",
			cfg.Database, TestDatabaseSuffix)
	}
	return nil
}

// TruncateAll empties every table. Intended exclusively for integration tests.
//
// It used to say that callers reach it only behind a separately configured
// test DSN, and that this is what makes it safe. That was wrong in the one
// way that mattered: `task test:integration` configured that DSN to the
// database the office plays on, because compose.office.yaml is an overlay on
// the same `db` service — same volume, same port, one database. The safety
// argument and its defeat were eleven lines apart in the same repository
// (issue #163).
//
// So the refusal lives here, where every caller passes, and it asks the
// server rather than reading the DSN. A connection string can be assembled in
// more ways than a check can anticipate; `current_database()` is the name of
// the thing that is about to be emptied, whoever built the string and however
// the environment was set.
func TruncateAll(ctx context.Context, s *Store) error {
	var name string
	if err := s.q.QueryRow(ctx, `select current_database()`).Scan(&name); err != nil {
		return fmt.Errorf("read the database name: %w", err)
	}
	if !strings.HasSuffix(name, TestDatabaseSuffix) {
		return fmt.Errorf(
			"refusing to empty the database %q: only a database whose name ends in %q "+
				"is a test database - point SP_TEST_DATABASE_URL at one (issue #163)",
			name, TestDatabaseSuffix)
	}

	const q = `truncate ttr_history, match_sets, matches, tournament_players, tournaments,
		player_credentials, kiosk_grants, identities, players cascade`

	if _, err := s.q.Exec(ctx, q); err != nil {
		return fmt.Errorf("truncate tables: %w", err)
	}
	return nil
}
