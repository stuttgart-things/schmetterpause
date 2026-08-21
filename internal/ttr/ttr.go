// Package ttr implements the rating used by the German table tennis
// association (Tischtennis-Rating).
//
// The package has no database and no HTTP dependency and works on plain
// values only, as required by the "Konventionen" section of CLAUDE.md. It is
// the single source of truth for how a rating changes.
//
// Two properties distinguish this from chess Elo and are easy to get subtly
// wrong:
//
//   - The divisor is 150, not 400. A 150-point gap therefore means roughly a
//     91% win probability, not 70%.
//   - Ratings are settled per event, not per match. The expected values
//     across all of a player's singles are summed and offset against the
//     achieved points once. That makes the result independent of the order in
//     which results are entered.
//
// Only singles count towards the rating. Doubles would need a rating of their
// own, which is why this package has no notion of a team.
package ttr

import "math"

// Divisor scales the rating difference in the win probability. The
// association system uses 150 where chess Elo uses 400.
const Divisor = 150.0

// BaseChangeConstant is the base value that scales a rating change. The
// association system knows further multipliers for young or inactive players;
// none of them are in the MVP.
const BaseChangeConstant = 16

// Result is one singles match from one player's point of view.
type Result struct {
	// OpponentRating is the opponent's rating before the event.
	OpponentRating int
	// Won reports whether the player won this match. Table tennis matches
	// cannot end in a draw, so a bool is enough.
	Won bool
}

// Change is the rating outcome for one player over one event.
type Change struct {
	// Before and After are the ratings around the event.
	Before, After int
	// Expected is the summed win probability across all matches of the
	// event, Achieved the points actually taken. Both are kept so that a
	// disputed calculation can be reconstructed later — the same reason
	// ttr_history exists.
	Expected, Achieved float64
}

// Delta is the rating change.
func (c Change) Delta() int { return c.After - c.Before }

// WinProbability returns the probability that a player rated rating beats one
// rated opponentRating.
//
//	P(A) = 1 / (1 + 10^((TTR_B - TTR_A) / 150))
func WinProbability(rating, opponentRating int) float64 {
	return 1 / (1 + math.Pow(10, float64(opponentRating-rating)/Divisor))
}

// Rate settles all of a player's singles from one event at once.
//
// Every expected value is computed against the rating the player held going
// into the event, never against an intermediate value. That is what makes the
// outcome independent of the order of results — enter the same matches in any
// sequence and Rate returns the same Change.
//
// With no results the rating stays untouched.
func Rate(rating int, results ...Result) Change {
	change := Change{Before: rating, After: rating}

	if len(results) == 0 {
		return change
	}

	for _, r := range results {
		change.Expected += WinProbability(rating, r.OpponentRating)
		if r.Won {
			change.Achieved++
		}
	}

	delta := BaseChangeConstant * (change.Achieved - change.Expected)
	change.After = rating + roundHalfAwayFromZero(delta)
	return change
}

// RateMatch settles a single match for both players at once. The common case
// outside tournaments, and the one the confirmation step uses.
func RateMatch(homeRating, awayRating int, homeWon bool) (home, away Change) {
	home = Rate(homeRating, Result{OpponentRating: awayRating, Won: homeWon})
	away = Rate(awayRating, Result{OpponentRating: homeRating, Won: !homeWon})
	return home, away
}

// roundHalfAwayFromZero rounds commercially: a half point always rounds away
// from zero, so a gain of 2.5 becomes 3 and a loss of 2.5 becomes -3. Go's
// math.Round already has exactly this behaviour; the wrapper exists to name
// the requirement and to give it a test of its own.
func roundHalfAwayFromZero(v float64) int {
	return int(math.Round(v))
}
