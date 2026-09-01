package tournament_test

import (
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/tournament"
)

// ids returns n distinct, stable player ids. Stable so a failing case can be
// read: player 3 is always player 3.
func ids(n int) []uuid.UUID {
	out := make([]uuid.UUID, n)
	for i := range out {
		out[i] = uuid.MustParse(fmt.Sprintf("00000000-0000-0000-0000-%012d", i+1))
	}
	return out
}

// pairKey names an encounter regardless of orientation, so "did these two
// meet" is a map lookup rather than two comparisons everywhere.
func pairKey(a, b uuid.UUID) [2]uuid.UUID {
	if a.String() > b.String() {
		a, b = b, a
	}
	return [2]uuid.UUID{a, b}
}

func TestEverybodyMeetsEverybodyExactlyOnce(t *testing.T) {
	for n := 2; n <= 11; n++ {
		t.Run(fmt.Sprintf("%d players", n), func(t *testing.T) {
			players := ids(n)
			seen := map[[2]uuid.UUID]int{}

			for _, round := range tournament.Draw(players) {
				for _, p := range round.Pairings {
					seen[pairKey(p.Home, p.Away)]++
				}
			}

			if want := tournament.Matches(n); len(seen) != want {
				t.Errorf("got %d distinct pairings, want %d", len(seen), want)
			}
			for pair, count := range seen {
				if count != 1 {
					t.Errorf("pair %v meets %d times, want 1", pair, count)
				}
			}
		})
	}
}

func TestNobodyPlaysTwiceInARound(t *testing.T) {
	for n := 2; n <= 11; n++ {
		t.Run(fmt.Sprintf("%d players", n), func(t *testing.T) {
			for _, round := range tournament.Draw(ids(n)) {
				busy := map[uuid.UUID]bool{}
				for _, p := range round.Pairings {
					for _, id := range []uuid.UUID{p.Home, p.Away} {
						if busy[id] {
							t.Errorf("round %d: %v plays twice", round.No, id)
						}
						busy[id] = true
					}
				}
				if round.Bye != uuid.Nil && busy[round.Bye] {
					t.Errorf("round %d: %v has a bye and plays", round.No, round.Bye)
				}
			}
		})
	}
}

// An even field needs no byes, and the placeholder must never leak into a
// pairing as a player somebody is expected to find.
func TestAnEvenFieldHasNoByes(t *testing.T) {
	for _, n := range []int{2, 4, 6, 8, 10} {
		rounds := tournament.Draw(ids(n))
		if len(rounds) != n-1 {
			t.Errorf("%d players: got %d rounds, want %d", n, len(rounds), n-1)
		}
		for _, round := range rounds {
			if round.Bye != uuid.Nil {
				t.Errorf("%d players, round %d: unexpected bye %v", n, round.No, round.Bye)
			}
			if len(round.Pairings) != n/2 {
				t.Errorf("%d players, round %d: %d pairings, want %d",
					n, round.No, len(round.Pairings), n/2)
			}
		}
	}
}

// The property that makes an odd field fair: everybody sits out exactly once,
// and nobody sits out twice while somebody else never does.
func TestAnOddFieldGivesEverybodyExactlyOneBye(t *testing.T) {
	for _, n := range []int{3, 5, 7, 9, 11} {
		players := ids(n)
		rounds := tournament.Draw(players)
		if len(rounds) != n {
			t.Errorf("%d players: got %d rounds, want %d", n, len(rounds), n)
		}

		byes := map[uuid.UUID]int{}
		for _, round := range rounds {
			if round.Bye == uuid.Nil {
				t.Errorf("%d players, round %d: expected a bye", n, round.No)
				continue
			}
			byes[round.Bye]++
		}
		for _, id := range players {
			if byes[id] != 1 {
				t.Errorf("%d players: %v sits out %d times, want 1", n, id, byes[id])
			}
		}
	}
}

// The orientation alternates so the same person is not printed on the left in
// every round. Cosmetic, but the draw is read by people.
func TestOrientationAlternatesBetweenRounds(t *testing.T) {
	rounds := tournament.Draw(ids(4))
	if len(rounds) < 2 {
		t.Fatalf("got %d rounds, want at least 2", len(rounds))
	}

	first := rounds[0].Pairings[0].Home
	sameSide := 0
	for _, round := range rounds {
		for _, p := range round.Pairings {
			if p.Home == first {
				sameSide++
			}
		}
	}
	if sameSide == len(rounds) {
		t.Errorf("player %v is home in all %d rounds", first, len(rounds))
	}
}

