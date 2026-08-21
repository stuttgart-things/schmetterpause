package ttr

import (
	"math"
	"slices"
	"testing"
)

// All expected values below were computed by hand from the formula in
// CLAUDE.md and are written out in the test names and comments, so a future
// change to the implementation has to argue with the arithmetic rather than
// with a recorded output.

const tolerance = 1e-6

func TestWinProbability(t *testing.T) {
	tests := []struct {
		name     string
		rating   int
		opponent int
		want     float64
	}{
		// 1 / (1 + 10^0) = 0.5
		{"equal ratings are a coin flip", 1000, 1000, 0.5},
		// 1 / (1 + 10^(-150/150)) = 1 / 1.1
		{"150 points ahead", 1150, 1000, 0.909091},
		// 1 / (1 + 10^(150/150)) = 1 / 11
		{"150 points behind", 1000, 1150, 0.090909},
		// 1 / (1 + 10^(-300/150)) = 1 / 1.01
		{"300 points ahead", 1300, 1000, 0.990099},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := WinProbability(tc.rating, tc.opponent)
			if math.Abs(got-tc.want) > tolerance {
				t.Errorf("WinProbability(%d, %d) = %.6f, want %.6f",
					tc.rating, tc.opponent, got, tc.want)
			}
		})
	}
}

// TestWinProbabilityUsesDivisor150 is the guard against the most likely
// mistake in this package: reaching for the chess Elo divisor of 400. At a
// 150-point gap the two disagree by a wide margin — 0.909 against 0.698 —
// so this catches it immediately.
func TestWinProbabilityUsesDivisor150(t *testing.T) {
	const elo400 = 0.698 // 1 / (1 + 10^(-150/400)), for contrast only

	got := WinProbability(1150, 1000)

	if math.Abs(got-elo400) < 0.05 {
		t.Fatalf("WinProbability(1150, 1000) = %.6f, which looks like the chess "+
			"Elo divisor of 400; the association system uses %v", got, Divisor)
	}
}

func TestWinProbabilityIsSymmetric(t *testing.T) {
	// Someone has to win, so the two probabilities must add up to 1.
	for _, gap := range []int{0, 1, 25, 150, 300, 1000} {
		a, b := 1000+gap, 1000

		sum := WinProbability(a, b) + WinProbability(b, a)

		if math.Abs(sum-1) > tolerance {
			t.Errorf("gap %d: P(a)+P(b) = %.9f, want 1", gap, sum)
		}
	}
}

func TestRateSingleMatch(t *testing.T) {
	tests := []struct {
		name      string
		rating    int
		opponent  int
		won       bool
		wantAfter int
	}{
		// E = 0.5, delta = 16 * (1 - 0.5) = +8
		{"equal ratings, win", 1000, 1000, true, 1008},
		// E = 0.5, delta = 16 * (0 - 0.5) = -8
		{"equal ratings, loss", 1000, 1000, false, 992},
		// E = 0.909091, delta = 16 * 0.090909 = +1.4545 -> +1
		{"favourite wins, gains almost nothing", 1150, 1000, true, 1151},
		// E = 0.090909, delta = 16 * -0.090909 = -1.4545 -> -1
		{"underdog loses, drops almost nothing", 1000, 1150, false, 999},
		// E = 0.090909, delta = 16 * 0.909091 = +14.5455 -> +15
		{"underdog wins, gains a lot", 1000, 1150, true, 1015},
		// E = 0.909091, delta = 16 * -0.909091 = -14.5455 -> -15
		{"favourite loses, drops a lot", 1150, 1000, false, 1135},
		// E = 0.990099, delta = 16 * 0.009901 = +0.1584 -> 0
		{"300 points ahead, win is worth nothing", 1300, 1000, true, 1300},
		// E = 0.009901, delta = 16 * -0.009901 = -0.1584 -> 0
		{"300 points behind, loss costs nothing", 1000, 1300, false, 1000},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Rate(tc.rating, Result{OpponentRating: tc.opponent, Won: tc.won})

			if got.After != tc.wantAfter {
				t.Errorf("Rate(%d, vs %d, won=%v).After = %d, want %d (delta %+d, want %+d)",
					tc.rating, tc.opponent, tc.won,
					got.After, tc.wantAfter, got.Delta(), tc.wantAfter-tc.rating)
			}
			if got.Before != tc.rating {
				t.Errorf("Before = %d, want %d", got.Before, tc.rating)
			}
		})
	}
}

