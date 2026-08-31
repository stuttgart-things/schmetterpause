package match_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stuttgart-things/schmetterpause/internal/match"
)

func bestOf(n, points int) match.Mode {
	return match.Mode{BestOf: n, PointsToWin: points}
}

func TestSetsToWin(t *testing.T) {
	for _, tc := range []struct{ bestOf, want int }{{1, 1}, {3, 2}, {5, 3}, {7, 4}} {
		if got := (match.Mode{BestOf: tc.bestOf}).SetsToWin(); got != tc.want {
			t.Errorf("best-of-%d needs %d sets, want %d", tc.bestOf, got, tc.want)
		}
	}
}

func TestValidateAcceptsRealResults(t *testing.T) {
	tests := []struct {
		name        string
		mode        match.Mode
		sets        []match.Set
		wantHome    int
		wantAway    int
		wantHomeWon bool
	}{
		{
			name: "straight sets", mode: bestOf(3, 11),
			sets:     []match.Set{{11, 9}, {11, 7}},
			wantHome: 2, wantAway: 0, wantHomeWon: true,
		},
		{
			name: "the full distance", mode: bestOf(5, 11),
			sets:     []match.Set{{11, 8}, {9, 11}, {11, 13}, {11, 6}, {12, 10}},
			wantHome: 3, wantAway: 2, wantHomeWon: true,
		},
		{
			// Past the target a set runs on until someone leads by two, so
			// 12:10 and 13:11 are the only shapes it can take.
			name: "deuce", mode: bestOf(3, 11),
			sets:     []match.Set{{12, 10}, {13, 11}},
			wantHome: 2, wantAway: 0, wantHomeWon: true,
		},
		{
			name: "to twenty-one", mode: bestOf(3, 21),
			sets:     []match.Set{{21, 19}, {19, 21}, {23, 21}},
			wantHome: 2, wantAway: 1, wantHomeWon: true,
		},
		{
			// Issue #114: the mode most breaks actually produce.
			name: "a single set", mode: bestOf(1, 11),
			sets:     []match.Set{{11, 8}},
			wantHome: 1, wantAway: 0, wantHomeWon: true,
		},
		{
			name: "a single set to twenty-one, lost", mode: bestOf(1, 21),
			sets:     []match.Set{{19, 21}},
			wantHome: 0, wantAway: 1, wantHomeWon: false,
		},
		{
			name: "away takes it", mode: bestOf(3, 11),
			sets:     []match.Set{{9, 11}, {5, 11}},
			wantHome: 0, wantAway: 2, wantHomeWon: false,
		},
		{
			name: "a whitewash is a real score", mode: bestOf(3, 11),
			sets:     []match.Set{{11, 0}, {11, 0}},
			wantHome: 2, wantAway: 0, wantHomeWon: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := match.Validate(match.Result{Mode: tc.mode, Sets: tc.sets})
			if err != nil {
				t.Fatalf("Validate(): %v", err)
			}
			if out.HomeSets != tc.wantHome || out.AwaySets != tc.wantAway {
				t.Errorf("sets = %d:%d, want %d:%d", out.HomeSets, out.AwaySets, tc.wantHome, tc.wantAway)
			}
			if out.HomeWon != tc.wantHomeWon {
				t.Errorf("HomeWon = %v, want %v", out.HomeWon, tc.wantHomeWon)
			}
		})
	}
}

