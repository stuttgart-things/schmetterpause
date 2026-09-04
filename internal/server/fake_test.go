package server_test

import (
	"context"
	"slices"
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
	players     *memPlayers
	identities  *memIdentities
	credentials *memCredentials
	kiosks      *memKioskGrants
	matches     *memMatches
	history     *memHistory
	tournaments *memTournaments
	pingErr     error
}

func newMemStore() *memStore {
	history := &memHistory{}
	matches := &memMatches{history: history}
	players := &memPlayers{matches: matches}
	return &memStore{
		players:     players,
		identities:  &memIdentities{players: players},
		credentials: &memCredentials{},
		kiosks:      &memKioskGrants{},
		matches:     matches,
		history:     history,
		tournaments: &memTournaments{matches: matches},
	}
}

func (m *memStore) Ping(context.Context) error                { return m.pingErr }
func (m *memStore) Players() repository.PlayerRepository      { return m.players }
func (m *memStore) Identities() repository.IdentityRepository { return m.identities }
func (m *memStore) Credentials() repository.CredentialRepository {
	return m.credentials
}
func (m *memStore) KioskGrants() repository.KioskGrantRepository { return m.kiosks }
func (m *memStore) Matches() repository.MatchRepository          { return m.matches }
func (m *memStore) TTRHistory() repository.TTRHistoryRepository  { return m.history }
func (m *memStore) Tournaments() repository.TournamentRepository { return m.tournaments }

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

func (p *memPlayers) ByDisplayName(_ context.Context, name string) (domain.Player, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Mirrors players_display_name_key: trimmed and folded.
	for _, row := range p.rows {
		if strings.EqualFold(strings.TrimSpace(row.DisplayName), strings.TrimSpace(name)) {
			return row, nil
		}
	}
	return domain.Player{}, domain.ErrNotFound
}

func (p *memPlayers) Admins(_ context.Context) ([]domain.Player, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	var admins []domain.Player
	for _, row := range p.rows {
		if row.IsAdmin {
			admins = append(admins, row)
		}
	}
	slices.SortFunc(admins, func(a, b domain.Player) int {
		return strings.Compare(strings.ToLower(a.DisplayName), strings.ToLower(b.DisplayName))
	})
	return admins, nil
}

func (p *memPlayers) SetAdmin(_ context.Context, id uuid.UUID, isAdmin bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := range p.rows {
		if p.rows[i].ID == id {
			p.rows[i].IsAdmin = isAdmin
			return nil
		}
	}
	return domain.ErrNotFound
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

func (i *memIdentities) Unlink(_ context.Context, provider domain.Provider, subject string) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	// A row that is not there is not an error, the same as against Postgres.
	delete(i.rows, i.key(provider, subject))
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

// memCredentials mirrors the primary key on player_credentials: one row per
// player and kind, so a second Put of the same kind replaces the first.
type memCredentials struct {
	repository.CredentialRepository
	mu   sync.Mutex
	rows map[string]domain.Credential
}

func (c *memCredentials) key(playerID uuid.UUID, kind domain.CredentialKind) string {
	return playerID.String() + "\x00" + string(kind)
}

func (c *memCredentials) Put(_ context.Context, playerID uuid.UUID, kind domain.CredentialKind, hash string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.rows == nil {
		c.rows = map[string]domain.Credential{}
	}
	c.rows[c.key(playerID, kind)] = domain.Credential{
		PlayerID:  playerID,
		Kind:      kind,
		Hash:      hash,
		UpdatedAt: time.Now(),
	}
	return nil
}

func (c *memCredentials) ForPlayer(_ context.Context, playerID uuid.UUID, kind domain.CredentialKind) (domain.Credential, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	row, ok := c.rows[c.key(playerID, kind)]
	if !ok {
		return domain.Credential{}, domain.ErrNotFound
	}
	return row, nil
}

// memKioskGrants mirrors kiosk_grants: one row per unlocked machine, keyed on
// the hash of the secret its cookie carries.
type memKioskGrants struct {
	repository.KioskGrantRepository
	mu   sync.Mutex
	rows []domain.KioskGrant
	// secrets maps the hash to the row, the way the unique index does.
	secrets map[string]uuid.UUID
}

func (k *memKioskGrants) Create(
	_ context.Context, secretHash []byte, expiresAt time.Time, userAgent string,
) (domain.KioskGrant, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.secrets == nil {
		k.secrets = map[string]uuid.UUID{}
	}
	now := time.Now()
	g := domain.KioskGrant{
		ID:         uuid.New(),
		CreatedAt:  now,
		LastSeenAt: now,
		ExpiresAt:  expiresAt,
		UserAgent:  userAgent,
	}
	k.rows = append(k.rows, g)
	k.secrets[string(secretHash)] = g.ID
	return g, nil
}

func (k *memKioskGrants) BySecret(_ context.Context, secretHash []byte) (domain.KioskGrant, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	id, ok := k.secrets[string(secretHash)]
	if !ok {
		return domain.KioskGrant{}, domain.ErrNotFound
	}
	for _, g := range k.rows {
		if g.ID == id {
			return g, nil
		}
	}
	return domain.KioskGrant{}, domain.ErrNotFound
}

func (k *memKioskGrants) Touch(_ context.Context, id uuid.UUID, at time.Time) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	for i := range k.rows {
		if k.rows[i].ID == id {
			k.rows[i].LastSeenAt = at
		}
	}
	return nil
}

