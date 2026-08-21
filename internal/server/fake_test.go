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
	pingErr    error
}

func newMemStore() *memStore {
	players := &memPlayers{}
	return &memStore{players: players, identities: &memIdentities{players: players}}
}

func (m *memStore) Ping(context.Context) error                { return m.pingErr }
func (m *memStore) Players() repository.PlayerRepository      { return m.players }
func (m *memStore) Identities() repository.IdentityRepository { return m.identities }

// InTx runs fn against the same store. There is no rollback here, which is
// fine for handler tests — that transactions actually hold is covered against
// real Postgres in internal/repository/postgres.
func (m *memStore) InTx(_ context.Context, fn func(repository.Store) error) error { return fn(m) }

type memPlayers struct {
	repository.PlayerRepository
	mu   sync.Mutex
	rows []domain.Player
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
