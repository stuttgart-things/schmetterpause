package postgres

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql-Treiber fuer goose
	"github.com/pressly/goose/v3"

	"github.com/stuttgart-things/schmetterpause/db"
)

// migrationsDir ist der Pfad innerhalb der eingebetteten FS aus db.Migrations.
const migrationsDir = "migrations"

// Migrate wendet alle ausstehenden Migrations an. Die Migrations sind ins
// Binary eingebettet, das Image bringt sie also selbst mit.
func Migrate(ctx context.Context, dsn string) error {
	return withGoose(ctx, dsn, func(ctx context.Context, sqlDB *sql.DB) error {
		return goose.UpContext(ctx, sqlDB, migrationsDir)
	})
}

// MigrateDown nimmt genau eine Migration zurueck. Nur fuer die lokale
// Entwicklung gedacht: Invariante 8 verlangt vorwaertsgerichtete, additive
// Migrations im Betrieb.
func MigrateDown(ctx context.Context, dsn string) error {
	return withGoose(ctx, dsn, func(ctx context.Context, sqlDB *sql.DB) error {
		return goose.DownContext(ctx, sqlDB, migrationsDir)
	})
}

// MigrationStatus schreibt den Stand der Migrations nach stdout.
func MigrationStatus(ctx context.Context, dsn string) error {
	return withGoose(ctx, dsn, func(ctx context.Context, sqlDB *sql.DB) error {
		return goose.StatusContext(ctx, sqlDB, migrationsDir)
	})
}

func withGoose(ctx context.Context, dsn string, fn func(context.Context, *sql.DB) error) error {
	goose.SetBaseFS(db.Migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("goose-dialekt setzen: %w", err)
	}

	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("migrationsverbindung oeffnen: %w", err)
	}
	defer func() { _ = sqlDB.Close() }()

	if err := fn(ctx, sqlDB); err != nil {
		return fmt.Errorf("migrations anwenden: %w", err)
	}
	return nil
}
