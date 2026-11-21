package server

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/counter"
	"github.com/stuttgart-things/schmetterpause/internal/match"
	"github.com/stuttgart-things/schmetterpause/internal/templates"
)

// counterPrefix is what a prefix may look like: the set rows' prefixes are
// "kiosk" and "t<round>-<uuid>". Anything else did not come from a page this
// server drew, and it ends up in an id and a CSS selector.
var counterPrefix = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)

// handleKioskCounter is one tap on the kiosk counter (docs/adr/0034).
//
// It keeps nothing. The options and the events so far come with the request,
// the counter package replays them with the new one, and the answer is the
// counter drawn again with the longer list in it. That is the whole of the
// state, which is why there is no table, no cache and no Redis behind this
// (CLAUDE.md, invariant 3), and why it works the same behind several replicas.
//
// It writes nothing either. When the match is over, the set rows of the same
// form come back filled, out of band, and the button that was always there
// enters the result through the handler that always did.
func (s *Server) handleKioskCounter(w http.ResponseWriter, r *http.Request) {
	// Unlocked is enough: counting is not a write, so the operator rule
	// (issue #90) applies when the result is entered, not here.
	if !s.kioskUnlocked(r) {
		http.Error(w, "Zugang nötig", http.StatusForbidden)
		return
	}

	prefix := r.FormValue("counter_prefix")
	if !counterPrefix.MatchString(prefix) {
		http.Error(w, "Unbekannter Zähler", http.StatusBadRequest)
		return
	}

	tournament := r.FormValue("counter_tournament") == "1"
	home, away := s.counterNames(r)
	v := templates.NewCounterView(prefix, tournament, home, away)

	action := r.FormValue("counter_action")
	started := r.FormValue("counter_state") == "on"

	switch {
	case action == "reset":
		s.render(w, r, templates.Counter(v))
		return
	case action == "start":
		// The mode is frozen here, from the form it sits in: a select
		// changed halfway through a match must not rewrite the sets
		// already played.
		mode := modeFrom(r)
		v.BestOf, v.PointsToWin = counterMode(mode.BestOf, mode.PointsToWin)
	case started:
		bestOf, _ := strconv.Atoi(r.FormValue("counter_best_of"))
		points, _ := strconv.Atoi(r.FormValue("counter_points"))
		v.BestOf, v.PointsToWin = counterMode(bestOf, points)
	default:
		// A tap on a counter that has not started: draw the questions.
		s.render(w, r, templates.Counter(v))
		return
	}

	v.Started = true
	v.First = counterFirst(r.FormValue("counter_first"), tournament)
	v.Warmup = r.FormValue("counter_warmup") != ""
	v.ChangeEnds = r.FormValue("counter_ends") != ""

	opts := counter.Options{
		BestOf:      v.BestOf,
		PointsToWin: v.PointsToWin,
		FirstServer: firstServer(v.First),
		Warmup:      v.Warmup,
		ChangeEnds:  v.ChangeEnds,
	}

	var events []counter.Event
	if action != "start" {
		events = []counter.Event(r.FormValue("counter_log"))
	}
	// Replayed once before the new event, so undo drops the last event that
	// counted rather than the last byte somebody sent.
	_, events = counter.Replay(opts, events)
	switch action {
	case "a":
		events = append(events, counter.PointA)
	case "b":
		events = append(events, counter.PointB)
	case "go":
		events = append(events, counter.Go)
	case "undo":
		if len(events) > 0 {
			events = events[:len(events)-1]
		}
	}
	v.State, events = counter.Replay(opts, events)
	v.Log = string(events)

	if v.Complete() {
		s.render(w, r, templates.CounterDone(v))
		return
	}
	s.render(w, r, templates.Counter(v))
}

// counterNames heads the two sides with the players the form names, or with
// the form's own labels while nobody is picked.
func (s *Server) counterNames(r *http.Request) (string, string) {
	name := func(field, fallback string) string {
		id, err := uuid.Parse(strings.TrimSpace(r.FormValue(field)))
		if err != nil {
			return fallback
		}
		p, err := s.store.Players().ByID(r.Context(), id)
		if err != nil {
			return fallback
		}
		return p.DisplayName
	}
	return name("home_id", templates.SideHome), name("away_id", templates.SideAway)
}

// counterMode keeps the mode to what the forms offer. The counter would play
// anything, but a set to a thousand did not come from a select on this site.
func counterMode(bestOf, pointsToWin int) (int, int) {
	mode := match.Mode{BestOf: bestOf, PointsToWin: pointsToWin}
	if !mode.Known() {
		return match.DefaultBestOf, match.PointsToEleven
	}
	return bestOf, pointsToWin
}

// counterFirst is who serves first. In a tournament it is side A whatever
// the request says (docs/adr/0013).
func counterFirst(raw string, tournament bool) string {
	if tournament {
		return templates.CounterFirstA
	}
	switch raw {
	case templates.CounterFirstA, templates.CounterFirstB:
		return raw
	default:
		return templates.CounterFirstRally
	}
}

func firstServer(first string) counter.Player {
	switch first {
	case templates.CounterFirstA:
		return counter.A
	case templates.CounterFirstB:
		return counter.B
	default:
		return counter.Nobody
	}
}
