package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/checkin"
	"github.com/stuttgart-things/schmetterpause/internal/config"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/match"
	"github.com/stuttgart-things/schmetterpause/internal/repository"
	"github.com/stuttgart-things/schmetterpause/internal/scoring"
	"github.com/stuttgart-things/schmetterpause/internal/tournament"
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
// Players and operators out, one finished result in (with an operator or with
// two check-ins, docs/adr/0024), and one question about a fob
// (docs/adr/0025). Open tournaments and their open places out, so a result
// can fill a place in a draw (docs/adr/0028). No updates, no deletes, no
// listing of matches, and nothing that lets the caller change a rating.

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

// apiOperator is one entry of GET /api/operators: somebody who may keep score.
// No TTR: the list answers who may watch and count, not how anybody plays.
type apiOperator struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
	Observer    bool      `json:"observer"`
}

// apiResult is the body of POST /api/results.
//
// Sets are pairs rather than objects so that the sender can pass its own
// scorer.State.CompletedSets ([][2]int) through unchanged — it is the field
// that exists over there for exactly this handover. Home first, away second,
// in the order they were played.
//
// Exactly one of two proofs comes with it: an operator who watched
// (docs/adr/0015), or the check-ins of both players (docs/adr/0024). The ids
// of those check-ins are the ones POST /api/identities/rfid/resolve handed
// out, so the sender passes back what it was given and asserts nothing.
type apiResult struct {
	HomeID        uuid.UUID `json:"home_id"`
	AwayID        uuid.UUID `json:"away_id"`
	OperatorID    uuid.UUID `json:"operator_id"`
	HomeCheckinID uuid.UUID `json:"home_checkin_id"`
	AwayCheckinID uuid.UUID `json:"away_checkin_id"`
	Sets          [][2]int  `json:"sets"`
	BestOf        int       `json:"best_of"`
	PointsToWin   int       `json:"points_to_win"`
	// PlayedAt is optional. The Zählwerk knows when the match ended; if it
	// says nothing, the moment the result arrives is close enough.
	PlayedAt *time.Time `json:"played_at,omitempty"`
	// TournamentID and Round name the place in a draw this result fills,
	// both or neither (docs/adr/0028). The Zählwerk got them from
	// GET /api/tournaments/{id}/pairings and passes them back unchecked; the
	// check is here.
	TournamentID *uuid.UUID `json:"tournament_id,omitempty"`
	Round        *int       `json:"round,omitempty"`
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

// handleAPIOperators lists everybody who may keep score for a result.
//
// That is every account, observers included: an observer does not play, and
// somebody who watches and counts is exactly what an operator is (ADR-0014,
// ADR-0022). /api/players stays the list of the two sides, which is why this is
// a second endpoint rather than a flag on that one -- a Zählwerk that does not
// know the flag would offer an observer as a player (docs/adr/0023).
//
// Whether the operator also plays in the match is still decided where it can
// be: in POST /api/results, against the two sides actually sent.
func (s *Server) handleAPIOperators(w http.ResponseWriter, r *http.Request) {
	if !s.scoreboardAuthorized(r) {
		s.apiRefuse(w, r, http.StatusUnauthorized, "a bearer token is required")
		return
	}

	players, err := s.store.Players().List(r.Context())
	if err != nil {
		s.log.ErrorContext(r.Context(), "listing operators for the api failed", "error", err)
		s.apiRefuse(w, r, http.StatusInternalServerError, "the operator list is not available")
		return
	}

	out := make([]apiOperator, 0, len(players))
	for _, p := range players {
		out = append(out, apiOperator{ID: p.ID, DisplayName: p.DisplayName, Observer: p.IsObserver})
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

	tour, status, msg := s.apiTournamentOf(r.Context(), body, result.Mode)
	if msg != "" {
		s.apiRefuse(w, r, status, msg)
		return
	}

	checkedIn := body.HomeCheckinID != uuid.Nil || body.AwayCheckinID != uuid.Nil
	if checkedIn && body.OperatorID != uuid.Nil {
		// Two proofs is a sender that has not decided which path it is on,
		// and taking either one would be guessing.
		s.apiRefuse(w, r, http.StatusBadRequest,
			"send operator_id or home_checkin_id and away_checkin_id, not both")
		return
	}
	if checkedIn {
		if body.HomeCheckinID == uuid.Nil || body.AwayCheckinID == uuid.Nil {
			s.apiRefuse(w, r, http.StatusBadRequest,
				"home_checkin_id and away_checkin_id are both required")
			return
		}
		if body.HomeCheckinID == body.AwayCheckinID {
			s.apiRefuse(w, r, http.StatusBadRequest,
				"home_checkin_id and away_checkin_id must differ")
			return
		}
		s.recordCheckedInResult(w, r, body, result, tour)
		return
	}

	// Without check-ins the operator is required, and may not be playing.
	// docs/adr/0014 gives the reason and docs/adr/0015 keeps it unchanged: a
	// machine has no identity, so it names the person standing there who
	// watched the match. Without that, a result has nobody behind it and both
	// players could confirm a match neither of them reported. docs/adr/0024
	// lifts this only where two check-ins take the operator's place.
	if body.OperatorID == uuid.Nil {
		s.apiRefuse(w, r, http.StatusBadRequest,
			"operator_id is required, or home_checkin_id and away_checkin_id")
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

	playedAt, sets := body.playedAt(), matchSets(result)

	var created domain.Match
	err := s.store.InTx(r.Context(), func(tx repository.Store) error {
		if err := checkAPIPlace(r.Context(), tx, tour, body); err != nil {
			return err
		}
		var err error
		created, err = tx.Matches().Create(r.Context(), domain.Match{
			HomeID:      body.HomeID,
			AwayID:      body.AwayID,
			BestOf:      result.Mode.BestOf,
			PointsToWin: result.Mode.PointsToWin,
			// Pending, not settled, unless this deployment counts the
			// Zählwerk's results at once (below). The reporter is a third
			// party, so scoring.load leaves BOTH players able to confirm —
			// the only path in this application where that is true.
			Status:     domain.MatchPending,
			ReportedBy: body.OperatorID,
			PlayedAt:   playedAt,
			EnteredVia: domain.EnteredViaScoreboard,
			// The place in a draw, when the Zählwerk named one. This
			// application handed it out and checked it above
			// (docs/adr/0028).
			TournamentID:    body.TournamentID,
			TournamentRound: body.Round,
			Sets:            sets,
		})
		if err != nil {
			return err
		}
		created, err = s.settleScoreboard(r.Context(), tx, created)
		return err
	})
	if errors.Is(err, domain.ErrObserver) {
		// The list this application hands out has no observers in it, so a
		// sender that names one holds a list from before the flag.
		s.apiRefuse(w, r, http.StatusUnprocessableEntity, "an observer cannot be one of the two players")
		return
	}
	if status, msg, ok := placeRefusal(err); ok {
		s.apiRefuse(w, r, status, msg)
		return
	}
	if err != nil {
		s.log.ErrorContext(r.Context(), "storing a scoreboard result failed", "error", err)
		s.apiRefuse(w, r, http.StatusInternalServerError, "the result could not be stored")
		return
	}

	s.log.InfoContext(r.Context(), "scoreboard result recorded", append([]any{
		"match_id", created.ID, "operator_id", body.OperatorID, "status", created.Status,
	}, body.placeAttrs()...)...)

	s.writeJSON(w, r, http.StatusCreated, struct {
		MatchID uuid.UUID          `json:"match_id"`
		Status  domain.MatchStatus `json:"status"`
	}{MatchID: created.ID, Status: created.Status})
}

// recordCheckedInResult stores a result whose proof is two check-ins
// (docs/adr/0024). The handler has already checked the body's shape: both ids
// are there, they differ, and there is no operator.
//
// The check-ins are read and the match written in one transaction, with the
// check-in rows locked. Otherwise two copies of the same result — a sender
// retrying after a lost answer — could both find the check-ins unused.
func (s *Server) recordCheckedInResult(w http.ResponseWriter, r *http.Request,
	body apiResult, result match.Result, tour *domain.Tournament,
) {
	var (
		created domain.Match
		missing uuid.UUID
	)
	err := s.store.InTx(r.Context(), func(tx repository.Store) error {
		found, err := tx.Checkins().Lock(r.Context(), []uuid.UUID{body.HomeCheckinID, body.AwayCheckinID})
		if err != nil {
			return err
		}
		for _, id := range []uuid.UUID{body.HomeCheckinID, body.AwayCheckinID} {
			if _, ok := found[id]; !ok {
				missing = id
				return domain.ErrNotFound
			}
		}
		if err := checkin.Verify(found[body.HomeCheckinID], found[body.AwayCheckinID],
			body.HomeID, body.AwayID, time.Now()); err != nil {
			return err
		}
		if err := checkAPIPlace(r.Context(), tx, tour, body); err != nil {
			return err
		}

		created, err = tx.Matches().Create(r.Context(), domain.Match{
			HomeID:      body.HomeID,
			AwayID:      body.AwayID,
			BestOf:      result.Mode.BestOf,
			PointsToWin: result.Mode.PointsToWin,
			// Pending, as on the operator path (docs/adr/0015). Nobody
			// reported it, so scoring.load lets either player confirm.
			// Counting it at once is the auto-accept question, which
			// docs/adr/0024 leaves to its own decision; a deployment can
			// answer it for itself with SP_SCOREBOARD_SETTLE (docs/adr/0027).
			Status:          domain.MatchPending,
			ReportedBy:      uuid.Nil,
			PlayedAt:        body.playedAt(),
			EnteredVia:      domain.EnteredViaCheckin,
			HomeCheckinID:   &body.HomeCheckinID,
			AwayCheckinID:   &body.AwayCheckinID,
			TournamentID:    body.TournamentID,
			TournamentRound: body.Round,
			Sets:            matchSets(result),
		})
		if err != nil {
			return err
		}
		created, err = s.settleScoreboard(r.Context(), tx, created)
		return err
	})

	if status, msg, ok := placeRefusal(err); ok {
		s.apiRefuse(w, r, status, msg)
		return
	}

	var rejection *checkin.Rejection
	switch {
	case errors.As(err, &rejection) && rejection.Kind == checkin.Used,
		errors.Is(err, domain.ErrConflict):
		// Conflict rather than unprocessable: the likeliest sender is the
		// Zählwerk retrying a result that did arrive, and 409 tells it so.
		s.apiRefuse(w, r, http.StatusConflict, "a check-in already carries a result")
		return
	case errors.As(err, &rejection):
		s.apiRefuse(w, r, http.StatusUnprocessableEntity, rejection.Error())
		return
	case errors.Is(err, domain.ErrNotFound):
		// The id, like the unknown player above: the sender got it from
		// this application, so naming it leaks nothing and says what is
		// stale.
		s.apiRefuse(w, r, http.StatusUnprocessableEntity, fmt.Sprintf("no check-in with id %s", missing))
		return
	case errors.Is(err, domain.ErrObserver):
		s.apiRefuse(w, r, http.StatusUnprocessableEntity, "an observer cannot be one of the two players")
		return
	case err != nil:
		s.log.ErrorContext(r.Context(), "storing a checked-in result failed", "error", err)
		s.apiRefuse(w, r, http.StatusInternalServerError, "the result could not be stored")
		return
	}

	s.log.InfoContext(r.Context(), "checked-in result recorded", append([]any{
		"match_id", created.ID, "status", created.Status,
		"home_checkin_id", body.HomeCheckinID, "away_checkin_id", body.AwayCheckinID,
	}, body.placeAttrs()...)...)

	s.writeJSON(w, r, http.StatusCreated, struct {
		MatchID uuid.UUID          `json:"match_id"`
		Status  domain.MatchStatus `json:"status"`
	}{MatchID: created.ID, Status: created.Status})
}

// settleScoreboard counts a result from the Zählwerk at once when this
// deployment says so (SP_SCOREBOARD_SETTLE=immediate, docs/adr/0027), inside
// the transaction that created it, so a failure leaves no match behind that is
// neither pending nor rated. Otherwise it leaves the match pending, which is
// docs/adr/0015.
//
// Only the timing changes: entered_via and reported_by are what the path wrote,
// so the row still says it came from the Zählwerk and who, if anybody, watched.
func (s *Server) settleScoreboard(ctx context.Context, tx repository.Store, m domain.Match) (domain.Match, error) {
	if s.cfg.ScoreboardSettle != config.SettleImmediate {
		return m, nil
	}
	settlement, err := scoring.SettleRecorded(ctx, tx, m, time.Now())
	if err != nil {
		return domain.Match{}, fmt.Errorf("settle the scoreboard result at once: %w", err)
	}
	return settlement.Match, nil
}

// apiTournamentOf is the tournament a result names, or nil when it names none.
// Everything about the place that does not depend on what is booked is
// checked here, before a transaction is opened; the place itself is checked
// inside it, by checkAPIPlace.
//
// The mode is compared, not overwritten as at the kiosk. There the mode is a
// form field somebody can leave wrong; here it is what the scorer counted by,
// and a result in another mode is another game than this draw is played in.
func (s *Server) apiTournamentOf(ctx context.Context, body apiResult, mode match.Mode) (
	*domain.Tournament, int, string,
) {
	if (body.TournamentID == nil) != (body.Round == nil) {
		return nil, http.StatusBadRequest, "tournament_id and round go together: send both or neither"
	}
	if body.TournamentID == nil {
		return nil, 0, ""
	}
	if *body.Round < 1 {
		return nil, http.StatusBadRequest, "round counts from 1"
	}

	tour, err := s.store.Tournaments().ByID(ctx, *body.TournamentID)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return nil, http.StatusUnprocessableEntity, fmt.Sprintf("no tournament with id %s", *body.TournamentID)
	case err != nil:
		s.log.ErrorContext(ctx, "loading a tournament for the api failed", "error", err)
		return nil, http.StatusInternalServerError, "the result could not be stored"
	case !tour.Open():
		return nil, http.StatusUnprocessableEntity, "the tournament is closed"
	case !slices.Contains(tour.Players, body.HomeID) || !slices.Contains(tour.Players, body.AwayID):
		return nil, http.StatusUnprocessableEntity, "these two do not play in this tournament"
	case mode.BestOf != tour.BestOf || mode.PointsToWin != tour.PointsToWin:
		return nil, http.StatusUnprocessableEntity, fmt.Sprintf(
			"this tournament is played best of %d to %d", tour.BestOf, tour.PointsToWin)
	}
	return &tour, 0, ""
}

// checkAPIPlace checks the place a result claims, against what the
// transaction sees booked. The same tournament.CheckPlace as the kiosk and the
// phone, so the Zählwerk cannot fill a place they could not.
func checkAPIPlace(ctx context.Context, tx repository.Store, tour *domain.Tournament, body apiResult) error {
	if tour == nil {
		return nil
	}
	return checkPlace(ctx, tx, *tour, body.HomeID, body.AwayID, *body.Round)
}

// placeRefusal is how a refused place reads on this surface. A filled place is
// a conflict: the likeliest sender is the Zählwerk retrying a result that did
// arrive, or a place entered at the kiosk meanwhile. Anything else about the
// place is a request that cannot be right.
func placeRefusal(err error) (int, string, bool) {
	if errors.Is(err, tournament.ErrPlaceFilled) {
		return http.StatusConflict, tournament.ErrPlaceFilled.Error(), true
	}
	for _, known := range []error{tournament.ErrFinalUndecided, tournament.ErrNotTheFinalists, tournament.ErrNotInDraw} {
		if errors.Is(err, known) {
			return http.StatusUnprocessableEntity, known.Error(), true
		}
	}
	return 0, "", false
}

// placeAttrs is the place a result filled, for its log line: nothing for a
// result outside a tournament, and the values rather than the pointers the
// body holds them in.
func (b apiResult) placeAttrs() []any {
	if b.TournamentID == nil || b.Round == nil {
		return nil
	}
	return []any{"tournament_id", *b.TournamentID, "round", *b.Round}
}

// playedAt is when the sender says the match ended, or now.
func (b apiResult) playedAt() time.Time {
	if b.PlayedAt != nil {
		return *b.PlayedAt
	}
	return time.Now()
}

// matchSets numbers the validated sets for storage.
func matchSets(result match.Result) []domain.MatchSet {
	sets := make([]domain.MatchSet, 0, len(result.Sets))
	for i, set := range result.Sets {
		sets = append(sets, domain.MatchSet{SetNo: i + 1, HomePoints: set.Home, AwayPoints: set.Away})
	}
	return sets
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

// apiFobResolve is the body of POST /api/identities/rfid/resolve.
//
// The UID travels in the body, never in the path, so it does not end up in
// anybody's access log (docs/adr/0025).
type apiFobResolve struct {
	UID  string      `json:"uid"`
	Half domain.Half `json:"half"`
}

// handleAPIResolveFob tells the Zählwerk who a fob is, and records that this
// player checked in at that half.
//
// The check-in is created here rather than asserted by the Zählwerk: a result
// that reports without an operator names two of them, and this application
// only believes its own (docs/adr/0024).
//
// An unknown fob outside a link window is a 404 with nothing in it, and
// leaves nothing behind: no row, and no UID or hash in the log. A badge held
// against the reader by mistake has to pass through without a trace.
func (s *Server) handleAPIResolveFob(w http.ResponseWriter, r *http.Request) {
	if !s.scoreboardAuthorized(r) {
		s.apiRefuse(w, r, http.StatusUnauthorized, "a bearer token is required")
		return
	}

	var body apiFobResolve
	dec := json.NewDecoder(io.LimitReader(r.Body, maxAPIBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		// Not err.Error(): a decode error can quote the input, and the input
		// is a card number.
		s.apiRefuse(w, r, http.StatusBadRequest, "the body is not a fob resolve request")
		return
	}
	if !body.Half.Valid() {
		s.apiRefuse(w, r, http.StatusBadRequest, "half must be left or right")
		return
	}

	got, err := s.fobs.Resolve(r.Context(), body.UID, body.Half)
	switch {
	case errors.Is(err, auth.ErrBadUID):
		s.apiRefuse(w, r, http.StatusBadRequest, auth.ErrBadUID.Error())
		return
	case errors.Is(err, auth.ErrUnknownFob):
		s.log.InfoContext(r.Context(), "unknown fob")
		w.WriteHeader(http.StatusNotFound)
		return
	case err != nil:
		s.log.ErrorContext(r.Context(), "resolving a fob failed", "error", err)
		s.apiRefuse(w, r, http.StatusInternalServerError, "the fob could not be resolved")
		return
	}

	if got.Linked {
		s.log.InfoContext(r.Context(), "fob linked", "player_id", got.PlayerID)
	}
	s.log.InfoContext(r.Context(), "fob check-in recorded",
		"player_id", got.PlayerID, "checkin_id", got.CheckinID, "half", body.Half)

	s.writeJSON(w, r, http.StatusOK, struct {
		PlayerID  uuid.UUID `json:"player_id"`
		CheckinID uuid.UUID `json:"checkin_id"`
	}{PlayerID: got.PlayerID, CheckinID: got.CheckinID})
}