func (k *memKioskGrants) SetOperator(
	_ context.Context, id, playerID uuid.UUID, at time.Time,
) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	for i := range k.rows {
		if k.rows[i].ID != id || !k.rows[i].Active(at) {
			continue
		}
		k.rows[i].OperatorID = &playerID
		return nil
	}
	return domain.ErrNotFound
}

func (k *memKioskGrants) Revoke(_ context.Context, id uuid.UUID, at time.Time) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	for i := range k.rows {
		// Only the first revocation counts, the same as the where clause
		// against Postgres.
		if k.rows[i].ID == id && k.rows[i].RevokedAt == nil {
			when := at
			k.rows[i].RevokedAt = &when
		}
	}
	return nil
}

func (k *memKioskGrants) RevokeAll(_ context.Context, at time.Time) (int, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	n := 0
	for i := range k.rows {
		if k.rows[i].Active(at) {
			when := at
			k.rows[i].RevokedAt = &when
			n++
		}
	}
	return n, nil
}

func (k *memKioskGrants) Active(_ context.Context, at time.Time) ([]domain.KioskGrant, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	var active []domain.KioskGrant
	for _, g := range k.rows {
		if g.Active(at) {
			active = append(active, g)
		}
	}
	slices.SortFunc(active, func(a, b domain.KioskGrant) int {
		return b.LastSeenAt.Compare(a.LastSeenAt)
	})
	return active, nil
}

type memMatches struct {
	repository.MatchRepository
	mu   sync.Mutex
	rows []domain.Match
	// history is what the schema cascades to when a match is deleted. The
	// fake has to do by hand what "on delete cascade" does in Postgres, or
	// an undo would leave the rating history behind and the next one would
	// refuse to run.
	history *memHistory
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
	// Mirrors the column default, so a handler that forgets to set it looks
	// the same here as it does against Postgres.
	if in.EnteredVia == "" {
		in.EnteredVia = domain.EnteredViaPlayer
	}
	m.rows = append(m.rows, in)
	return in, nil
}

