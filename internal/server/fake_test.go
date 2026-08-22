package server_test

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/repository"
)

// memStore is an in-memory repository.Store.
//
// This is what the repository interfaces buy (invariant 5): the handlers run
// against it exactly as they run against Postgres, with no database in sight.
// Methods that are not implemented stay nil and panic when called, which is
// intended — it surfaces immediately rather than returning a quiet zero value.
type memStore struct {
	repository.Store
	players    *memPlayers
	identities *memIdentities
	matches    *memMatches
	history    *memHistory
	pingErr    error
}

func newMemStore() *memStore {
	matches := &memMatches{}
	players := &memPlayers{matches: matches}
	return &memStore{
		players:    players,
		identities: &memIdentities{players: players},
		matches:    matches,
		history:    &memHistory{},
	}
}

func (m *memStore) Ping(context.Context) error                  { return m.pingErr }
func (m *memStore) Players() repository.PlayerRepository        { return m.players }
func (m *memStore) Identities() repository.IdentityRepository   { return m.identities }
func (m *memStore) Matches() repository.MatchRepository         { return m.matches }
func (m *memStore) TTRHistory() repository.TTRHistoryRepository { return m.history }

// InTx runs fn against the same store. There is no rollback here, which is
// fine for handler tests — that transactions actually hold is covered against
// real Postgres in internal/repository/postgres.
func (m *memStore) InTx(_ context.Context, fn func(repository.Store) error) error { return fn(m) }

type memPlayers struct {
	repository.PlayerRepository
	// matches lets Records count confirmed results, the same join the
	// Postgres implementation does in one statement.
	matches *memMatches
	mu      sync.Mutex
	rows    []domain.Player
}

// Records mirrors the Postgres aggregate: confirmed matches only, and the
// winner from the set scores rather than from the rating change.
func (p *memPlayers) Records(ctx context.Context) ([]domain.PlayerRecord, error) {
	players, err := p.List(ctx)
	if err != nil {
		return nil, err
	}

	records := make([]domain.PlayerRecord, 0, len(players))
	for _, player := range players {
		record := domain.PlayerRecord{Player: player}

		for _, m := range p.matches.all() {
			if m.Status != domain.MatchConfirmed {
				continue
			}
			atHome := m.HomeID == player.ID
			if !atHome && m.AwayID != player.ID {
				continue
			}

			var home, away int
			for _, set := range m.Sets {
				if set.HomePoints > set.AwayPoints {
					home++
				} else {
					away++
				}
			}

			record.Played++
			if (atHome && home > away) || (!atHome && away > home) {
				record.Won++
			}
		}
		record.Lost = record.Played - record.Won
		records = append(records, record)
	}
	return records, nil
}

func (p *memPlayers) Create(_ context.Context, displayName string, ttr int) (domain.Player, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Mirrors players_display_name_key: unique on the trimmed, lowercased name.
	for _, existing := range p.rows {
		if strings.EqualFold(strings.TrimSpace(existing.DisplayName), strings.TrimSpace(displayName)) {
			return domain.Player{}, domain.ErrConflict
		}
	}

	player := domain.Player{
		ID:          uuid.New(),
		DisplayName: displayName,
		TTR:         ttr,
		CreatedAt:   time.Now(),
	}
	p.rows = append(p.rows, player)
	return player, nil
}

func (p *memPlayers) ByID(_ context.Context, id uuid.UUID) (domain.Player, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, player := range p.rows {
		if player.ID == id {
			return player, nil
		}
	}
	return domain.Player{}, domain.ErrNotFound
}

func (p *memPlayers) List(context.Context) ([]domain.Player, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	return append([]domain.Player(nil), p.rows...), nil
}

func (p *memPlayers) UpdateTTR(_ context.Context, id uuid.UUID, ttr int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := range p.rows {
		if p.rows[i].ID == id {
			p.rows[i].TTR = ttr
			return nil
		}
	}
	return domain.ErrNotFound
}

func (p *memPlayers) Count(context.Context) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	return len(p.rows), nil
}

type memIdentities struct {
	repository.IdentityRepository
	// players resolves a linked id to the full record, the same join the
	// Postgres implementation does.
	players *memPlayers
	mu      sync.Mutex
	rows    map[string]uuid.UUID
}

func (i *memIdentities) key(provider domain.Provider, subject string) string {
	return string(provider) + "\x00" + subject
}

