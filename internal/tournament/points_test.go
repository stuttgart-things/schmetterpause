package tournament_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/tournament"
)

// bestOf is confirmed() with the mode set, which is what decides whether a
// loss went to the last set the match could have.
func bestOf(n int, home, away uuid.UUID, sets ...[2]int) domain.Match {
	m := confirmed(home, away, sets...)
	m.BestOf = n
	return m
}

func TestTablePointsAreThreeAWinAndOneForACloseLoss(t *testing.T) {
	cases := []struct {
		won, lostDeciding, want int
	}{
		{0, 0, 0},
		{1, 0, 3},
		{0, 1, 1},
		{3, 2, 11},
	}
	for _, c := range cases {
		row := tournament.TableRow{Won: c.won, LostDeciding: c.lostDeciding, Lost: 4}
		if got := row.TablePoints(); got != c.want {
			t.Errorf("%d wins and %d close losses = %d points, want %d",
				c.won, c.lostDeciding, got, c.want)
		}
	}
}

// A loss counts as close when the match needed the last set the mode allows.
func TestOnlyTheLastSetTheModeAllowsCounts(t *testing.T) {
	p := ids(2)
	cases := []struct {
		name  string
		match domain.Match
		want  int
	}{
		{"best of three, 2:1", bestOf(3, p[0], p[1], [2]int{11, 9}, [2]int{9, 11}, [2]int{11, 8}), 1},
		{"best of three, 2:0", bestOf(3, p[0], p[1], [2]int{11, 9}, [2]int{11, 8}), 0},
		{"best of five, 3:2", bestOf(5, p[0], p[1],
			[2]int{11, 9}, [2]int{9, 11}, [2]int{11, 8}, [2]int{7, 11}, [2]int{11, 9}), 1},
		{"best of five, 3:1", bestOf(5, p[0], p[1],
			[2]int{11, 9}, [2]int{9, 11}, [2]int{11, 8}, [2]int{11, 7}), 0},
		// A single set is the whole match. Every loss would earn a point,
		// and then losing more would lift somebody above losing less on the
		// same wins — a reward for turning up rather than for playing close.
		{"one set", bestOf(1, p[0], p[1], [2]int{11, 9}), 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rows := tournament.Table(p, []domain.Match{c.match}, 0, tournament.ScoreByPoints)
			loser := rows[1]
			if loser.LostDeciding != c.want {
				t.Errorf("the loser has %d close losses, want %d", loser.LostDeciding, c.want)
			}
		})
	}
}

// The point of the whole setting: counting in points can order a field
// differently from counting wins. Before the bonus point it could not — 3·W
// is a monotone transform of W — and the choice was a column rather than an
// answer.
//
// Five players, so exactly two of them end up level on wins. B and C both win
// twice; C won their meeting, so counting wins puts C ahead. B's other loss
// went to the deciding set and C's did not, so counting points puts B ahead
// by one. That swap is the whole feature.
func TestPointsCanOutrankAFlatterRecordOnTheSameWins(t *testing.T) {
	p := ids(5)
	a, b, c, d, e := p[0], p[1], p[2], p[3], p[4]

	flat := func(home, away uuid.UUID) domain.Match {
		return bestOf(3, home, away, [2]int{11, 5}, [2]int{11, 7})
	}
	matches := []domain.Match{
		// A wins everything, and takes B to the deciding set doing it.
		bestOf(3, a, b, [2]int{11, 9}, [2]int{9, 11}, [2]int{11, 8}),
		flat(a, c), flat(a, d), flat(a, e),
		// C beats B, which is what puts C above B on wins.
		flat(c, b), flat(c, d),
		flat(b, d), flat(b, e),
		flat(e, c),
		flat(d, e),
	}

	byWins := tournament.Table(p, matches, 0, tournament.ScoreByWins)
	byPoints := tournament.Table(p, matches, 0, tournament.ScoreByPoints)

	// Two wins each, and the meeting between them decides it.
	if byWins[1].PlayerID != c || byWins[2].PlayerID != b {
		t.Fatalf("counting wins gives %v then %v behind the leader, want C then B",
			byWins[1].PlayerID, byWins[2].PlayerID)
	}
	// Six points against seven: B's close loss is worth one, C's is worth
	// nothing, and they are no longer level so the meeting never comes up.
	if byPoints[1].PlayerID != b || byPoints[2].PlayerID != c {
		t.Errorf("counting points gives %v then %v behind the leader, want B then C",
			byPoints[1].PlayerID, byPoints[2].PlayerID)
	}
	if byPoints[1].TablePoints() != 7 || byPoints[2].TablePoints() != 6 {
		t.Errorf("points are %d and %d, want 7 and 6",
			byPoints[1].TablePoints(), byPoints[2].TablePoints())
	}
	// The leader is the leader either way: four wins is twelve points, and
	// nothing about the bonus touches that.
	if byWins[0].PlayerID != a || byPoints[0].PlayerID != a {
		t.Errorf("the leader changed: %v by wins, %v by points",
			byWins[0].PlayerID, byPoints[0].PlayerID)
	}
}

// Counting in points must not invent a rank where wins found a tie for the
// same reason, or lose one: the bonus separates players, it does not stop
// separating them.
func TestPointsStillShareARankWhenNothingSeparates(t *testing.T) {
	p := ids(2)
	rows := tournament.Table(p, []domain.Match{
		bestOf(3, p[0], p[1], [2]int{11, 9}, [2]int{11, 8}),
	}, 0, tournament.ScoreByPoints)

	// One win against no win, one point against none: no tie here.
	if rows[0].Shared || rows[1].Shared {
		t.Errorf("a decided pair shares a rank: %d and %d", rows[0].Rank, rows[1].Rank)
	}
	if rows[0].TablePoints() != 3 || rows[1].TablePoints() != 0 {
		t.Errorf("points are %d and %d, want 3 and 0",
			rows[0].TablePoints(), rows[1].TablePoints())
	}
}