// TestRateEvent is the case the MVP plan asks for by name: several singles
// settled as one event.
//
// A player rated 1000 plays three matches in a tournament — beats 900, loses
// to an equal opponent, beats 1100:
//
//	E(vs 900)  = 0.822745
//	E(vs 1000) = 0.500000
//	E(vs 1100) = 0.177255
//	           = 1.500000 summed
//
// Achieved is 2 of 3, so delta = 16 * (2 - 1.5) = +8.
func TestRateEvent(t *testing.T) {
	results := []Result{
		{OpponentRating: 900, Won: true},
		{OpponentRating: 1000, Won: false},
		{OpponentRating: 1100, Won: true},
	}

	got := Rate(1000, results...)

	if got.After != 1008 {
		t.Errorf("After = %d, want 1008 (delta %+d, want +8)", got.After, got.Delta())
	}
	if math.Abs(got.Expected-1.5) > tolerance {
		t.Errorf("Expected = %.6f, want 1.500000", got.Expected)
	}
	if got.Achieved != 2 {
		t.Errorf("Achieved = %v, want 2", got.Achieved)
	}
}

// TestRateIsOrderIndependent is the property that motivates event-wise
// scoring in the first place.
//
// A player rated 1000 loses to 800 and beats 850:
//
//	E(vs 800) = 0.955625
//	E(vs 850) = 0.909091
//	          = 1.864716 summed
//
// Achieved is 1, so delta = 16 * (1 - 1.864716) = -13.8355 -> -14, giving 986
// whichever way round the results are entered.
//
// Settling match by match would land on 986 or 987 depending on the order,
// because the second expected value would be computed against a rating the
// first match had already moved. That is exactly the surprise the rule
// avoids, and it is why this case was chosen over one where both approaches
// happen to agree.
func TestRateIsOrderIndependent(t *testing.T) {
	results := []Result{
		{OpponentRating: 800, Won: false},
		{OpponentRating: 850, Won: true},
	}

	forward := Rate(1000, results...)
	reversed := slices.Clone(results)
	slices.Reverse(reversed)
	backward := Rate(1000, reversed...)

	if forward.After != 986 {
		t.Errorf("After = %d, want 986 (delta %+d, want -14)", forward.After, forward.Delta())
	}
	if forward.After != backward.After {
		t.Errorf("order changes the outcome: %d forwards, %d backwards",
			forward.After, backward.After)
	}
}

func TestRateWithoutResultsLeavesTheRatingAlone(t *testing.T) {
	got := Rate(1234)

	if got.Before != 1234 || got.After != 1234 {
		t.Errorf("Rate(1234) = %+v, want the rating unchanged", got)
	}
	if got.Delta() != 0 {
		t.Errorf("Delta() = %d, want 0", got.Delta())
	}
}

// TestRateMatchIsZeroSum checks the property that makes a ranking stable:
// what one player gains, the other loses.
func TestRateMatchIsZeroSum(t *testing.T) {
	tests := []struct{ home, away int }{
		{1000, 1000},
		{1150, 1000},
		{1000, 1150},
		{1300, 1000},
		{1000, 1300},
	}

	for _, tc := range tests {
		for _, homeWon := range []bool{true, false} {
			home, away := RateMatch(tc.home, tc.away, homeWon)

			if home.Delta() != -away.Delta() {
				t.Errorf("RateMatch(%d, %d, homeWon=%v): home %+d, away %+d — not zero-sum",
					tc.home, tc.away, homeWon, home.Delta(), away.Delta())
			}
		}
	}
}

func TestRateMatchMirrorsRate(t *testing.T) {
	home, away := RateMatch(1000, 1150, true)

	wantHome := Rate(1000, Result{OpponentRating: 1150, Won: true})
	wantAway := Rate(1150, Result{OpponentRating: 1000, Won: false})

	if home != wantHome {
		t.Errorf("home = %+v, want %+v", home, wantHome)
	}
	if away != wantAway {
		t.Errorf("away = %+v, want %+v", away, wantAway)
	}
}

// TestRoundHalfAwayFromZero pins the commercial rounding down. The negative
// half is the interesting half: rounding towards zero or towards negative
// infinity would both be defensible in isolation and both wrong here.
func TestRoundHalfAwayFromZero(t *testing.T) {
	tests := []struct {
		in   float64
		want int
	}{
		{2.5, 3},
		{-2.5, -3},
		{0.5, 1},
		{-0.5, -1},
		{1.4999, 1},
		{-1.4999, -1},
		{0, 0},
		{13.8355, 14},
		{-13.8355, -14},
	}

	for _, tc := range tests {
		if got := roundHalfAwayFromZero(tc.in); got != tc.want {
			t.Errorf("roundHalfAwayFromZero(%v) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestChangeDelta(t *testing.T) {
	if got := (Change{Before: 1000, After: 1015}).Delta(); got != 15 {
		t.Errorf("Delta() = %d, want 15", got)
	}
	if got := (Change{Before: 1015, After: 1000}).Delta(); got != -15 {
		t.Errorf("Delta() = %d, want -15", got)
	}
}
