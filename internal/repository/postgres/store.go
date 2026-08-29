// Package postgres implements the repository interfaces on Postgres.
//
// This is the only place in the project that contains SQL (invariant 5).
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stuttgart-things/schmetterpause/internal/repository"
)

// queryer is the subset shared by *pgxpool.Pool and pgx.Tx. The repositories
// work against this alone and therefore do not know whether they are
// currently running inside a transaction.
type queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Store is the Postgres implementation of repository.Store.
type Store struct {
	// pool is nil when this Store is bound to a running transaction.
	pool *pgxpool.Pool
	q    queryer
}

var _ repository.Store = (*Store)(nil)

// Open builds a connection pool. The DSN comes exclusively from the
// environment (invariant 2); this package knows no default hosts.
func Open(ctx context.Context, dsn string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database dsn: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("build connection pool: %w", err)
	}

	return &Store{pool: pool, q: pool}, nil
}

// Close releases the connection pool.
func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

// Ping checks that the database is reachable. Its consumer is /readyz.
func (s *Store) Ping(ctx context.Context) error {
	if s.pool == nil {
		// Inside a transaction the connection exists by definition.
		return nil
	}
	if err := s.pool.Ping(ctx); err != nil {
		return fmt.Errorf("database unreachable: %w", err)
	}
	return nil
}

// InTx runs fn inside a transaction. If this Store is already bound to a
// transaction, fn continues in that same transaction.
func (s *Store) InTx(ctx context.Context, fn func(repository.Store) error) error {
	if s.pool == nil {
		return fn(s)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	if err := fn(&Store{q: tx}); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			return errors.Join(err, fmt.Errorf("rollback: %w", rbErr))
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// Players returns the player repository.
func (s *Store) Players() repository.PlayerRepository { return playerRepo{s.q} }

// Identities returns the identity repository.
func (s *Store) Identities() repository.IdentityRepository { return identityRepo{s.q} }

// Credentials returns the credential repository.
func (s *Store) Credentials() repository.CredentialRepository { return credentialRepo{s.q} }

// KioskGrants returns the kiosk grant repository.
func (s *Store) KioskGrants() repository.KioskGrantRepository { return kioskGrantRepo{s.q} }

// Matches returns the match repository.
func (s *Store) Matches() repository.MatchRepository { return matchRepo{s.q} }

// TTRHistory returns the rating history repository.
func (s *Store) TTRHistory() repository.TTRHistoryRepository { return ttrHistoryRepo{s.q} }

// uniqueViolation is the SQLSTATE Postgres reports for a broken unique
// constraint. Kept here so no caller has to know it.
const uniqueViolation = "23505"

// isUniqueViolation reports whether err came from a unique constraint.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation
}
