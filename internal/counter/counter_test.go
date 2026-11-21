package counter_test

import (
	"strings"
	"testing"

	"github.com/stuttgart-things/schmetterpause/internal/counter"
)

// play turns a string of 'a', 'b' and 'g' into events, so a match reads the
// way it was played.
func play(o counter.Options, s string) (counter.State, []counter.Event) {
	return counter.Replay(o, []counter.Event(s))
}

func eleven(bestOf int) counter.Options {
	return counter.Options{BestOf: bestOf, PointsToWin: 11, FirstServer: counter.A}
}

// The rules sheet: two serves each at eleven, five at twenty-one, every point
// from deuce on, and the opener serves at deuce in both.
func TestTheServiceFollowsTheRulesSheet(t *testing.T) {
	cases := []struct {
		played, target int
		want           counter.Player
		left           int
	}{
		{0, 11, counter.A, 2},
		{1, 11, counter.A, 1},
		{2, 11, counter.B, 2},
		{3, 11, counter.B, 1},
		{4, 11, counter.A, 2},
		{19, 11, counter.B, 1},
		{20, 11, counter.A, 1}, // 10:10
		{21, 11, counter.B, 1},
		{22, 11, counter.A, 1},
		{0, 21, counter.A, 5},
		{4, 21, counter.A, 1},
		{5, 21, counter.B, 5},
		{40, 21, counter.A, 1}, // 20:20
		{41, 21, counter.B, 1},
	}
	for _, c := range cases {
		got, left := counter.Server(counter.A, c.played, c.target)
		if got != c.want || left != c.left {
			t.Errorf("after %d points to %d: %v with %d left, want %v with %d",
				c.played, c.target, got, left, c.want, c.left)
		}
	}
}

// A set ends at the target with two to spare, and not before.
func TestASetNeedsTwoPointsToSpare(t *testing.T) {
	s, _ := play(eleven(3), strings.Repeat("ab", 10)+"a")
	if len(s.Sets) != 0 || s.Score != [2]int{11, 10} {
		t.Fatalf("11:10 ended the set: sets %v, score %v", s.Sets, s.Score)
	}
	s, _ = play(eleven(3), strings.Repeat("ab", 10)+"aa")
	if len(s.Sets) != 1 || s.Sets[0] != [2]int{12, 10} {
		t.Fatalf("12:10 did not end the set: sets %v, score %v", s.Sets, s.Score)
	}
}

// The house rule: whoever lost a set opens the next one.
func TestTheLoserOfASetOpensTheNext(t *testing.T) {
	// A opened and wins 11:0; B lost and opens set two.
	s, _ := play(eleven(3), strings.Repeat("a", 11))
	if s.Serving != counter.B || s.ServesLeft != 2 {
		t.Errorf("set two opened by %v with %d left, want B with 2", s.Serving, s.ServesLeft)
	}
	// B wins set two; A opens set three.
	s, _ = play(eleven(3), strings.Repeat("a", 11)+strings.Repeat("b", 11))
	if s.Serving != counter.A {
		t.Errorf("set three opened by %v, want A", s.Serving)
	}
}

// A match is over at a majority of the sets, and nothing counts after it.
func TestTheMatchEndsAtAMajority(t *testing.T) {
	s, kept := play(eleven(3), strings.Repeat("a", 22)+"ab")
	if s.Phase != counter.PhaseComplete || s.Winner != counter.A {
		t.Fatalf("phase %v, winner %v, want complete and A", s.Phase, s.Winner)
	}
	if len(kept) != 22 {
		t.Errorf("kept %d events, want the 22 that were played", len(kept))
	}
	if s.SetsWon != [2]int{2, 0} {
		t.Errorf("sets won %v, want 2:0", s.SetsWon)
	}
}

// "Umdi": the rally for the first serve scores nothing and names the opener.
func TestTheServeRallyDecidesWhoServes(t *testing.T) {
	o := eleven(1)
	o.FirstServer = counter.Nobody
	s, _ := play(o, "")
	if s.Phase != counter.PhaseServeRally || s.Serving != counter.Nobody {
		t.Fatalf("start: phase %v, serving %v", s.Phase, s.Serving)
	}
	s, _ = play(o, "b")
	if s.Phase != counter.PhasePlaying || s.Serving != counter.B || s.Score != [2]int{} {
		t.Errorf("after the rally: phase %v, serving %v, score %v", s.Phase, s.Serving, s.Score)
	}
}

