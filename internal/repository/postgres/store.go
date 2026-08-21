// Package postgres implementiert die Repository-Schnittstellen auf Postgres.
//
// Das ist die einzige Stelle im Projekt, an der SQL steht (Invariante 5).
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

// queryer ist die gemeinsame Teilmenge von *pgxpool.Pool und pgx.Tx. Die
// Repositories arbeiten nur dagegen und wissen dadurch nicht, ob sie gerade
// innerhalb einer Transaktion laufen.
type queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Store ist die Postgres-Implementierung von repository.Store.
type Store struct {
	// pool ist nil, wenn dieser Store an eine laufende Transaktion gebunden ist.
	pool *pgxpool.Pool
	q    queryer
}

var _ repository.Store = (*Store)(nil)

// Open baut einen Verbindungspool auf. Die DSN kommt ausschliesslich aus der
// Umgebung (Invariante 2); dieses Package kennt keine Default-Hosts.
func Open(ctx context.Context, dsn string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("datenbank-dsn parsen: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("verbindungspool aufbauen: %w", err)
	}

	return &Store{pool: pool, q: pool}, nil
}

// Close gibt den Verbindungspool frei.
func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

// Ping prueft die Erreichbarkeit der Datenbank. Ziel ist /readyz.
func (s *Store) Ping(ctx context.Context) error {
	if s.pool == nil {
		// Innerhalb einer Transaktion ist die Verbindung per Definition da.
		return nil
	}
	if err := s.pool.Ping(ctx); err != nil {
		return fmt.Errorf("datenbank nicht erreichbar: %w", err)
	}
	return nil
}

// InTx fuehrt fn in einer Transaktion aus. Ist dieser Store bereits an eine
// Transaktion gebunden, laeuft fn in derselben Transaktion weiter.
func (s *Store) InTx(ctx context.Context, fn func(repository.Store) error) error {
	if s.pool == nil {
		return fn(s)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("transaktion beginnen: %w", err)
	}

	if err := fn(&Store{q: tx}); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			return errors.Join(err, fmt.Errorf("rollback: %w", rbErr))
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("transaktion abschliessen: %w", err)
	}
	return nil
}

// Players liefert das Spieler-Repository.
func (s *Store) Players() repository.PlayerRepository { return playerRepo{s.q} }

// Identities liefert das Identitaeten-Repository.
func (s *Store) Identities() repository.IdentityRepository { return identityRepo{s.q} }

// Matches liefert das Match-Repository.
func (s *Store) Matches() repository.MatchRepository { return matchRepo{s.q} }

// TTRHistory liefert das Historien-Repository.
func (s *Store) TTRHistory() repository.TTRHistoryRepository { return ttrHistoryRepo{s.q} }
