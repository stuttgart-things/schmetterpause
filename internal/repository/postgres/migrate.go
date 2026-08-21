package postgres

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver for goose
	"github.com/pressly/goose/v3"

	"github.com/stuttgart-things/schmetterpause/db"
)

// migrationsDir is the path inside the embedded FS from db.Migrations.
const migrationsDir = "migrations"

// Migrate applies all pending migrations. The migrations are embedded in the
// binary, so the image brings them along itself.
func Migrate(ctx context.Context, dsn string) error {
	return withGoose(ctx, dsn, func(ctx context.Context, sqlDB *sql.DB) error {
		return goose.UpContext(ctx, sqlDB, migrationsDir)
	})
}

// MigrateDown rolls back exactly one migration. Intended for local
// development only: invariant 8 calls for forward-only, additive migrations
// in operation.
func MigrateDown(ctx context.Context, dsn string) error {
	return withGoose(ctx, dsn, func(ctx context.Context, sqlDB *sql.DB) error {
		return goose.DownContext(ctx, sqlDB, migrationsDir)
	})
}

// MigrationStatus writes the migration state to stdout.
func MigrationStatus(ctx context.Context, dsn string) error {
	return withGoose(ctx, dsn, func(ctx context.Context, sqlDB *sql.DB) error {
		return goose.StatusContext(ctx, sqlDB, migrationsDir)
	})
}

func withGoose(ctx context.Context, dsn string, fn func(context.Context, *sql.DB) error) error {
	goose.SetBaseFS(db.Migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open migration connection: %w", err)
	}
	defer func() { _ = sqlDB.Close() }()

	if err := fn(ctx, sqlDB); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