// backdate moves a stored match into the past, so a test can watch something
// age without waiting for it to.
func (m *memMatches) backdate(id uuid.UUID, at time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.rows {
		if m.rows[i].ID == id {
			m.rows[i].PlayedAt = at
			return
		}
	}
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

func (m *memMatches) PendingCountFor(ctx context.Context, playerID uuid.UUID) (int, error) {
	waiting, err := m.PendingFor(ctx, playerID)
	return len(waiting), err
}

// WaitingOnOpponentFor mirrors the Postgres query: pending under this
// player's own name. Disputed is deliberately absent — those are in
// PendingFor for both sides already, and one match under two headings reads
// as two matches.
//
// Oldest first, like the query, so a test that asserts on order asserts on
// the order a reader sees.
func (m *memMatches) WaitingOnOpponentFor(_ context.Context, playerID uuid.UUID) ([]domain.Match, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var out []domain.Match
	for _, row := range m.rows {
		if row.HomeID != playerID && row.AwayID != playerID {
			continue
		}
		if row.Status == domain.MatchPending && row.ReportedBy == playerID {
			out = append(out, row)
		}
	}
	slices.SortFunc(out, func(a, b domain.Match) int {
		return a.PlayedAt.Compare(b.PlayedAt)
	})
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

func (m *memMatches) Recent(_ context.Context, limit int) ([]domain.Match, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Newest first, like "order by played_at desc". Insertion order stands
	// in for it: the fake stamps PlayedAt as it goes.
	var out []domain.Match
	for i := len(m.rows) - 1; i >= 0; i-- {
		out = append(out, m.rows[i])
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

// Delete mirrors the Postgres statement, cascade included: the sets live on
// the row here, and the history is dropped alongside because the schema does.
func (m *memMatches) Delete(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.rows {
		if m.rows[i].ID != id || m.rows[i].Status != domain.MatchConfirmed {
			continue
		}
		m.rows = append(m.rows[:i], m.rows[i+1:]...)
		if m.history != nil {
			m.history.dropMatch(id)
		}
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

func (h *memHistory) ForMatch(_ context.Context, matchID uuid.UUID) ([]domain.TTRChange, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	var out []domain.TTRChange
	for _, c := range h.rows {
		if c.MatchID == matchID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (h *memHistory) ForMatches(_ context.Context, matchIDs []uuid.UUID) ([]domain.TTRChange, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	wanted := make(map[uuid.UUID]bool, len(matchIDs))
	for _, id := range matchIDs {
		wanted[id] = true
	}

	var out []domain.TTRChange
	for _, c := range h.rows {
		if wanted[c.MatchID] {
			out = append(out, c)
		}
	}
	return out, nil
}

// dropMatch is the cascade, called from memMatches.Delete.
func (h *memHistory) dropMatch(matchID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	kept := h.rows[:0]
	for _, c := range h.rows {
		if c.MatchID != matchID {
			kept = append(kept, c)
		}
	}
	h.rows = kept
}

func (h *memHistory) ForPlayer(_ context.Context, playerID uuid.UUID, limit int) ([]domain.TTRChange, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Newest first, like the "order by created_at desc" in Postgres. The
	// fake used to hand them back oldest first, which made ForPlayer answer
	// a different question here than in production.
	var out []domain.TTRChange
	for i := len(h.rows) - 1; i >= 0; i-- {
		if h.rows[i].PlayerID == playerID {
			out = append(out, h.rows[i])
		}
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

// memTournaments is enough of a tournament store for the handler tests: the
// kiosk asks it which ones are still open, and nothing here needs the draw.
type memTournaments struct {
	repository.TournamentRepository
	// matches is where booked results live: a tournament owns which matches
	// belong to it, not the matches themselves (docs/adr/0009).
	matches *memMatches
	mu      sync.Mutex
	rows    []domain.Tournament
}

func (m *memTournaments) Create(_ context.Context, t domain.Tournament) (domain.Tournament, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	if t.Status == "" {
		t.Status = domain.TournamentOpen
	}
	m.rows = append(m.rows, t)
	return t, nil
}

// List answers in the order Postgres does: open ones first, newest first
// within each group. The kiosk only shows the open ones, but a fake that
// ordered them differently would hide an ordering bug rather than catch it.
func (m *memTournaments) List(_ context.Context, limit int) ([]domain.Tournament, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := slices.Clone(m.rows)
	slices.SortStableFunc(out, func(a, b domain.Tournament) int {
		if a.Open() != b.Open() {
			if a.Open() {
				return -1
			}
			return 1
		}
		return b.CreatedAt.Compare(a.CreatedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *memTournaments) ByID(_ context.Context, id uuid.UUID) (domain.Tournament, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, t := range m.rows {
		if t.ID == id {
			return t, nil
		}
	}
	return domain.Tournament{}, domain.ErrNotFound
}

func (m *memTournaments) Close(_ context.Context, id uuid.UUID, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, t := range m.rows {
		if t.ID == id {
			m.rows[i].Status = domain.TournamentClosed
			m.rows[i].ClosedAt = &at
			return nil
		}
	}
	return domain.ErrNotFound
}

// Matches returns what has been booked to this tournament, every status, the
// way Postgres does — the draw has to tell "not played" from "waiting on a
// confirmation".
func (m *memTournaments) Matches(_ context.Context, id uuid.UUID) ([]domain.Match, error) {
	if m.matches == nil {
		return nil, nil
	}
	var out []domain.Match
	for _, match := range m.matches.all() {
		if match.TournamentID != nil && *match.TournamentID == id {
			out = append(out, match)
		}
	}
	return out, nil
}

func (m *memTournaments) DeleteIfEmpty(ctx context.Context, id uuid.UUID) (bool, error) {
	played, _ := m.Matches(ctx, id)
	if len(played) > 0 {
		return false, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for i, t := range m.rows {
		if t.ID == id {
			m.rows = append(m.rows[:i], m.rows[i+1:]...)
			return true, nil
		}
	}
	return false, nil
}

func (m *memTournaments) Replace(ctx context.Context, in domain.Tournament) (domain.Tournament, error) {
	played, _ := m.Matches(ctx, in.ID)
	if len(played) > 0 {
		return domain.Tournament{}, domain.ErrNotFound
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for i, t := range m.rows {
		if t.ID != in.ID {
			continue
		}
		t.Name, t.Format = in.Name, in.Format
		t.BestOf, t.PointsToWin, t.WithFinal = in.BestOf, in.PointsToWin, in.WithFinal
		t.Players = slices.Clone(in.Players)
		m.rows[i] = t
		return t, nil
	}
	return domain.Tournament{}, domain.ErrNotFound
}