func TestValidateRejectsImpossibleResults(t *testing.T) {
	tests := []struct {
		name  string
		mode  match.Mode
		sets  []match.Set
		want  match.Kind
		setNo int
	}{
		{
			// The rule people get wrong most often: reaching eleven is not
			// enough at 10:10.
			name: "won by a single point", mode: bestOf(3, 11),
			sets: []match.Set{{11, 10}}, want: match.KindMarginTooSmall, setNo: 1,
		},
		{
			name: "nobody reached the target", mode: bestOf(3, 11),
			sets: []match.Set{{10, 8}}, want: match.KindSetNotFinished, setNo: 1,
		},
		{
			// It would have ended at 12:10, so 13:10 never happened.
			name: "ran past the decision", mode: bestOf(3, 11),
			sets: []match.Set{{11, 9}, {13, 10}}, want: match.KindOvershoot, setNo: 2,
		},
		{
			name: "far past the decision", mode: bestOf(3, 11),
			sets: []match.Set{{15, 5}}, want: match.KindOvershoot, setNo: 1,
		},
		{
			name: "eleven points in a twenty-one match", mode: bestOf(3, 21),
			sets: []match.Set{{11, 9}}, want: match.KindSetNotFinished, setNo: 1,
		},
		{
			name: "level", mode: bestOf(3, 11),
			sets: []match.Set{{11, 11}}, want: match.KindDraw, setNo: 1,
		},
		{
			name: "negative", mode: bestOf(3, 11),
			sets: []match.Set{{11, -1}}, want: match.KindNegativePoints, setNo: 1,
		},
		{
			name: "more sets than the mode allows", mode: bestOf(3, 11),
			sets: []match.Set{{11, 0}, {0, 11}, {11, 0}, {11, 0}}, want: match.KindTooManySets,
		},
		{
			// 2:0 in a best-of-three ends it; a third set cannot have been
			// played.
			name: "a set after the match was won", mode: bestOf(3, 11),
			sets: []match.Set{{11, 0}, {11, 0}, {11, 0}}, want: match.KindSetsAfterDecision, setNo: 3,
		},
		{
			name: "unfinished", mode: bestOf(3, 11),
			sets: []match.Set{{11, 0}, {0, 11}}, want: match.KindNoWinner,
		},
		{
			name: "no sets at all", mode: bestOf(3, 11),
			sets: nil, want: match.KindNoSets,
		},
		{
			// One set is the whole match, so a second one cannot have been
			// played — the same rejection a fourth set gets in best-of-three.
			name: "a second set in the one-set mode", mode: bestOf(1, 11),
			sets: []match.Set{{11, 0}, {11, 0}}, want: match.KindTooManySets,
		},
		{
			name: "best-of-four is not a thing", mode: bestOf(4, 11),
			sets: []match.Set{{11, 0}, {11, 0}}, want: match.KindUnknownMode,
		},
		{
			name: "fifteen points is not an option", mode: bestOf(3, 15),
			sets: []match.Set{{15, 0}, {15, 0}}, want: match.KindUnknownMode,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := match.Validate(match.Result{Mode: tc.mode, Sets: tc.sets})

			var rejection *match.Rejection
			if !errors.As(err, &rejection) {
				t.Fatalf("Validate() = %v, want a *match.Rejection", err)
			}
			if rejection.Kind != tc.want {
				t.Errorf("Kind = %q, want %q (%v)", rejection.Kind, tc.want, err)
			}
			if rejection.SetNo != tc.setNo {
				t.Errorf("SetNo = %d, want %d", rejection.SetNo, tc.setNo)
			}
		})
	}
}

// TestDeuceBoundaries walks the edge where the target score stops deciding a
// set, because that is where an off-by-one is invisible in review and obvious
// to anyone who plays.
func TestDeuceBoundaries(t *testing.T) {
	tests := []struct {
		set  match.Set
		want bool
	}{
		{match.Set{11, 9}, true},   // the last score the target alone decides
		{match.Set{11, 10}, false}, // one clear point is not two
		{match.Set{12, 10}, true},  // the first deuce score
		{match.Set{12, 11}, false},
		{match.Set{13, 11}, true},
		{match.Set{13, 10}, false}, // would already have ended at 12:10
		{match.Set{20, 18}, true},  // a long deuce is still a deuce
	}

	for _, tc := range tests {
		_, err := match.Validate(match.Result{
			Mode: bestOf(3, 11),
			Sets: []match.Set{tc.set, {11, 0}},
		})
		if got := err == nil; got != tc.want {
			t.Errorf("%d:%d accepted = %v, want %v (%v)", tc.set.Home, tc.set.Away, got, tc.want, err)
		}
	}
}

// TestRejectionIsMatchesOnKind keeps errors.Is usable for callers that only
// care which rule was broken.
func TestRejectionIsMatchesOnKind(t *testing.T) {
	_, err := match.Validate(match.Result{Mode: bestOf(3, 11), Sets: []match.Set{{11, 10}}})

	if !errors.Is(err, &match.Rejection{Kind: match.KindMarginTooSmall}) {
		t.Errorf("errors.Is did not match on the kind: %v", err)
	}
	if errors.Is(err, &match.Rejection{Kind: match.KindDraw}) {
		t.Errorf("errors.Is matched the wrong kind: %v", err)
	}
}

func TestRejectionErrorNamesTheSet(t *testing.T) {
	_, err := match.Validate(match.Result{Mode: bestOf(5, 11), Sets: []match.Set{{11, 0}, {11, 10}}})

	if got := err.Error(); !strings.Contains(got, "set 2") {
		t.Errorf("Error() = %q, want it to name set 2", got)
	}
}