// A tournament somebody is still adding people to is an ordinary state, not
// an error.
func TestTooFewPlayersDrawNothing(t *testing.T) {
	for _, players := range [][]uuid.UUID{nil, ids(1)} {
		if got := tournament.Draw(players); got != nil {
			t.Errorf("Draw(%d players) = %v, want nil", len(players), got)
		}
	}
}

func TestMatchesCountsThePairs(t *testing.T) {
	for n, want := range map[int]int{0: 0, 1: 0, 2: 1, 4: 6, 6: 15, 8: 28, 12: 66} {
		if got := tournament.Matches(n); got != want {
			t.Errorf("Matches(%d) = %d, want %d", n, got, want)
		}
	}
}

// confirmed builds a confirmed match with the given set scores, so the table
// tests read as results rather than as struct literals.
func confirmed(home, away uuid.UUID, sets ...[2]int) domain.Match {
	m := domain.Match{HomeID: home, AwayID: away, Status: domain.MatchConfirmed}
	for i, s := range sets {
		m.Sets = append(m.Sets, domain.MatchSet{
			SetNo: i + 1, HomePoints: s[0], AwayPoints: s[1],
		})
	}
	return m
}

func TestTableCountsWinsSetsAndPoints(t *testing.T) {
	p := ids(2)
	rows := tournament.Table(p, []domain.Match{
		confirmed(p[0], p[1], [2]int{11, 5}, [2]int{11, 7}),
	})

	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	winner, loser := rows[0], rows[1]

	if winner.PlayerID != p[0] {
		t.Errorf("winner is %v, want %v", winner.PlayerID, p[0])
	}
	if winner.Won != 1 || winner.Lost != 0 || winner.Played != 1 {
		t.Errorf("winner record = %d/%d in %d, want 1/0 in 1",
			winner.Won, winner.Lost, winner.Played)
	}
	if winner.SetsWon != 2 || winner.SetsLost != 0 {
		t.Errorf("winner sets = %d:%d, want 2:0", winner.SetsWon, winner.SetsLost)
	}
	if winner.PointsFor != 22 || winner.PointsAgainst != 12 {
		t.Errorf("winner points = %d:%d, want 22:12", winner.PointsFor, winner.PointsAgainst)
	}
	if loser.SetDiff() != -2 {
		t.Errorf("loser set diff = %d, want -2", loser.SetDiff())
	}
	if winner.Rank != 1 || loser.Rank != 2 {
		t.Errorf("ranks = %d and %d, want 1 and 2", winner.Rank, loser.Rank)
	}
}

// Somebody who has not played is a row of zeroes, not an absence — the state
// every tournament is in for its first twenty minutes.
func TestParticipantsWithoutResultsStillAppear(t *testing.T) {
	p := ids(4)
	rows := tournament.Table(p, nil)

	if len(rows) != 4 {
		t.Fatalf("got %d rows, want 4", len(rows))
	}
	for _, r := range rows {
		if r.Rank != 0 {
			t.Errorf("%v has rank %d before playing, want 0", r.PlayerID, r.Rank)
		}
	}
}

// The rule people at the table actually invoke: "ja, aber ich hab gegen dich
// gewonnen". Both players win two and lose one; the direct encounter decides,
// even though the loser of it has the better overall set difference.
func TestTheDirectEncounterBreaksATie(t *testing.T) {
	p := ids(4)
	a, b, c, d := p[0], p[1], p[2], p[3]

	// Two wins each, and a beat b. But a's wins are three-setters and one of
	// his matches was a whitewash against him, so b's set difference is the
	// better one by a distance: +3 against 0.
	rows := tournament.Table(p, []domain.Match{
		confirmed(a, b, [2]int{11, 9}, [2]int{9, 11}, [2]int{11, 9}),
		confirmed(a, c, [2]int{11, 9}, [2]int{9, 11}, [2]int{11, 9}),
		confirmed(d, a, [2]int{11, 0}, [2]int{11, 0}),
		confirmed(b, c, [2]int{11, 0}, [2]int{11, 0}),
		confirmed(b, d, [2]int{11, 0}, [2]int{11, 0}),
		confirmed(c, d, [2]int{11, 0}, [2]int{11, 0}),
	})

	if rows[0].PlayerID != a {
		t.Errorf("first is %v, want %v — the direct encounter should outrank set difference",
			rows[0].PlayerID, a)
	}
	if rows[1].PlayerID != b {
		t.Errorf("second is %v, want %v", rows[1].PlayerID, b)
	}
	if rows[0].Won != rows[1].Won {
		t.Fatalf("the test no longer describes a tie: %d wins against %d",
			rows[0].Won, rows[1].Won)
	}
	// The point of the case: b really does have the better set difference.
	if rows[1].SetDiff() <= rows[0].SetDiff() {
		t.Errorf("set diffs are %d and %d; the tie-break is not being exercised",
			rows[0].SetDiff(), rows[1].SetDiff())
	}
}

