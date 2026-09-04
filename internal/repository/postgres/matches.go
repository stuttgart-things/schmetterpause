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
	reported_by, played_at, confirmed_at, entered_via, tournament_id,
	tournament_round`

func (r matchRepo) Create(ctx context.Context, m domain.Match) (domain.Match, error) {
	const insertMatch = `
		insert into matches (home_id, away_id, best_of, points_to_win, status,
		                     reported_by, played_at, entered_via, tournament_id,
		                     tournament_round)
		values ($1, $2, $3, $4, $5, $6, coalesce($7, now()), $8, $9, $10)
		returning ` + matchColumns

	var playedAt *time.Time
	if !m.PlayedAt.IsZero() {
		playedAt = &m.PlayedAt
	}
	status := m.Status
	if status == "" {
		status = domain.MatchPending
	}
	// An unset EnteredVia is a player's own entry, matching the column
	// default. A caller that forgets it gets the truthful answer for the
	// path almost every row takes, not a constraint violation.
	via := m.EnteredVia
	if via == "" {
		via = domain.EnteredViaPlayer
	}

	created, err := scanMatch(r.q.QueryRow(ctx, insertMatch,
		m.HomeID, m.AwayID, m.BestOf, m.PointsToWin, string(status), m.ReportedBy,
		playedAt, string(via), m.TournamentID, m.TournamentRound))
	if err != nil {
		return domain.Match{}, fmt.Errorf("create match: %w", err)
	}

	const insertSet = `
		insert into match_sets (match_id, set_no, home_points, away_points)
		values ($1, $2, $3, $4)`

	for _, s := range m.Sets {
		if _, err := r.q.Exec(ctx, insertSet, created.ID, s.SetNo, s.HomePoints, s.AwayPoints); err != nil {
			return domain.Match{}, fmt.Errorf("create set %d of match %s: %w", s.SetNo, created.ID, err)
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
		return domain.Match{}, fmt.Errorf("load match %s: %w", id, err)
	}

	sets, err := r.setsFor(ctx, []uuid.UUID{id})
	if err != nil {
		return domain.Match{}, err
	}
	m.Sets = sets[id]
	return m, nil
}

func (r matchRepo) PendingFor(ctx context.Context, playerID uuid.UUID) ([]domain.Match, error) {
	// What needs confirming is what somebody else recorded. A contested
	// match is in here too, from both sides: it is waiting on somebody to
	// say what the result really was, and if it only appeared in the answer
	// to the dispute it could be lost to a page reload.
	const q = `
		select ` + matchColumns + `
		from matches
		where (home_id = $1 or away_id = $1)
		  and (
		        (status = 'pending' and reported_by <> $1)
		     or status = 'disputed'
		  )
		order by played_at desc`

	return r.list(ctx, q, playerID)
}

func (r matchRepo) PendingCountFor(ctx context.Context, playerID uuid.UUID) (int, error) {
	// Same condition as PendingFor, counted rather than fetched.
	const q = `
		select count(*)
		from matches
		where (home_id = $1 or away_id = $1)
		  and (
		        (status = 'pending' and reported_by <> $1)
		     or status = 'disputed'
		  )`

	var n int
	if err := r.q.QueryRow(ctx, q, playerID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count what waits on player %s: %w", playerID, err)
	}
	return n, nil
}

func (r matchRepo) WaitingOnOpponentFor(ctx context.Context, playerID uuid.UUID) ([]domain.Match, error) {
	// The exact complement of the pending half of PendingFor: same status,
	// the other side of the reported_by test. Disputed matches are NOT in
	// here — those already appear in PendingFor for both participants,
	// because either of them may put the result right, and listing one match
	// in both sections would read as two.
	//
	// Ordered oldest first, the opposite of PendingFor. This list is read to
	// find what is stuck, and what is stuck is at the top.
	const q = `
		select ` + matchColumns + `
		from matches
		where (home_id = $1 or away_id = $1)
		  and status = 'pending'
		  and reported_by = $1
		order by played_at asc`

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

func (r matchRepo) Recent(ctx context.Context, limit int) ([]domain.Match, error) {
	// Ordered by played_at rather than confirmed_at: this is a list of what
	// happened at the table, and a match still waiting for its opponent has
	// no confirmation time to sort by at all.
	//
	// No index for it. matches_status_idx covers the filtered reads; an
	// unfiltered sort over a few thousand rows costs nothing, and an index
	// that earns its keep only at a volume this application will not reach
	// is a migration with no benefit behind it.
	const q = `
		select ` + matchColumns + `
		from matches
		order by played_at desc
		limit $1`

	return r.list(ctx, q, limit)
}

// Delete removes a confirmed match. The schema cascades match_sets and
// ttr_history, so one statement takes the whole thing with it.
//
// The status is in the condition rather than checked first: two kiosks
// pressing "zurücknehmen" on the same result would otherwise both read
// "confirmed" and both proceed.
func (r matchRepo) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `delete from matches where id = $1 and status = 'confirmed'`

	tag, err := r.q.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete match %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r matchRepo) SetStatus(ctx context.Context, id uuid.UUID, status domain.MatchStatus, confirmedAt *time.Time) error {
	const q = `update matches set status = $2, confirmed_at = $3 where id = $1`

	tag, err := r.q.Exec(ctx, q, id, string(status), confirmedAt)
	if err != nil {
		return fmt.Errorf("set status of match %s to %s: %w", id, status, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("match %s: %w", id, domain.ErrNotFound)
	}
	return nil
}

func (r matchRepo) ReplaceResult(ctx context.Context, id uuid.UUID, corrected domain.Match) error {
	// The status is part of the where clause, not of a check before it: two
	// players correcting the same match at the same moment would otherwise
	// both pass the check and the second would overwrite the first.
	const updateMatch = `
		update matches
		set best_of = $2, points_to_win = $3, reported_by = $4,
		    status = 'pending', confirmed_at = null
		where id = $1 and status = 'disputed'`

	tag, err := r.q.Exec(ctx, updateMatch, id,
		corrected.BestOf, corrected.PointsToWin, corrected.ReportedBy)
	if err != nil {
		return fmt.Errorf("correct match %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		// Nothing was updated, so either the match is gone or it was not
		// contested. Both are worth telling apart to the caller.
		if _, err := r.ByID(ctx, id); err != nil {
			return err
		}
		return fmt.Errorf("match %s is not contested: %w", id, domain.ErrConflict)
	}

	const deleteSets = `delete from match_sets where match_id = $1`
	if _, err := r.q.Exec(ctx, deleteSets, id); err != nil {
		return fmt.Errorf("clear the sets of match %s: %w", id, err)
	}

	const insertSet = `
		insert into match_sets (match_id, set_no, home_points, away_points)
		values ($1, $2, $3, $4)`

	for _, s := range corrected.Sets {
		if _, err := r.q.Exec(ctx, insertSet, id, s.SetNo, s.HomePoints, s.AwayPoints); err != nil {
			return fmt.Errorf("write set %d of match %s: %w", s.SetNo, id, err)
		}
	}
	return nil
}

func (r matchRepo) list(ctx context.Context, query string, args ...any) ([]domain.Match, error) {
	rows, err := r.q.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("load matches: %w", err)
	}
	defer rows.Close()

	var (
		matches []domain.Match
		ids     []uuid.UUID
	)
	for rows.Next() {
		m, err := scanMatch(rows)
		if err != nil {
			return nil, fmt.Errorf("read match: %w", err)
		}
		matches = append(matches, m)
		ids = append(ids, m.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read matches: %w", err)
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

// setsFor loads the sets for several matches in a single query.
func (r matchRepo) setsFor(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]domain.MatchSet, error) {
	const q = `
		select match_id, set_no, home_points, away_points
		from match_sets
		where match_id = any($1)
		order by match_id, set_no`

	rows, err := r.q.Query(ctx, q, ids)
	if err != nil {
		return nil, fmt.Errorf("load sets: %w", err)
	}
	defer rows.Close()

	sets := make(map[uuid.UUID][]domain.MatchSet, len(ids))
	for rows.Next() {
		var (
			matchID uuid.UUID
			s       domain.MatchSet
		)
		if err := rows.Scan(&matchID, &s.SetNo, &s.HomePoints, &s.AwayPoints); err != nil {
			return nil, fmt.Errorf("read set: %w", err)
		}
		sets[matchID] = append(sets[matchID], s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read sets: %w", err)
	}
	return sets, nil
}

// scanMatch reads one match row in the order given by matchColumns.
func scanMatch(row pgx.Row) (domain.Match, error) {
	var (
		m      domain.Match
		status string
		via    string
	)
	err := row.Scan(&m.ID, &m.HomeID, &m.AwayID, &m.BestOf, &m.PointsToWin,
		&status, &m.ReportedBy, &m.PlayedAt, &m.ConfirmedAt, &via, &m.TournamentID,
		&m.TournamentRound)
	if err != nil {
		return domain.Match{}, err
	}
	m.Status = domain.MatchStatus(status)
	m.EnteredVia = domain.EnteredVia(via)
	return m, nil
}
