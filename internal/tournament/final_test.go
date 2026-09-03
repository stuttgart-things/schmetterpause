package tournament_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/tournament"
)

// A return leg is the same pair in a different slot. That is the whole point:
// keyed on the pair alone the second meeting would find the first one's
// result and never be playable (docs/adr/0011).
func TestTheSecondLegRepeatsEveryPairInNewRounds(t *testing.T) {
	for _, n := range []int{2, 3, 4, 5, 8} {
		players := ids(n)
		rounds := tournament.Draw(players, 2)

		if want := tournament.GroupRounds(n, 2); len(rounds) != want {
			t.Fatalf("%d players: got %d rounds, want %d", n, len(rounds), want)
		}

		meetings := map[[2]uuid.UUID]int{}
		slots := map[[3]string]bool{}
		for _, round := range rounds {
			for _, p := range round.Pairings {
				key := [2]uuid.UUID{p.Home, p.Away}
				if p.Away.String() < p.Home.String() {
					key = [2]uuid.UUID{p.Away, p.Home}
				}
				meetings[key]++
				slot := [3]string{key[0].String(), key[1].String(), string(rune(round.No))}
				if slots[slot] {
					t.Errorf("%d players: a pair meets twice in round %d", n, round.No)
				}
				slots[slot] = true
			}
		}

		if want := n * (n - 1) / 2; len(meetings) != want {
			t.Errorf("%d players: %d distinct pairs, want %d", n, len(meetings), want)
		}
		for pair, times := range meetings {
			if times != 2 {
				t.Errorf("%d players: pair %v meets %d times, want 2", n, pair, times)
			}
		}
	}
}

// The sides swap in the return leg, and that is the only thing it changes.
func TestTheReturnLegSwapsTheSides(t *testing.T) {
	players := ids(4)
	rounds := tournament.Draw(players, 2)
	group := tournament.GroupRounds(4, 1)

	first := map[[2]uuid.UUID]bool{}
	for _, round := range rounds[:group] {
		for _, p := range round.Pairings {
			first[[2]uuid.UUID{p.Home, p.Away}] = true
		}
	}
	for _, round := range rounds[group:] {
		for _, p := range round.Pairings {
			if first[[2]uuid.UUID{p.Home, p.Away}] {
				t.Errorf("round %d repeats the first leg's orientation", round.No)
			}
			if !first[[2]uuid.UUID{p.Away, p.Home}] {
				t.Errorf("round %d has a pairing the first leg never had", round.No)
			}
		}
	}
}

func TestMatchesCountsTheFormatItIsGiven(t *testing.T) {
	for _, tc := range []struct {
		n, legs int
		final   bool
		want    int
	}{
		{4, 1, false, 6},
		{4, 1, true, 7},
		{4, 2, false, 12},
		{8, 1, false, 28},
		{8, 2, false, 56},
		{8, 2, true, 57},
		{1, 2, true, 0},
	} {
		if got := tournament.Matches(tc.n, tc.legs, tc.final); got != tc.want {
			t.Errorf("Matches(%d, %d, %v) = %d, want %d",
				tc.n, tc.legs, tc.final, got, tc.want)
		}
	}
}

// The final is the two best of the group — but only when the table can name
// them. A decider between two arbitrarily chosen of three equals is a draw
// with an audience.
func TestTheFinalNeedsAnUnambiguousTopTwo(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()

	for _, tc := range []struct {
		name string
		rows []tournament.TableRow
		ok   bool
	}{
		{
			"clear",
			[]tournament.TableRow{
				{PlayerID: a, Played: 2, Rank: 1},
				{PlayerID: b, Played: 2, Rank: 2},
				{PlayerID: c, Played: 2, Rank: 3},
			},
			true,
		},
		{
			"two share first",
			[]tournament.TableRow{
				{PlayerID: a, Played: 2, Rank: 1, Shared: true},
				{PlayerID: b, Played: 2, Rank: 1, Shared: true},
				{PlayerID: c, Played: 2, Rank: 3},
			},
			false,
		},
		{
			"two share second",
			[]tournament.TableRow{
				{PlayerID: a, Played: 2, Rank: 1},
				{PlayerID: b, Played: 2, Rank: 2, Shared: true},
				{PlayerID: c, Played: 2, Rank: 2, Shared: true},
			},
			false,
		},
		{
			"nobody has played",
			[]tournament.TableRow{{PlayerID: a}, {PlayerID: b}},
			false,
		},
		{"a field of one", []tournament.TableRow{{PlayerID: a, Played: 1}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pairing, ok := tournament.Final(tc.rows)
			if ok != tc.ok {
				t.Fatalf("Final() ok = %v, want %v", ok, tc.ok)
			}
			if ok && (pairing.Home != tc.rows[0].PlayerID || pairing.Away != tc.rows[1].PlayerID) {
				t.Error("the final is not between the best two")
			}
		})
	}
}

// Counting the final in the group table would let its result move the
// standings that decided who plays it.
func TestTheGroupTableLeavesTheFinalOut(t *testing.T) {
	p := ids(3)
	group, final := 3, 4
	matches := []domain.Match{
		confirmedIn(p[0], p[1], group, 11, 5),
		confirmedIn(p[1], p[2], group, 11, 5),
		confirmedIn(p[2], p[0], group, 11, 5),
		// The decider, played and confirmed.
		confirmedIn(p[1], p[0], final, 11, 2),
	}

	rows := tournament.Table(p, matches, 3, tournament.ScoreByWins)
	for _, r := range rows {
		if r.Played != 2 {
			t.Errorf("player %v played %d group matches, want 2", r.PlayerID, r.Played)
		}
	}

	// With no group boundary given, everything counts — which is what a
	// tournament without a final needs.
	all := tournament.Table(p, matches, 0, tournament.ScoreByWins)
	total := 0
	for _, r := range all {
		total += r.Played
	}
	if total != 8 {
		t.Errorf("without a boundary %d appearances, want 8", total)
	}
}

// A row from before slots existed has no round and is a group match: those
// tournaments are single round robins, where the two keys are the same.
func TestATableCountsMatchesWithoutARound(t *testing.T) {
	p := ids(2)
	m := confirmedIn(p[0], p[1], 0, 11, 4)
	m.TournamentRound = nil

	rows := tournament.Table(p, []domain.Match{m}, 1, tournament.ScoreByWins)
	if rows[0].Played != 1 {
		t.Error("a match from before rounds existed was dropped from the table")
	}
}

func confirmedIn(home, away uuid.UUID, round, homePoints, awayPoints int) domain.Match {
	m := domain.Match{
		HomeID: home, AwayID: away,
		Status: domain.MatchConfirmed,
		Sets:   []domain.MatchSet{{SetNo: 1, HomePoints: homePoints, AwayPoints: awayPoints}},
	}
	if round > 0 {
		m.TournamentRound = &round
	}
	return m
}
