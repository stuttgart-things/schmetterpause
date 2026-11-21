package templates

import (
	"strconv"

	"github.com/stuttgart-things/schmetterpause/internal/counter"
)

// The values of counter_first, the question who serves first.
const (
	CounterFirstA     = "a"
	CounterFirstB     = "b"
	CounterFirstRally = "rally"
)

// CounterView is the point-by-point counter at the kiosk (docs/adr/0034).
//
// Everything it knows travels in the page: the options asked before the first
// point and the events since. The server replays them on every tap and draws
// this again, so it holds nothing between two points.
type CounterView struct {
	// Prefix is the one the set rows of the same form carry, so the counter
	// can fill exactly those boxes when the match is over.
	Prefix string
	// Tournament fixes the first serve on side A (docs/adr/0013) and drops
	// the question.
	Tournament bool
	// HomeName and AwayName head the two sides. The form's labels when no
	// players are picked yet.
	HomeName, AwayName string

	// Started is false while the questions are still being answered.
	Started     bool
	First       string
	Warmup      bool
	ChangeEnds  bool
	BestOf      int
	PointsToWin int
	// Log is the accepted events, as a string of their bytes.
	Log   string
	State counter.State
}

// CounterSideView is one of the two big buttons.
type CounterSideView struct {
	// Action is what a tap sends: "a" or "b".
	Action     string
	Name       string
	Points     int
	Sets       int
	Serving    bool
	ServesLeft int
}

// ID is the element the counter swaps itself into.
func (v CounterView) ID() string { return "counter-" + v.Prefix }

// Sides are the two players in the order they stand at the table, left
// first. With a change of ends the buttons move with the players, so the
// umpire taps the side the point was won on.
func (v CounterView) Sides() []CounterSideView {
	s := v.State
	a := CounterSideView{
		Action: string(counter.PointA), Name: v.HomeName,
		Points: s.Score[0], Sets: s.SetsWon[0],
		Serving: s.Serving == counter.A, ServesLeft: s.ServesLeft,
	}
	b := CounterSideView{
		Action: string(counter.PointB), Name: v.AwayName,
		Points: s.Score[1], Sets: s.SetsWon[1],
		Serving: s.Serving == counter.B, ServesLeft: s.ServesLeft,
	}
	if s.Swapped {
		return []CounterSideView{b, a}
	}
	return []CounterSideView{a, b}
}

// Playing reports whether a tap on a side is a point.
func (v CounterView) Playing() bool { return v.State.Phase == counter.PhasePlaying }

// Rally reports whether a tap on a side decides the first serve.
func (v CounterView) Rally() bool { return v.State.Phase == counter.PhaseServeRally }

// Paused reports whether the counter waits for "go": after the warm-up and at
// a change of ends.
func (v CounterView) Paused() bool {
	return v.State.Phase == counter.PhaseWarmup || v.State.Phase == counter.PhaseChangeEnds
}

// Complete reports whether the match is over.
func (v CounterView) Complete() bool { return v.State.Phase == counter.PhaseComplete }

// GoLabel names the button that ends a pause.
func (v CounterView) GoLabel() string {
	if v.State.Phase == counter.PhaseWarmup {
		return "Los"
	}
	return "Weiter"
}

// Banner is the one line that says what the table is waiting for. German:
// the umpire reads it.
func (v CounterView) Banner() string {
	s := v.State
	switch s.Phase {
	case counter.PhaseWarmup:
		return "Einspielen. Nichts zählt, bis du auf „Los“ tippst."
	case counter.PhaseServeRally:
		return "Um die Angabe: Wer hat den Ballwechsel gewonnen?"
	case counter.PhaseChangeEnds:
		return "Seitenwechsel. Weiter, wenn beide stehen."
	case counter.PhaseComplete:
		won, lost := s.SetsWon[0], s.SetsWon[1]
		name := v.HomeName
		if s.Winner == counter.B {
			won, lost = lost, won
			name = v.AwayName
		}
		return name + " gewinnt " + strconv.Itoa(won) + ":" + strconv.Itoa(lost) +
			". Die Sätze stehen unten. Prüfen, dann eintragen."
	default:
		name := v.HomeName
		if s.Serving == counter.B {
			name = v.AwayName
		}
		return "Aufschlag " + name + ", noch " + strconv.Itoa(s.ServesLeft)
	}
}

// History is the finished sets, home first, the way the set rows write them.
func (v CounterView) History() []string {
	out := make([]string, 0, len(v.State.Sets))
	for _, set := range v.State.Sets {
		out = append(out, strconv.Itoa(set[0])+":"+strconv.Itoa(set[1]))
	}
	return out
}

// FilledSets are the finished sets as entry rows, for the boxes the counter
// fills once the match is over. The rest stay empty, which is 0:0.
func (v CounterView) FilledSets() []SetInput {
	sets := EmptySets()
	for i, set := range v.State.Sets {
		if i >= len(sets) {
			break
		}
		sets[i] = SetInput{Home: strconv.Itoa(set[0]), Away: strconv.Itoa(set[1])}
	}
	return sets
}

// CanUndo reports whether there is an event to take back.
func (v CounterView) CanUndo() bool { return v.Log != "" }

// NewCounterView is a counter that has not started, with the defaults the
// rules sheet implies: the first serve played for outside a tournament, ends
// changed after every set.
func NewCounterView(prefix string, tournament bool, homeName, awayName string) CounterView {
	first := CounterFirstRally
	if tournament {
		first = CounterFirstA
	}
	return CounterView{
		Prefix: prefix, Tournament: tournament,
		HomeName: homeName, AwayName: awayName,
		First: first, ChangeEnds: true,
	}
}

// pairingPrefix keeps every id on a tournament page unique: twenty-eight
// pairings share one template, and two fields with one id is a label that
// points at the wrong box.
func pairingPrefix(p TournamentPairingView) string {
	return "t" + strconv.Itoa(p.Round) + "-" + p.HomeID
}