func (i *memIdentities) Link(_ context.Context, provider domain.Provider, subject string, playerID uuid.UUID) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.rows == nil {
		i.rows = map[string]uuid.UUID{}
	}
	i.rows[i.key(provider, subject)] = playerID
	return nil
}

func (i *memIdentities) PlayerBy(ctx context.Context, provider domain.Provider, subject string) (domain.Player, error) {
	i.mu.Lock()
	id, ok := i.rows[i.key(provider, subject)]
	i.mu.Unlock()

	if !ok {
		return domain.Player{}, domain.ErrNotFound
	}
	return i.players.ByID(ctx, id)
}

type memMatches struct {
	repository.MatchRepository
	mu   sync.Mutex
	rows []domain.Match
}

func (m *memMatches) Create(_ context.Context, in domain.Match) (domain.Match, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	in.ID = uuid.New()
	if in.PlayedAt.IsZero() {
		in.PlayedAt = time.Now()
	}
	if in.Status == "" {
		in.Status = domain.MatchPending
	}
	m.rows = append(m.rows, in)
	return in, nil
}

// all returns a copy of the stored matches, for assertions.
func (m *memMatches) all() []domain.Match {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]domain.Match(nil), m.rows...)
}

func (m *memMatches) ByID(_ context.Context, id uuid.UUID) (domain.Match, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, row := range m.rows {
		if row.ID == id {
			return row, nil
		}
	}
	return domain.Match{}, domain.ErrNotFound
}

// PendingFor mirrors the Postgres query: the player is in it, and it is
// either pending under somebody else's name or contested by either of them.
func (m *memMatches) PendingFor(_ context.Context, playerID uuid.UUID) ([]domain.Match, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var out []domain.Match
	for _, row := range m.rows {
		if row.HomeID != playerID && row.AwayID != playerID {
			continue
		}
		waiting := (row.Status == domain.MatchPending && row.ReportedBy != playerID) ||
			row.Status == domain.MatchDisputed
		if waiting {
			out = append(out, row)
		}
	}
	return out, nil
}

// RecentFor mirrors the Postgres query: every match the player is in, newest
// first, whatever its status.
func (m *memMatches) RecentFor(_ context.Context, playerID uuid.UUID, limit int) ([]domain.Match, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var out []domain.Match
	for i := len(m.rows) - 1; i >= 0; i-- {
		row := m.rows[i]
		if row.HomeID == playerID || row.AwayID == playerID {
			out = append(out, row)
		}
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func (m *memMatches) SetStatus(_ context.Context, id uuid.UUID, status domain.MatchStatus, confirmedAt *time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.rows {
		if m.rows[i].ID != id {
			continue
		}
		m.rows[i].Status = status
		m.rows[i].ConfirmedAt = confirmedAt
		return nil
	}
	return domain.ErrNotFound
}

// ReplaceResult mirrors the Postgres statement, including that the status is
// part of the condition rather than checked before it.
func (m *memMatches) ReplaceResult(_ context.Context, id uuid.UUID, corrected domain.Match) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.rows {
		if m.rows[i].ID != id {
			continue
		}
		if m.rows[i].Status != domain.MatchDisputed {
			return domain.ErrConflict
		}
		m.rows[i].BestOf = corrected.BestOf
		m.rows[i].PointsToWin = corrected.PointsToWin
		m.rows[i].ReportedBy = corrected.ReportedBy
		m.rows[i].Sets = corrected.Sets
		m.rows[i].Status = domain.MatchPending
		m.rows[i].ConfirmedAt = nil
		return nil
	}
	return domain.ErrNotFound
}

type memHistory struct {
	repository.TTRHistoryRepository
	mu   sync.Mutex
	rows []domain.TTRChange
}

func (h *memHistory) Append(_ context.Context, changes []domain.TTRChange) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Mirrors ttr_history_player_match_key: one entry per player and match,
	// which is what stops a rating being settled twice.
	for _, c := range changes {
		for _, existing := range h.rows {
			if existing.PlayerID == c.PlayerID && existing.MatchID == c.MatchID {
				return domain.ErrConflict
			}
		}
	}
	h.rows = append(h.rows, changes...)
	return nil
}

func (h *memHistory) ForPlayer(_ context.Context, playerID uuid.UUID, limit int) ([]domain.TTRChange, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	var out []domain.TTRChange
	for _, c := range h.rows {
		if c.PlayerID == playerID {
			out = append(out, c)
		}
		if len(out) == limit {
			break
		}
	}
	return out, nil
}
