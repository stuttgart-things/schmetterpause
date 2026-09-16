package server

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/match"
)

// The only JSON this application speaks, and the only place it may grow.
//
// CLAUDE.md forbade JSON handlers "as long as no external consumer exists".
// docs/adr/0015 is the consumer arriving: the Zählwerk counts the match at the
// table and hands the finished result over. That invariant is therefore used
// up rather than lifted — it still holds for everything without a consumer,
// and this surface stays exactly as narrow as the ADR describes. Anything
// wider needs its own ADR.
//
// Two routes, one direction each: players out, one finished result in. No
// updates, no deletes, no listing of matches, and nothing that lets the caller
// change a rating.

// maxAPIBody bounds what a handler will read. The largest legitimate body is a
// result with a handful of sets — a few hundred bytes — so this is four orders
// of magnitude of headroom and still refuses a body meant to exhaust memory.
const maxAPIBody = 64 << 10

// apiPlayer is one row of GET /api/players.
//
// Three fields, because three are what the other side needs to put a name on a
// result: the id to send back, the name to show at the table, and the rating so
// a scoreboard can display it without asking a second time. Deliberately not
// the whole domain.Player — this is a published shape, and every field added
// here is one somebody may come to depend on.
type apiPlayer struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
	TTR         int       `json:"ttr"`
}

// apiResult is the body of POST /api/results.
//
// Sets are pairs rather than objects so that the sender can pass its own
// scorer.State.CompletedSets ([][2]int) through unchanged — it is the field
// that exists over there for exactly this handover. Home first, away second,
// in the order they were played.
type apiResult struct {
	HomeID      uuid.UUID `json:"home_id"`
	AwayID      uuid.UUID `json:"away_id"`
	OperatorID  uuid.UUID `json:"operator_id"`
	Sets        [][2]int  `json:"sets"`
	BestOf      int       `json:"best_of"`
	PointsToWin int       `json:"points_to_win"`
	// PlayedAt is optional. The Zählwerk knows when the match ended; if it
	// says nothing, the moment the result arrives is close enough.
	PlayedAt *time.Time `json:"played_at,omitempty"`
}

// apiError is what every refusal on this surface looks like.
//
// One shape, because the reader is a program: a machine that has to tell "you
// sent nonsense" from "that player does not exist" should not have to parse
// German prose. The message stays short and says what to change.
type apiError struct {
	Error string `json:"error"`
}

// scoreboardAuthorized reports whether the request carries the token.
//
// Bearer, and constant-time, like kioskTokenMatches. A token in a header
// rather than a query string, so it does not end up in an access log.
func (s *Server) scoreboardAuthorized(r *http.Request) bool {
	header := r.Header.Get("Authorization")
	token, ok := strings.CutPrefix(header, "Bearer ")
	if !ok {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.ScoreboardToken)) == 1
}

// writeJSON is the one place this package encodes.
func (s *Server) writeJSON(w http.ResponseWriter, r *http.Request, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		// The status line has already gone; there is nothing to tell the
		// caller. Logged so it is not invisible.
		s.log.ErrorContext(r.Context(), "writing the api response failed", "error", err)
	}
}

func (s *Server) apiRefuse(w http.ResponseWriter, r *http.Request, status int, msg string) {
	s.writeJSON(w, r, status, apiError{Error: msg})
}

// handleAPIPlayers lists the players so a result can belong to somebody.
//
// Read, not mirrored: docs/adr/0015 keeps this application the only owner of
// player identity, and the other side holds no copy. That is why there is no
// ETag and no pagination — the caller is expected to ask again, and an office
// has tens of players.
func (s *Server) handleAPIPlayers(w http.ResponseWriter, r *http.Request) {
	if !s.scoreboardAuthorized(r) {
		s.apiRefuse(w, r, http.StatusUnauthorized, "a bearer token is required")
		return
	}

	// Playing, not List: an observer is never one of the two sides, so the
	// Zählwerk is not offered one (docs/adr/0022).
	players, err := s.store.Players().Playing(r.Context())
	if err != nil {
		s.log.ErrorContext(r.Context(), "listing players for the api failed", "error", err)
		s.apiRefuse(w, r, http.StatusInternalServerError, "the player list is not available")
		return
	}

	out := make([]apiPlayer, 0, len(players))
	for _, p := range players {
		out = append(out, apiPlayer{ID: p.ID, DisplayName: p.DisplayName, TTR: p.TTR})
	}
	s.writeJSON(w, r, http.StatusOK, out)
}