// The warm-up waits for go; points pressed during it are not points.
func TestNothingCountsDuringTheWarmup(t *testing.T) {
	o := eleven(1)
	o.Warmup = true
	s, kept := play(o, "aab")
	if s.Phase != counter.PhaseWarmup || len(kept) != 0 {
		t.Fatalf("points during the warm-up counted: phase %v, kept %q", s.Phase, kept)
	}
	s, _ = play(o, "ga")
	if s.Phase != counter.PhasePlaying || s.Score != [2]int{1, 0} {
		t.Errorf("after go: phase %v, score %v", s.Phase, s.Score)
	}
	// With the serve played for, the warm-up leads into the rally.
	o.FirstServer = counter.Nobody
	s, _ = play(o, "g")
	if s.Phase != counter.PhaseServeRally {
		t.Errorf("warm-up then rally: phase %v", s.Phase)
	}
}

// Ends change after every set, and at five in the deciding set — once.
func TestEndsChangeAfterASetAndAtFiveInTheDecider(t *testing.T) {
	o := eleven(3)
	o.ChangeEnds = true

	s, _ := play(o, strings.Repeat("a", 11))
	if s.Phase != counter.PhaseChangeEnds || !s.Swapped {
		t.Fatalf("after set one: phase %v, swapped %v", s.Phase, s.Swapped)
	}
	s, _ = play(o, strings.Repeat("a", 11)+"a")
	if s.Score != [2]int{} {
		t.Errorf("a point during the change of ends counted: %v", s.Score)
	}

	// Set two to B, then the decider.
	twoSets := strings.Repeat("a", 11) + "g" + strings.Repeat("b", 11) + "g"
	s, _ = play(o, twoSets)
	if s.Swapped || s.Phase != counter.PhasePlaying {
		t.Fatalf("decider start: swapped %v, phase %v", s.Swapped, s.Phase)
	}
	s, _ = play(o, twoSets+"aaaa")
	if s.Phase != counter.PhasePlaying {
		t.Errorf("changed ends at 4:0")
	}
	s, _ = play(o, twoSets+"aaaaa")
	if s.Phase != counter.PhaseChangeEnds || !s.Swapped {
		t.Errorf("at 5:0 in the decider: phase %v, swapped %v", s.Phase, s.Swapped)
	}
	// Once only: 6:0 after go, and 5:5 later, do not change again.
	s, _ = play(o, twoSets+"aaaaagabbbbb")
	if s.Phase != counter.PhasePlaying || !s.Swapped {
		t.Errorf("changed ends a second time in the decider: phase %v, swapped %v", s.Phase, s.Swapped)
	}
}

// Without the option, nobody waits and nothing swaps.
func TestEndsStayWhenNotAsked(t *testing.T) {
	s, _ := play(eleven(3), strings.Repeat("a", 11)+strings.Repeat("b", 11)+"aaaaa")
	if s.Swapped || s.Phase != counter.PhasePlaying {
		t.Errorf("swapped %v, phase %v", s.Swapped, s.Phase)
	}
}

// Undo is the last event dropped: replaying one fewer puts the score, the
// service and the ends back, across a set boundary too.
func TestUndoIsReplayingOneEventFewer(t *testing.T) {
	o := eleven(3)
	o.ChangeEnds = true
	before, _ := play(o, strings.Repeat("a", 10)+"b")
	_, kept := play(o, strings.Repeat("a", 10)+"b"+"a")
	after, _ := counter.Replay(o, kept[:len(kept)-1])
	if after.Score != before.Score || after.Serving != before.Serving ||
		after.Swapped != before.Swapped || len(after.Sets) != len(before.Sets) {
		t.Errorf("undo across the set end: got %+v, want %+v", after, before)
	}
}

// Nonsense in the list is dropped rather than refused: the list came from a
// page, and the kept list is what goes back into it.
func TestUnknownEventsAreDropped(t *testing.T) {
	_, kept := play(eleven(1), "axb?g")
	if string(kept) != "ab" {
		t.Errorf("kept %q, want %q", kept, "ab")
	}
}
