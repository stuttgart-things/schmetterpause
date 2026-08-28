package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

type playerRepo struct{ q queryer }

func (r playerRepo) Create(ctx context.Context, displayName string, initialTTR int) (domain.Player, error) {
	const q = `
		insert into players (display_name, ttr)
		values ($1, $2)
		returning id, display_name, ttr, created_at, is_admin`

	var p domain.Player
	err := r.q.QueryRow(ctx, q, displayName, initialTTR).
		Scan(&p.ID, &p.DisplayName, &p.TTR, &p.CreatedAt, &p.IsAdmin)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Player{}, fmt.Errorf("create player %q: %w", displayName, domain.ErrConflict)
		}
		return domain.Player{}, fmt.Errorf("create player %q: %w", displayName, err)
	}
	return p, nil
}

func (r playerRepo) ByID(ctx context.Context, id uuid.UUID) (domain.Player, error) {
	const q = `select id, display_name, ttr, created_at, is_admin from players where id = $1`

	var p domain.Player
	err := r.q.QueryRow(ctx, q, id).Scan(&p.ID, &p.DisplayName, &p.TTR, &p.CreatedAt, &p.IsAdmin)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.Player{}, fmt.Errorf("player %s: %w", id, domain.ErrNotFound)
	case err != nil:
		return domain.Player{}, fmt.Errorf("load player %s: %w", id, err)
	}
	return p, nil
}

func (r playerRepo) List(ctx context.Context) ([]domain.Player, error) {
	const q = `
		select id, display_name, ttr, created_at, is_admin
		from players
		order by ttr desc, lower(display_name)`

	rows, err := r.q.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("load player list: %w", err)
	}
	defer rows.Close()

	var players []domain.Player
	for rows.Next() {
		var p domain.Player
		if err := rows.Scan(&p.ID, &p.DisplayName, &p.TTR, &p.CreatedAt, &p.IsAdmin); err != nil {
			return nil, fmt.Errorf("read player: %w", err)
		}
		players = append(players, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read player list: %w", err)
	}
	return players, nil
}

// Records counts confirmed matches per player in one statement.
//
// The winner comes from the set scores rather than from the rating history:
// a strong favourite who wins can move by zero points, so "did the rating go
// up" is not the same question as "did they win".
func (r playerRepo) Records(ctx context.Context) ([]domain.PlayerRecord, error) {
	const q = `
		with decided as (
			select m.id,
			       m.home_id,
			       m.away_id,
			       count(*) filter (where s.home_points > s.away_points) as home_sets,
			       count(*) filter (where s.away_points > s.home_points) as away_sets
			from matches m
			join match_sets s on s.match_id = m.id
			where m.status = 'confirmed'
			group by m.id
		),
		per_player as (
			select home_id as player_id, home_sets > away_sets as won from decided
			union all
			select away_id as player_id, away_sets > home_sets as won from decided
		)
		select p.id, p.display_name, p.ttr, p.created_at,
		       count(pp.player_id) as played,
		       count(*) filter (where pp.won) as won
		from players p
		left join per_player pp on pp.player_id = p.id
		group by p.id, p.display_name, p.ttr, p.created_at
		order by p.ttr desc, lower(btrim(p.display_name))`

	rows, err := r.q.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("load player records: %w", err)
	}
	defer rows.Close()

	var records []domain.PlayerRecord
	for rows.Next() {
		var rec domain.PlayerRecord
		err := rows.Scan(&rec.Player.ID, &rec.Player.DisplayName, &rec.Player.TTR,
			&rec.Player.CreatedAt, &rec.Played, &rec.Won)
		if err != nil {
			return nil, fmt.Errorf("read player record: %w", err)
		}
		rec.Lost = rec.Played - rec.Won
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read player records: %w", err)
	}
	return records, nil
}

// ByDisplayName resolves the name SP_BOOTSTRAP_ADMIN carries.
//
// Matched the way players_display_name_key is unique — trimmed and folded —
// so the variable does not have to reproduce whatever casing somebody typed
// into the join form.
func (r playerRepo) ByDisplayName(ctx context.Context, name string) (domain.Player, error) {
	const q = `
		select id, display_name, ttr, created_at, is_admin
		from players
		where lower(btrim(display_name)) = lower(btrim($1))`

	var p domain.Player
	err := r.q.QueryRow(ctx, q, name).Scan(&p.ID, &p.DisplayName, &p.TTR, &p.CreatedAt, &p.IsAdmin)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.Player{}, fmt.Errorf("player %q: %w", name, domain.ErrNotFound)
	case err != nil:
		return domain.Player{}, fmt.Errorf("load player %q: %w", name, err)
	}
	return p, nil
}

// Admins returns everybody holding the flag, by name.
//
// By name rather than by rating: this list answers "who may act for other
// people", and the ranking order would suggest the two have something to do
// with each other.
func (r playerRepo) Admins(ctx context.Context) ([]domain.Player, error) {
	const q = `
		select id, display_name, ttr, created_at, is_admin
		from players
		where is_admin
		order by lower(btrim(display_name))`

	rows, err := r.q.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("load admins: %w", err)
	}
	defer rows.Close()

	var admins []domain.Player
	for rows.Next() {
		var p domain.Player
		if err := rows.Scan(&p.ID, &p.DisplayName, &p.TTR, &p.CreatedAt, &p.IsAdmin); err != nil {
			return nil, fmt.Errorf("read admin: %w", err)
		}
		admins = append(admins, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read admins: %w", err)
	}
	return admins, nil
}

// SetAdmin grants or withdraws the flag.
func (r playerRepo) SetAdmin(ctx context.Context, id uuid.UUID, isAdmin bool) error {
	tag, err := r.q.Exec(ctx, `update players set is_admin = $2 where id = $1`, id, isAdmin)
	if err != nil {
		return fmt.Errorf("set admin flag of player %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("player %s: %w", id, domain.ErrNotFound)
	}
	return nil
}

func (r playerRepo) Count(ctx context.Context) (int, error) {
	var n int
	if err := r.q.QueryRow(ctx, `select count(*) from players`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count players: %w", err)
	}
	return n, nil
}

func (r playerRepo) UpdateTTR(ctx context.Context, id uuid.UUID, ttr int) error {
	tag, err := r.q.Exec(ctx, `update players set ttr = $2 where id = $1`, id, ttr)
	if err != nil {
		return fmt.Errorf("set ttr of player %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("player %s: %w", id, domain.ErrNotFound)
	}
	return nil
}
