package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/match"
)

type tournamentRepo struct{ q queryer }

const tournamentColumns = `id, name, format, status, best_of, points_to_win, ` +
	`with_final, rated, count_points, created_by, created_at, closed_at`

func (r tournamentRepo) Create(ctx context.Context, t domain.Tournament) (domain.Tournament, error) {
	const insert = `
		insert into tournaments (name, format, status, best_of, points_to_win,
		                         with_final, rated, count_points, created_by)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		returning ` + tournamentColumns

	format := t.Format
	if format == "" {
		format = domain.TournamentRoundRobin
	}
	status := t.Status
	if status == "" {
		status = domain.TournamentOpen
	}
	// A zero mode is a caller that did not say, not a caller asking for a
	// tournament of no sets. The column defaults say the same thing, but an
	// explicit insert never reaches them.
	mode := match.Mode{BestOf: t.BestOf, PointsToWin: t.PointsToWin}
	if mode.BestOf == 0 {
		mode.BestOf = match.DefaultBestOf
	}
	if mode.PointsToWin == 0 {
		mode.PointsToWin = match.PointsToEleven
	}

	created, err := scanTournament(r.q.QueryRow(ctx, insert,
		t.Name, string(format), string(status), mode.BestOf, mode.PointsToWin,
		t.WithFinal, t.Rated, t.CountPoints, t.CreatedBy))
	if err != nil {
		return domain.Tournament{}, fmt.Errorf("create tournament: %w", err)
	}

	// The position is the index, so the draw can be recomputed instead of
	// stored. Writing it explicitly rather than relying on insertion order
	// is what makes that true: a select without an order by is a set.
	const addPlayer = `
		insert into tournament_players (tournament_id, player_id, position)
		values ($1, $2, $3)`

	for i, playerID := range t.Players {
		if _, err := r.q.Exec(ctx, addPlayer, created.ID, playerID, i); err != nil {
			if isUniqueViolation(err) {
				return domain.Tournament{}, fmt.Errorf(
					"player %s is in tournament %s twice: %w",
					playerID, created.ID, domain.ErrConflict)
			}
			return domain.Tournament{}, fmt.Errorf(
				"add player %s to tournament %s: %w", playerID, created.ID, err)
		}
	}
	created.Players = t.Players
	return created, nil
}

func (r tournamentRepo) ByID(ctx context.Context, id uuid.UUID) (domain.Tournament, error) {
	const q = `select ` + tournamentColumns + ` from tournaments where id = $1`

	t, err := scanTournament(r.q.QueryRow(ctx, q, id))
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.Tournament{}, fmt.Errorf("tournament %s: %w", id, domain.ErrNotFound)
	case err != nil:
		return domain.Tournament{}, fmt.Errorf("load tournament %s: %w", id, err)
	}

	fields, err := r.playersFor(ctx, []uuid.UUID{id})
	if err != nil {
		return domain.Tournament{}, err
	}
	t.Players = fields[id]
	return t, nil
}