// Two players nothing separates share a position, and the next distinct one
// skips what they used up — 1, 1, 3, the way a sports table reads.
func TestPlayersNothingSeparatesShareARank(t *testing.T) {
	p := ids(3)
	a, b, c := p[0], p[1], p[2]

	// a and b each beat c by the same margin and have not met.
	rows := tournament.Table(p, []domain.Match{
		confirmed(a, c, [2]int{11, 5}, [2]int{11, 5}),
		confirmed(b, c, [2]int{11, 5}, [2]int{11, 5}),
	})

	if rows[0].Rank != 1 || rows[1].Rank != 1 {
		t.Errorf("ranks = %d and %d, want 1 and 1", rows[0].Rank, rows[1].Rank)
	}
	if !rows[0].Shared || !rows[1].Shared {
		t.Errorf("shared flags = %v and %v, want both true", rows[0].Shared, rows[1].Shared)
	}
	if rows[2].Rank != 3 {
		t.Errorf("third rank = %d, want 3 — a shared rank uses up a position", rows[2].Rank)
	}
}

// A result nobody has agreed to is not a result, and a match against somebody
// outside the tournament is not this tournament's business.
func TestTableIgnoresUnconfirmedAndOutsideMatches(t *testing.T) {
	p := ids(2)
	outsider := ids(3)[2]

	rows := tournament.Table(p, []domain.Match{
		{HomeID: p[0], AwayID: p[1], Status: domain.MatchPending,
			Sets: []domain.MatchSet{{SetNo: 1, HomePoints: 11, AwayPoints: 0}}},
		{HomeID: p[0], AwayID: p[1], Status: domain.MatchDisputed,
			Sets: []domain.MatchSet{{SetNo: 1, HomePoints: 11, AwayPoints: 0}}},
		confirmed(p[0], outsider, [2]int{11, 0}),
	})

	for _, r := range rows {
		if r.Played != 0 {
			t.Errorf("%v counted %d matches, want 0", r.PlayerID, r.Played)
		}
	}
}

// The draw and the table have to agree about the field, or a tournament ends
// with a table that is missing a row.
func TestEveryPairInTheDrawCanReachTheTable(t *testing.T) {
	p := ids(5)

	var matches []domain.Match
	for _, round := range tournament.Draw(p) {
		for _, pair := range round.Pairings {
			matches = append(matches, confirmed(pair.Home, pair.Away, [2]int{11, 0}))
		}
	}

	rows := tournament.Table(p, matches)
	if len(rows) != len(p) {
		t.Fatalf("got %d rows, want %d", len(rows), len(p))
	}

	total := 0
	for _, r := range rows {
		total += r.Played
		if r.Played != len(p)-1 {
			t.Errorf("%v played %d, want %d", r.PlayerID, r.Played, len(p)-1)
		}
	}
	// Every match is counted for both players, so the sum is twice the draw.
	if want := 2 * tournament.Matches(len(p)); total != want {
		t.Errorf("total appearances = %d, want %d", total, want)
	}
}

// A set difference of zero belongs to somebody who has not played, and it
// must not outrank somebody who turned up and lost. The overall ranking makes
// the same exception, and a tournament table that disagreed with it would be
// read as a bug in whichever of the two somebody looked at second.
func TestAPlayerWhoTurnedUpOutranksOneWhoDidNot(t *testing.T) {
	p := ids(3)
	loser, winner, absent := p[0], p[1], p[2]

	rows := tournament.Table(p, []domain.Match{
		confirmed(winner, loser, [2]int{11, 0}, [2]int{11, 0}),
	})

	var positions = map[uuid.UUID]int{}
	for i, r := range rows {
		positions[r.PlayerID] = i
	}
	if positions[loser] > positions[absent] {
		t.Errorf("the absent player sorts above somebody who played and lost")
	}
	if rows[positions[absent]].Rank != 0 {
		t.Errorf("absent rank = %d, want 0", rows[positions[absent]].Rank)
	}
	if rows[positions[loser]].Rank != 2 {
		t.Errorf("loser rank = %d, want 2", rows[positions[loser]].Rank)
	}
}
