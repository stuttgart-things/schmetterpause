package server_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/config"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// A deployment with SP_SCOREBOARD_SETTLE=immediate, the Pong sandbox of
// docs/adr/0027: nobody there confirms anything, so a result from the
// Zählwerk counts the moment it arrives, on both of its paths.

func immediateScoreboardHandler(t *testing.T) (http.Handler, *memStore) {
	t.Helper()

	store := newMemStore()
	cfg := testConfig()
	cfg.ScoreboardToken = testScoreboardToken
	cfg.ScoreboardSettle = config.SettleImmediate
	cfg.SessionKey = testSessionKey
	h := newHandlerConfig(cfg, store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))
	return h, store
}

// settledAnswer decodes a 201 and requires it to say confirmed.
func settledAnswer(t *testing.T, raw []byte) uuid.UUID {
	t.Helper()

	var created struct {
		MatchID uuid.UUID `json:"match_id"`
		Status  string    `json:"status"`
	}
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatalf("decoding the response: %v", err)
	}
	if created.Status != string(domain.MatchConfirmed) {
		t.Errorf("answered status = %q, want confirmed", created.Status)
	}
	return created.MatchID
}

// ratingsMoved requires the winner up and the loser down by the same points,
// with a history entry each that explains it.
func ratingsMoved(t *testing.T, store *memStore, matchID uuid.UUID, winner, loser domain.Player) {
	t.Helper()

	w, err := store.Players().ByID(t.Context(), winner.ID)
	if err != nil {
		t.Fatalf("reading the winner: %v", err)
	}
	l, err := store.Players().ByID(t.Context(), loser.ID)
	if err != nil {
		t.Fatalf("reading the loser: %v", err)
	}
	if w.TTR <= winner.TTR || l.TTR >= loser.TTR {
		t.Errorf("TTR did not move: winner %d -> %d, loser %d -> %d", winner.TTR, w.TTR, loser.TTR, l.TTR)
	}
	if w.TTR-winner.TTR != loser.TTR-l.TTR {
		t.Errorf("TTR is not conserved: winner +%d, loser -%d", w.TTR-winner.TTR, loser.TTR-l.TTR)
	}
	for _, p := range []domain.Player{winner, loser} {
		history, err := store.TTRHistory().ForPlayer(t.Context(), p.ID, 10)
		if err != nil {
			t.Fatalf("reading the history of %s: %v", p.DisplayName, err)
		}
		if len(history) != 1 || history[0].MatchID != matchID {
			t.Errorf("history of %s = %+v, want one entry for match %s", p.DisplayName, history, matchID)
		}
	}
}

func TestAnImmediateScoreboardResultCountsAtOnceAndKeepsItsOperator(t *testing.T) {
	h, store := immediateScoreboardHandler(t)
	home, away, operator := threePlayers(t, store)

	rec := apiPost(t, h, testScoreboardToken, goodResult(home, away, operator))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/results = %d %s, want 201", rec.Code, rec.Body)
	}
	id := settledAnswer(t, rec.Body.Bytes())

	stored, err := store.Matches().ByID(t.Context(), id)
	if err != nil {
		t.Fatalf("reading the match: %v", err)
	}
	if stored.Status != domain.MatchConfirmed || stored.ConfirmedAt == nil {
		t.Errorf("stored status = %q, confirmed_at %v, want confirmed with a time", stored.Status, stored.ConfirmedAt)
	}
	// Only the timing changes: the row still says where it came from and
	// who watched.
	if stored.EnteredVia != domain.EnteredViaScoreboard {
		t.Errorf("entered_via = %q, want %q", stored.EnteredVia, domain.EnteredViaScoreboard)
	}
	if stored.ReportedBy != operator.ID {
		t.Errorf("reported_by = %s, want the operator %s", stored.ReportedBy, operator.ID)
	}
	ratingsMoved(t, store, id, home, away)
}

func TestAnImmediateCheckedInResultCountsAtOnceWithNobodyReporting(t *testing.T) {
	h, store := immediateScoreboardHandler(t)
	home, away, _ := threePlayers(t, store)
	left := checkedIn(t, store, home.ID, domain.HalfLeft)
	right := checkedIn(t, store, away.ID, domain.HalfRight)

	rec := apiPost(t, h, testScoreboardToken, checkedInResult(home, away, left.ID, right.ID))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/results = %d %s, want 201", rec.Code, rec.Body)
	}
	id := settledAnswer(t, rec.Body.Bytes())

	stored, err := store.Matches().ByID(t.Context(), id)
	if err != nil {
		t.Fatalf("reading the match: %v", err)
	}
	if stored.Status != domain.MatchConfirmed {
		t.Errorf("stored status = %q, want confirmed", stored.Status)
	}
	if stored.EnteredVia != domain.EnteredViaCheckin || stored.ReportedBy != uuid.Nil {
		t.Errorf("entered_via %q, reported_by %s, want checkin and nobody", stored.EnteredVia, stored.ReportedBy)
	}
	ratingsMoved(t, store, id, home, away)
}

// A refused result moves nothing, settled or not: the check runs before the
// transaction that would count it.
func TestAnImmediateResultWithTheOperatorPlayingIsRefusedAndMovesNothing(t *testing.T) {
	h, store := immediateScoreboardHandler(t)
	home, away, _ := threePlayers(t, store)

	rec := apiPost(t, h, testScoreboardToken, goodResult(home, away, home))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("POST /api/results = %d %s, want 422", rec.Code, rec.Body)
	}
	after, err := store.Players().ByID(t.Context(), home.ID)
	if err != nil {
		t.Fatalf("reading the player: %v", err)
	}
	if after.TTR != home.TTR {
		t.Errorf("TTR moved on a refused result: %d -> %d", home.TTR, after.TTR)
	}
}