func (r tournamentRepo) List(ctx context.Context, limit int) ([]domain.Tournament, error) {
	// Open ones first: at the table, the thing still being played is what
	// somebody is looking for, and last week's finished evening is not.
	const q = `
		select ` + tournamentColumns + `
		from tournaments
		order by (status = 'open') desc, created_at desc
		limit $1`

	rows, err := r.q.Query(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("list tournaments: %w", err)
	}
	defer rows.Close()

	var (
		out []domain.Tournament
		ids []uuid.UUID
	)
	for rows.Next() {
		t, err := scanTournament(rows)
		if err != nil {
			return nil, fmt.Errorf("scan tournament: %w", err)
		}
		out = append(out, t)
		ids = append(ids, t.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list tournaments: %w", err)
	}

	// One query for every field rather than one per tournament: a list of
	// ten would otherwise be eleven round trips.
	fields, err := r.playersFor(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Players = fields[out[i].ID]
	}
	return out, nil
}

func (r tournamentRepo) Close(ctx context.Context, id uuid.UUID, at time.Time) error {
	// Idempotent by the where clause rather than by a read first: closing an
	// already closed tournament leaves the original closed_at alone, which
	// is the honest timestamp.
	const q = `
		update tournaments
		set status = 'closed', closed_at = $2
		where id = $1 and status <> 'closed'`

	if _, err := r.q.Exec(ctx, q, id, at); err != nil {
		return fmt.Errorf("close tournament %s: %w", id, err)
	}
	return nil
}

func (r tournamentRepo) Matches(ctx context.Context, id uuid.UUID) ([]domain.Match, error) {
	const q = `
		select ` + matchColumns + `
		from matches
		where tournament_id = $1
		order by played_at asc`

	return matchRepo(r).list(ctx, q, id)
}

// playersFor loads the fields of several tournaments in one query, in draw
// order. The order is the contract: the pairings are recomputed from it.
func (r tournamentRepo) playersFor(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	out := make(map[uuid.UUID][]uuid.UUID, len(ids))
	if len(ids) == 0 {
		return out, nil
	}

	const q = `
		select tournament_id, player_id
		from tournament_players
		where tournament_id = any($1)
		order by tournament_id, position`

	rows, err := r.q.Query(ctx, q, ids)
	if err != nil {
		return nil, fmt.Errorf("load tournament players: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tournamentID, playerID uuid.UUID
		if err := rows.Scan(&tournamentID, &playerID); err != nil {
			return nil, fmt.Errorf("scan tournament player: %w", err)
		}
		out[tournamentID] = append(out[tournamentID], playerID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load tournament players: %w", err)
	}
	return out, nil
}

// scanTournament reads one row in the order given by tournamentColumns.
func scanTournament(row pgx.Row) (domain.Tournament, error) {
	var (
		t      domain.Tournament
		format string
		status string
	)
	err := row.Scan(&t.ID, &t.Name, &format, &status, &t.BestOf, &t.PointsToWin,
		&t.WithFinal, &t.Rated, &t.CountPoints, &t.CreatedBy, &t.CreatedAt, &t.ClosedAt)
	if err != nil {
		return domain.Tournament{}, err
	}
	t.Format = domain.TournamentFormat(format)
	t.Status = domain.TournamentStatus(status)
	return t, nil
}

// DeleteIfEmpty removes a tournament nobody has played in.
//
// The emptiness check is part of the statement rather than a read before it:
// two people pressing delete while somebody enters a result at the table is
// exactly the race this has to lose safely.
func (r tournamentRepo) DeleteIfEmpty(ctx context.Context, id uuid.UUID) (bool, error) {
	const del = `
		delete from tournaments
		where id = $1
		  and not exists (select 1 from matches m where m.tournament_id = tournaments.id)`

	tag, err := r.q.Exec(ctx, del, id)
	if err != nil {
		return false, fmt.Errorf("delete tournament %s: %w", id, err)
	}
	return tag.RowsAffected() > 0, nil
}

// Replace changes the field and the mode of a tournament that holds no
// results, and returns the tournament as it now stands.
//
// The field is rewritten wholesale rather than diffed: position is the draw,
// so "who is in it, in which order" is one value, and patching it a name at a
// time would produce orders nobody chose.
func (r tournamentRepo) Replace(ctx context.Context, t domain.Tournament) (domain.Tournament, error) {
	const update = `
		update tournaments
		set name = $2, format = $3, best_of = $4, points_to_win = $5,
		    with_final = $6, rated = $7, count_points = $8
		where id = $1
		  and not exists (select 1 from matches m where m.tournament_id = tournaments.id)
		returning ` + tournamentColumns

	updated, err := scanTournament(r.q.QueryRow(ctx, update,
		t.ID, t.Name, string(t.Format), t.BestOf, t.PointsToWin, t.WithFinal,
		t.Rated, t.CountPoints))
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// Either it is gone or somebody entered a result while this form was
		// open. Both mean the same thing to the caller: this edit no longer
		// applies.
		return domain.Tournament{}, domain.ErrNotFound
	case err != nil:
		return domain.Tournament{}, fmt.Errorf("replace tournament %s: %w", t.ID, err)
	}

	if _, err := r.q.Exec(ctx,
		`delete from tournament_players where tournament_id = $1`, t.ID); err != nil {
		return domain.Tournament{}, fmt.Errorf("clear field of %s: %w", t.ID, err)
	}
	const addPlayer = `
		insert into tournament_players (tournament_id, player_id, position)
		values ($1, $2, $3)`
	for i, playerID := range t.Players {
		if _, err := r.q.Exec(ctx, addPlayer, t.ID, playerID, i); err != nil {
			return domain.Tournament{}, fmt.Errorf(
				"add player %s to tournament %s: %w", playerID, t.ID, err)
		}
	}
	updated.Players = t.Players
	return updated, nil
}
