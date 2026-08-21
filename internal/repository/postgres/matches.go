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

type matchRepo struct{ q queryer }

const matchColumns = `id, home_id, away_id, best_of, points_to_win, status,
	reported_by, played_at, confirmed_at`

func (r matchRepo) Create(ctx context.Context, m domain.Match) (domain.Match, error) {
	const insertMatch = `
		insert into matches (home_id, away_id, best_of, points_to_win, status, reported_by, played_at)
		values ($1, $2, $3, $4, $5, $6, coalesce($7, now()))
		returning ` + matchColumns

	var playedAt *time.Time
	if !m.PlayedAt.IsZero() {
		playedAt = &m.PlayedAt
	}
	status := m.Status
	if status == "" {
		status = domain.MatchPending
	}

	created, err := scanMatch(r.q.QueryRow(ctx, insertMatch,
		m.HomeID, m.AwayID, m.BestOf, m.PointsToWin, string(status), m.ReportedBy, playedAt))
	if err != nil {
		return domain.Match{}, fmt.Errorf("match anlegen: %w", err)
	}

	const insertSet = `
		insert into match_sets (match_id, set_no, home_points, away_points)
		values ($1, $2, $3, $4)`

	for _, s := range m.Sets {
		if _, err := r.q.Exec(ctx, insertSet, created.ID, s.SetNo, s.HomePoints, s.AwayPoints); err != nil {
			return domain.Match{}, fmt.Errorf("satz %d von match %s anlegen: %w", s.SetNo, created.ID, err)
		}
	}
	created.Sets = m.Sets
	return created, nil
}

func (r matchRepo) ByID(ctx context.Context, id uuid.UUID) (domain.Match, error) {
	const q = `select ` + matchColumns + ` from matches where id = $1`

	m, err := scanMatch(r.q.QueryRow(ctx, q, id))
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.Match{}, fmt.Errorf("match %s: %w", id, domain.ErrNotFound)
	case err != nil:
		return domain.Match{}, fmt.Errorf("match %s laden: %w", id, err)
	}

	sets, err := r.setsFor(ctx, []uuid.UUID{id})
	if err != nil {
		return domain.Match{}, err
	}
	m.Sets = sets[id]
	return m, nil
}

func (r matchRepo) PendingFor(ctx context.Context, playerID uuid.UUID) ([]domain.Match, error) {
	// Zu bestaetigen ist, was jemand anderes eingetragen hat.
	const q = `
		select ` + matchColumns + `
		from matches
		where status = 'pending'
		  and (home_id = $1 or away_id = $1)
		  and reported_by <> $1
		order by played_at desc`

	return r.list(ctx, q, playerID)
}

func (r matchRepo) RecentFor(ctx context.Context, playerID uuid.UUID, limit int) ([]domain.Match, error) {
	const q = `
		select ` + matchColumns + `
		from matches
		where home_id = $1 or away_id = $1
		order by played_at desc
		limit $2`

	return r.list(ctx, q, playerID, limit)
}

func (r matchRepo) SetStatus(ctx context.Context, id uuid.UUID, status domain.MatchStatus, confirmedAt *time.Time) error {
	const q = `update matches set status = $2, confirmed_at = $3 where id = $1`

	tag, err := r.q.Exec(ctx, q, id, string(status), confirmedAt)
	if err != nil {
		return fmt.Errorf("status von match %s auf %s setzen: %w", id, status, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("match %s: %w", id, domain.ErrNotFound)
	}
	return nil
}

func (r matchRepo) list(ctx context.Context, query string, args ...any) ([]domain.Match, error) {
	rows, err := r.q.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("matches laden: %w", err)
	}
	defer rows.Close()

	var (
		matches []domain.Match
		ids     []uuid.UUID
	)
	for rows.Next() {
		m, err := scanMatch(rows)
		if err != nil {
			return nil, fmt.Errorf("match lesen: %w", err)
		}
		matches = append(matches, m)
		ids = append(ids, m.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("matches lesen: %w", err)
	}
	if len(ids) == 0 {
		return matches, nil
	}

	sets, err := r.setsFor(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range matches {
		matches[i].Sets = sets[matches[i].ID]
	}
	return matches, nil
}

// setsFor laedt die Saetze zu mehreren Matches in einer Abfrage.
func (r matchRepo) setsFor(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]domain.MatchSet, error) {
	const q = `
		select match_id, set_no, home_points, away_points
		from match_sets
		where match_id = any($1)
		order by match_id, set_no`

	rows, err := r.q.Query(ctx, q, ids)
	if err != nil {
		return nil, fmt.Errorf("saetze laden: %w", err)
	}
	defer rows.Close()

	sets := make(map[uuid.UUID][]domain.MatchSet, len(ids))
	for rows.Next() {
		var (
			matchID uuid.UUID
			s       domain.MatchSet
		)
		if err := rows.Scan(&matchID, &s.SetNo, &s.HomePoints, &s.AwayPoints); err != nil {
			return nil, fmt.Errorf("satz lesen: %w", err)
		}
		sets[matchID] = append(sets[matchID], s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("saetze lesen: %w", err)
	}
	return sets, nil
}

// scanMatch liest eine Match-Zeile in der Reihenfolge von matchColumns.
func scanMatch(row pgx.Row) (domain.Match, error) {
	var (
		m      domain.Match
		status string
	)
	err := row.Scan(&m.ID, &m.HomeID, &m.AwayID, &m.BestOf, &m.PointsToWin,
		&status, &m.ReportedBy, &m.PlayedAt, &m.ConfirmedAt)
	if err != nil {
		return domain.Match{}, err
	}
	m.Status = domain.MatchStatus(status)
	return m, nil
}
