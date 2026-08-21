// Package match validates a proposed match result.
//
// Pure domain logic: no database, no HTTP, plain values only, as the
// "Konventionen" section of CLAUDE.md requires. It is the single place that
// decides whether a set score is possible.
//
// The rejections it produces carry a Kind and the numbers behind it rather
// than a sentence. The Definition of Done for match entry is that a rejected
// result says *why*, and the player reads that sentence in German while this
// package is written in English — so phrasing it belongs to the layer that
// renders it, not here.
package match

import (
	"errors"
	"fmt"
)

// Allowed match modes. Both are settled in issue #19: eleven is the default,
// twenty-one the other option.
const (
	PointsToEleven    = 11
	PointsToTwentyOne = 21
)

// allowedBestOf mirrors the matches_best_of_valid constraint in the schema.
var allowedBestOf = map[int]bool{3: true, 5: true, 7: true}

// allowedPointsToWin mirrors the decision in issue #19.
var allowedPointsToWin = map[int]bool{PointsToEleven: true, PointsToTwentyOne: true}

// Mode is how a match is played.
type Mode struct {
	// BestOf is the number of sets at most: 3, 5 or 7.
	BestOf int
	// PointsToWin is the target score of a single set: 11 or 21.
	PointsToWin int
}

// SetsToWin is how many sets a player needs to take the match.
func (m Mode) SetsToWin() int { return m.BestOf/2 + 1 }

// Set is one set's score.
type Set struct {
	Home, Away int
}

// Result is a proposed result, in the order the sets were played.
type Result struct {
	Mode Mode
	Sets []Set
}

// Outcome is what a valid result amounts to.
type Outcome struct {
	HomeSets, AwaySets int
	// HomeWon reports which side took the match.
	HomeWon bool
}

// Kind names a reason a result was rejected.
type Kind string

const (
	// KindUnknownMode is a best-of or target score outside the allowed set.
	KindUnknownMode Kind = "unknown_mode"
	// KindNoSets is a result without a single set.
	KindNoSets Kind = "no_sets"
	// KindTooManySets is more sets than the mode allows.
	KindTooManySets Kind = "too_many_sets"
	// KindNegativePoints is a negative score.
	KindNegativePoints Kind = "negative_points"
	// KindDraw is a set that ended level. Table tennis has no draws.
	KindDraw Kind = "draw"
	// KindSetNotFinished is a set nobody has won yet.
	KindSetNotFinished Kind = "set_not_finished"
	// KindMarginTooSmall is a set won by a single point.
	KindMarginTooSmall Kind = "margin_too_small"
	// KindOvershoot is a set that ran past the point where it was already
	// decided — 13:10 at eleven, when it would have ended at 12:10.
	KindOvershoot Kind = "overshoot"
	// KindNoWinner is a result where nobody took enough sets.
	KindNoWinner Kind = "no_winner"
	// KindSetsAfterDecision is a set played after the match was already won.
	KindSetsAfterDecision Kind = "sets_after_decision"
)

// Rejection explains why a result is not possible.
//
// Its Error string is for logs and is English; the numbers it carries are
// what the presentation layer needs to phrase the message a player reads.
type Rejection struct {
	Kind Kind
	// SetNo is the 1-based set the problem is in, or 0 if the problem is the
	// result as a whole.
	SetNo int
	// Want and Got carry the numbers behind the rejection: the required
	// value and the one supplied. Their meaning depends on Kind.
	Want, Got int
}

// Error implements error.
func (r *Rejection) Error() string {
	if r.SetNo > 0 {
		return fmt.Sprintf("set %d rejected: %s (want %d, got %d)", r.SetNo, r.Kind, r.Want, r.Got)
	}
	return fmt.Sprintf("result rejected: %s (want %d, got %d)", r.Kind, r.Want, r.Got)
}

// Is lets callers match on the kind alone: errors.Is(err, &Rejection{Kind: K}).
func (r *Rejection) Is(target error) bool {
	var other *Rejection
	return errors.As(target, &other) && other.Kind == r.Kind
}

// Validate checks a proposed result and reports what it amounts to.
//
// It stops at the first problem. Listing every fault at once would be a
// nicer form, but a wrong set score usually makes the ones after it
// meaningless, and a single specific sentence beats a list of consequences.
func Validate(r Result) (Outcome, error) {
	if !allowedBestOf[r.Mode.BestOf] {
		return Outcome{}, &Rejection{Kind: KindUnknownMode, Got: r.Mode.BestOf}
	}
	if !allowedPointsToWin[r.Mode.PointsToWin] {
		return Outcome{}, &Rejection{Kind: KindUnknownMode, Got: r.Mode.PointsToWin}
	}

	setsToWin := r.Mode.SetsToWin()

	switch {
	case len(r.Sets) == 0:
		return Outcome{}, &Rejection{Kind: KindNoSets, Want: setsToWin}
	case len(r.Sets) > r.Mode.BestOf:
		return Outcome{}, &Rejection{Kind: KindTooManySets, Want: r.Mode.BestOf, Got: len(r.Sets)}
	}

	var out Outcome
	for i, set := range r.Sets {
		setNo := i + 1

		if err := validateSet(set, r.Mode.PointsToWin, setNo); err != nil {
			return Outcome{}, err
		}

		// A set played after one side already had enough is not a scoring
		// mistake but an impossible match, so it is reported as such.
		if out.HomeSets == setsToWin || out.AwaySets == setsToWin {
			return Outcome{}, &Rejection{
				Kind: KindSetsAfterDecision, SetNo: setNo, Want: setsToWin,
			}
		}

		if set.Home > set.Away {
			out.HomeSets++
		} else {
			out.AwaySets++
		}
	}

	switch {
	case out.HomeSets == setsToWin:
		out.HomeWon = true
	case out.AwaySets == setsToWin:
		out.HomeWon = false
	default:
		// Nobody got there: the match is unfinished, not mis-scored.
		return Outcome{}, &Rejection{
			Kind: KindNoWinner,
			Want: setsToWin,
			Got:  max(out.HomeSets, out.AwaySets),
		}
	}

	return out, nil
}

// validateSet checks one set against the target score.
func validateSet(s Set, pointsToWin, setNo int) error {
	if s.Home < 0 || s.Away < 0 {
		return &Rejection{Kind: KindNegativePoints, SetNo: setNo, Got: min(s.Home, s.Away)}
	}
	if s.Home == s.Away {
		return &Rejection{Kind: KindDraw, SetNo: setNo, Got: s.Home}
	}

	hi, lo := max(s.Home, s.Away), min(s.Home, s.Away)

	if hi < pointsToWin {
		return &Rejection{Kind: KindSetNotFinished, SetNo: setNo, Want: pointsToWin, Got: hi}
	}
	if hi-lo < 2 {
		// Reaching the target is not enough at 10:10 and beyond; two clear
		// points are.
		return &Rejection{Kind: KindMarginTooSmall, SetNo: setNo, Want: 2, Got: hi - lo}
	}
	if hi > pointsToWin && (hi-lo != 2 || lo < pointsToWin-1) {
		// Past the target a set ends the moment someone leads by two, so the
		// only possible scores are 12:10, 13:11, 14:12 and so on.
		return &Rejection{Kind: KindOvershoot, SetNo: setNo, Want: pointsToWin, Got: hi}
	}
	return nil
}
