// Package counter keeps the score of one match point by point, for the kiosk
// when no Zählwerk is at the table (docs/adr/0034).
//
// It holds no state of its own. A match is its options and the list of what
// happened, and Replay plays that list from the start every time. That is what
// lets the server stay stateless — the list rides in the page — and what makes
// undo nothing more than dropping the last event, across a set or a change of
// ends as easily as within one.
//
// The rules are the ones on the rules sheet (internal/templates/rules.templ):
// the service changes every ServeEvery points and after every point from
// DeuceFrom all, the loser of a set opens the next one, ends change after every
// set and once more in the deciding set. The Zählwerk keeps the same rules in
// its own repository; the duplication is the price ADR-0034 accepts.
//
// No database, no HTTP, plain values only (CLAUDE.md, "Konventionen").
package counter

// Player is one of the two, by the side they started on: A is home, the left
// column of the entry form, and in a tournament side A (docs/adr/0013).
type Player int

const (
	// Nobody is the zero value: no server named yet, no winner yet.
	Nobody Player = iota
	A
	B
)

// Other is the opponent.
func (p Player) Other() Player {
	switch p {
	case A:
		return B
	case B:
		return A
	default:
		return Nobody
	}
}

// Phase is where the match stands. Only PhasePlaying scores points; in the
// others an event means something else or nothing at all.
type Phase string

const (
	// PhaseWarmup waits for the umpire to say the knock-up is over.
	PhaseWarmup Phase = "warmup"
	// PhaseServeRally waits for the rally that decides the first serve
	// ("umdi"). Its winner serves first and scores nothing.
	PhaseServeRally Phase = "serve_rally"
	PhasePlaying    Phase = "playing"
	// PhaseChangeEnds waits for the umpire to say both are at their new end.
	PhaseChangeEnds Phase = "change_ends"
	PhaseComplete   Phase = "complete"
)

// Event is one thing that happened at the table. A byte, so a whole match
// fits in a hidden form field as a short string.
type Event byte

const (
	// PointA and PointB are a point to that player in PhasePlaying, and the
	// serve rally won by that player in PhaseServeRally.
	PointA Event = 'a'
	PointB Event = 'b'
	// Go ends the warm-up and the change of ends alike: "play on".
	Go Event = 'g'
)

// Options are what is asked before the first point.
type Options struct {
	BestOf      int
	PointsToWin int
	// FirstServer is who serves the first set. Nobody means it is played for
	// in a serve rally.
	FirstServer Player
	// Warmup starts the match with a knock-up nothing counts in.
	Warmup bool
	// ChangeEnds swaps the players' ends after every set and at half the
	// target in the deciding set, and pauses until the umpire says go.
	ChangeEnds bool
}

// MaxEvents bounds a match. A best of seven to twenty-one with long deuces is
// a few hundred points; anything far past that did not come from a table.
const MaxEvents = 2000

// State is a match after some events.
type State struct {
	Phase Phase
	// Sets are the finished sets, home score first.
	Sets [][2]int
	// Score is the set being played, A first.
	Score [2]int
	// SetsWon counts the finished sets per player, A first.
	SetsWon [2]int
	// Serving is who serves the next point, Nobody before it is decided.
	Serving Player
	// ServesLeft is how many serves Serving has before the service changes,
	// counting the next one.
	ServesLeft int
	// Swapped says A stands on the right. Derived from the sets played and the
	// deciding-set change, never stored.
	Swapped bool
	// Winner is set once the match is over.
	Winner Player

	opener       Player // who served first in the current set
	decidingSwap bool   // the deciding set's mid-set change has happened
}

// Replay plays events from the start and returns where the match stands, and
// the events that were accepted. An event that means nothing in its phase — a
// point during the warm-up, anything after the match is over — is left out,
// so the list handed back is the one to carry forward and to undo from.
func Replay(o Options, events []Event) (State, []Event) {
	o = o.normalised()
	s := State{Phase: firstPhase(o), Serving: o.FirstServer, opener: o.FirstServer}
	s.serve(o)

	kept := make([]Event, 0, len(events))
	for i, e := range events {
		if i >= MaxEvents {
			break
		}
		if s.apply(o, e) {
			kept = append(kept, e)
		}
	}
	return s, kept
}

func (o Options) normalised() Options {
	if o.BestOf < 1 || o.BestOf%2 == 0 {
		o.BestOf = 1
	}
	if o.PointsToWin < 1 {
		o.PointsToWin = 11
	}
	if o.FirstServer != A && o.FirstServer != B {
		o.FirstServer = Nobody
	}
	return o
}