// handleAPIResults takes a finished match from the Zählwerk.
//
// It creates the match pending rather than calling scoring.Record, which is
// the one place this path deliberately differs from the kiosk. docs/adr/0015:
// while the chain button -> Zählwerk -> database is still being tested, a
// result somebody agreed to is worth more than one already in the ranking. The
// change later is a different line, not a different design.
func (s *Server) handleAPIResults(w http.ResponseWriter, r *http.Request) {
	if !s.scoreboardAuthorized(r) {
		s.apiRefuse(w, r, http.StatusUnauthorized, "a bearer token is required")
		return
	}

	var body apiResult
	dec := json.NewDecoder(io.LimitReader(r.Body, maxAPIBody))
	// A misspelt field is a bug on the sending side, and silently dropping it
	// would enter a result that is not the one that was played.
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		s.apiRefuse(w, r, http.StatusBadRequest, "the body is not a result: "+err.Error())
		return
	}

	result, msg := body.toMatchResult()
	if msg != "" {
		s.apiRefuse(w, r, http.StatusBadRequest, msg)
		return
	}

	if body.HomeID == uuid.Nil || body.AwayID == uuid.Nil {
		s.apiRefuse(w, r, http.StatusBadRequest, "home_id and away_id are required")
		return
	}
	if body.HomeID == body.AwayID {
		s.apiRefuse(w, r, http.StatusBadRequest, "home_id and away_id must differ")
		return
	}

	// The operator is required, and may not be playing. docs/adr/0014 gives
	// the reason and docs/adr/0015 keeps it unchanged: a machine has no
	// identity, so it names the person standing there who watched the match.
	// Without that, a result has nobody behind it and both players could
	// confirm a match neither of them reported.
	if body.OperatorID == uuid.Nil {
		s.apiRefuse(w, r, http.StatusBadRequest, "operator_id is required")
		return
	}
	if body.OperatorID == body.HomeID || body.OperatorID == body.AwayID {
		s.apiRefuse(w, r, http.StatusUnprocessableEntity,
			"the operator may not be one of the two players")
		return
	}

	for _, id := range []uuid.UUID{body.HomeID, body.AwayID, body.OperatorID} {
		if _, err := s.store.Players().ByID(r.Context(), id); err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				// Which one is named, because the sender chose all three from
				// a list this application gave it: a mismatch means the list
				// is stale, and saying which id is unknown is what makes that
				// diagnosable. It leaks nothing — the caller already holds the
				// whole list.
				s.apiRefuse(w, r, http.StatusUnprocessableEntity,
					fmt.Sprintf("no player with id %s", id))
				return
			}
			s.log.ErrorContext(r.Context(), "looking up a player for the api failed", "error", err)
			s.apiRefuse(w, r, http.StatusInternalServerError, "the result could not be stored")
			return
		}
	}

	playedAt := time.Now()
	if body.PlayedAt != nil {
		playedAt = *body.PlayedAt
	}

	sets := make([]domain.MatchSet, 0, len(result.Sets))
	for i, set := range result.Sets {
		sets = append(sets, domain.MatchSet{SetNo: i + 1, HomePoints: set.Home, AwayPoints: set.Away})
	}

	created, err := s.store.Matches().Create(r.Context(), domain.Match{
		HomeID:      body.HomeID,
		AwayID:      body.AwayID,
		BestOf:      result.Mode.BestOf,
		PointsToWin: result.Mode.PointsToWin,
		// Pending, not settled. The reporter is a third party, so scoring.load
		// leaves BOTH players able to confirm — the only path in this
		// application where that is true.
		Status:     domain.MatchPending,
		ReportedBy: body.OperatorID,
		PlayedAt:   playedAt,
		EnteredVia: domain.EnteredViaScoreboard,
		// A tournament game has a place in a draw, and who hands that out is
		// its own question. docs/adr/0015 postpones it by name.
		Sets: sets,
	})
	if errors.Is(err, domain.ErrObserver) {
		// The list this application hands out has no observers in it, so a
		// sender that names one holds a list from before the flag.
		s.apiRefuse(w, r, http.StatusUnprocessableEntity, "an observer cannot be one of the two players")
		return
	}
	if err != nil {
		s.log.ErrorContext(r.Context(), "storing a scoreboard result failed", "error", err)
		s.apiRefuse(w, r, http.StatusInternalServerError, "the result could not be stored")
		return
	}

	s.log.InfoContext(r.Context(), "scoreboard result recorded",
		"match_id", created.ID, "operator_id", body.OperatorID)

	s.writeJSON(w, r, http.StatusCreated, struct {
		MatchID uuid.UUID          `json:"match_id"`
		Status  domain.MatchStatus `json:"status"`
	}{MatchID: created.ID, Status: created.Status})
}

// toMatchResult turns the wire shape into what match.Validate checks, and
// returns the reason when it cannot.
//
// The rules live in internal/match and are not restated here: this surface
// gets the same arithmetic as the form on the start page, so a result the
// Zählwerk would accept and this would not is a disagreement about table
// tennis rather than about JSON.
func (b apiResult) toMatchResult() (match.Result, string) {
	if len(b.Sets) == 0 {
		return match.Result{}, "sets must not be empty"
	}
	if b.BestOf <= 0 || b.PointsToWin <= 0 {
		return match.Result{}, "best_of and points_to_win are required"
	}

	sets := make([]match.Set, 0, len(b.Sets))
	for _, s := range b.Sets {
		sets = append(sets, match.Set{Home: s[0], Away: s[1]})
	}
	result := match.Result{
		Mode: match.Mode{BestOf: b.BestOf, PointsToWin: b.PointsToWin},
		Sets: sets,
	}
	if _, err := match.Validate(result); err != nil {
		return match.Result{}, describeRejection(err)
	}
	return result, ""
}