func firstPhase(o Options) Phase {
	switch {
	case o.Warmup:
		return PhaseWarmup
	case o.FirstServer == Nobody:
		return PhaseServeRally
	default:
		return PhasePlaying
	}
}

// apply moves the state on by one event and reports whether it meant
// anything.
func (s *State) apply(o Options, e Event) bool {
	switch s.Phase {
	case PhaseWarmup:
		if e != Go {
			return false
		}
		if o.FirstServer == Nobody {
			s.Phase = PhaseServeRally
		} else {
			s.Phase = PhasePlaying
		}
		return true

	case PhaseServeRally:
		p, ok := pointFor(e)
		if !ok {
			return false
		}
		s.opener = p
		s.Phase = PhasePlaying
		s.serve(o)
		return true

	case PhaseChangeEnds:
		if e != Go {
			return false
		}
		s.Phase = PhasePlaying
		return true

	case PhasePlaying:
		p, ok := pointFor(e)
		if !ok {
			return false
		}
		s.point(o, p)
		return true

	default:
		return false
	}
}

func pointFor(e Event) (Player, bool) {
	switch e {
	case PointA:
		return A, true
	case PointB:
		return B, true
	default:
		return Nobody, false
	}
}

func (s *State) point(o Options, p Player) {
	s.Score[p-1]++

	if winner := setWinner(s.Score, o.PointsToWin); winner != Nobody {
		s.Sets = append(s.Sets, s.Score)
		s.SetsWon[winner-1]++
		s.Score = [2]int{}
		s.decidingSwap = false

		if s.SetsWon[winner-1] > o.BestOf/2 {
			s.Winner = winner
			s.Phase = PhaseComplete
			s.Serving = Nobody
			s.ServesLeft = 0
			return
		}
		// The house rule on the rules sheet: whoever lost the set opens
		// the next one.
		s.opener = winner.Other()
		s.serve(o)
		if o.ChangeEnds {
			s.Swapped = !s.Swapped
			s.Phase = PhaseChangeEnds
		}
		return
	}

	s.serve(o)
	// ITTF 2.14: in the last possible set the ends change again when one
	// player first reaches half the target — five in a set to eleven.
	if o.ChangeEnds && !s.decidingSwap && len(s.Sets) == o.BestOf-1 &&
		max(s.Score[0], s.Score[1]) >= o.PointsToWin/2 {
		s.decidingSwap = true
		s.Swapped = !s.Swapped
		s.Phase = PhaseChangeEnds
	}
}

// setWinner is who has won the set at this score, or Nobody while it runs:
// the target reached with two points to spare.
func setWinner(score [2]int, pointsToWin int) Player {
	a, b := score[0], score[1]
	switch {
	case a >= pointsToWin && a-b >= 2:
		return A
	case b >= pointsToWin && b-a >= 2:
		return B
	default:
		return Nobody
	}
}

// serve works out who serves next and how many serves they have left, from
// the set's opener and the points played in it.
func (s *State) serve(o Options) {
	if s.opener == Nobody {
		s.Serving, s.ServesLeft = Nobody, 0
		return
	}
	server, left := Server(s.opener, s.Score[0]+s.Score[1], o.PointsToWin)
	s.Serving, s.ServesLeft = server, left
}

// Server is who serves after played points of a set the opener began, and how
// many serves in a row they have left counting this one.
//
// Blocks of ServeEvery serves alternate until both reach DeuceFrom; from there
// the service changes after every point. At eleven, 10:10 is twenty points,
// ten blocks of two, so the opener serves at deuce; at twenty-one, 20:20 is
// forty, eight blocks of five, the same.
func Server(opener Player, played, pointsToWin int) (Player, int) {
	every := ServeEvery(pointsToWin)
	deuce := 2 * DeuceFrom(pointsToWin)

	var turn, left int
	if played < deuce {
		turn = played / every
		left = every - played%every
	} else {
		turn = deuce/every + (played - deuce)
		left = 1
	}
	if turn%2 == 0 {
		return opener, left
	}
	return opener.Other(), left
}

// ServeEvery is how many points a player serves in a row before the service
// changes: two in a set to eleven, five in a set to twenty-one.
//
// Not a constant, because the form offers both modes and the answer is
// different in each. "Always two" is the rule most people know, and it is the
// wrong one for the twenty-one mode the picker also offers.
func ServeEvery(pointsToWin int) int {
	if pointsToWin >= 21 {
		return 5
	}
	return 2
}

// DeuceFrom is the score from which a set no longer ends at the target, and
// from which the service changes after every point.
func DeuceFrom(pointsToWin int) int { return pointsToWin - 1 }
