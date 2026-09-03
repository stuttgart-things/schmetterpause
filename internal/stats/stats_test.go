package stats_test

import (
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/stats"
)

func ids(n int) []uuid.UUID {
	out := make([]uuid.UUID, n)
	for i := range out {
		out[i] = uuid.MustParse(fmt.Sprintf("00000000-0000-0000-0000-%012d", i+1))
	}
	return out
}

// confirmed builds a confirmed match, so the tests read as results rather than
// as struct literals. pointsToWin is 11 unless a test says otherwise.
func confirmed(home, away uuid.UUID, sets ...[2]int) domain.Match {
	m := domain.Match{
		HomeID: home, AwayID: away,
		Status: domain.MatchConfirmed, BestOf: 3, PointsToWin: 11,
	}
	for i, s := range sets {
		m.Sets = append(m.Sets, domain.MatchSet{
			SetNo: i + 1, HomePoints: s[0], AwayPoints: s[1],
		})
	}
	return m
}

func TestMatrixRecordsBothSidesOfAResult(t *testing.T) {
	p := ids(2)
	anna, bodo := p[0], p[1]

	rows := stats.Matrix(p, []domain.Match{
		confirmed(anna, bodo, [2]int{11, 5}, [2]int{11, 7}),
		confirmed(bodo, anna, [2]int{11, 5}, [2]int{11, 7}),
		confirmed(anna, bodo, [2]int{11, 5}, [2]int{11, 7}),
	})

	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	if got := rows[0].Cells[1]; got.Won != 2 || got.Lost != 1 {
		t.Errorf("Anna against Bodo = %d:%d, want 2:1", got.Won, got.Lost)
	}
	// The mirror image is redundant on purpose: somebody looking up their own
	// standing should find it on their own line, not have to transpose.
	if got := rows[1].Cells[0]; got.Won != 1 || got.Lost != 2 {
		t.Errorf("Bodo against Anna = %d:%d, want 1:2", got.Won, got.Lost)
	}
	if rows[0].Won != 2 || rows[0].Lost != 1 {
		t.Errorf("Anna's row total = %d:%d, want 2:1", rows[0].Won, rows[0].Lost)
	}
}

// Orientation must not decide the winner. The draw alternates home and away by
// round, and nobody at the table checks which name the app printed first.
func TestMatrixIgnoresWhichSideWasEnteredFirst(t *testing.T) {
	p := ids(2)
	anna, bodo := p[0], p[1]

	// The same result, entered from each side.
	fromAnna := stats.Matrix(p, []domain.Match{confirmed(anna, bodo, [2]int{11, 5}, [2]int{11, 5})})
	fromBodo := stats.Matrix(p, []domain.Match{confirmed(bodo, anna, [2]int{5, 11}, [2]int{5, 11})})

	for _, rows := range []([]stats.Row){fromAnna, fromBodo} {
		if got := rows[0].Cells[1]; got.Won != 1 || got.Lost != 0 {
			t.Errorf("Anna against Bodo = %d:%d, want 1:0", got.Won, got.Lost)
		}
	}
}

func TestMatrixMarksTheDiagonalAndLeavesItEmpty(t *testing.T) {
	p := ids(3)
	rows := stats.Matrix(p, []domain.Match{confirmed(p[0], p[1], [2]int{11, 0})})

	for i, row := range rows {
		if !row.Cells[i].Self {
			t.Errorf("row %d: the diagonal cell is not marked Self", i)
		}
		if row.Cells[i].Played() {
			t.Errorf("row %d: a player has a record against themselves", i)
		}
	}
}

// A result nobody agreed to is not a result, and a match against somebody
// outside the list is not this matrix's business.
func TestMatrixIgnoresUnconfirmedAndOutsiders(t *testing.T) {
	p := ids(2)
	outsider := ids(3)[2]

	pending := confirmed(p[0], p[1], [2]int{11, 0}, [2]int{11, 0})
	pending.Status = domain.MatchPending
	disputed := confirmed(p[0], p[1], [2]int{11, 0}, [2]int{11, 0})
	disputed.Status = domain.MatchDisputed

	rows := stats.Matrix(p, []domain.Match{
		pending, disputed,
		confirmed(p[0], outsider, [2]int{11, 0}, [2]int{11, 0}),
	})

	for i, row := range rows {
		if row.Won+row.Lost != 0 {
			t.Errorf("row %d counted %d matches, want 0", i, row.Won+row.Lost)
		}
	}
}

func TestMatrixDropsDuplicatePlayers(t *testing.T) {
	p := ids(2)
	rows := stats.Matrix([]uuid.UUID{p[0], p[1], p[0]}, nil)

	if len(rows) != 2 {
		t.Fatalf("got %d rows for two distinct players, want 2", len(rows))
	}
	for i, row := range rows {
		if len(row.Cells) != 2 {
			t.Errorf("row %d has %d cells, want 2 — the matrix is not square", i, len(row.Cells))
		}
	}
}

func TestComputeCountsSetsPointsAndMatches(t *testing.T) {
	p := ids(2)
	got := stats.Compute([]domain.Match{
		confirmed(p[0], p[1], [2]int{11, 5}, [2]int{9, 11}, [2]int{11, 7}),
		confirmed(p[1], p[0], [2]int{11, 0}, [2]int{11, 0}),
	})

	if got.Matches != 2 {
		t.Errorf("Matches = %d, want 2", got.Matches)
	}
	if got.Sets != 5 {
		t.Errorf("Sets = %d, want 5", got.Sets)
	}
	if want := 11 + 5 + 9 + 11 + 11 + 7 + 11 + 0 + 11 + 0; got.Points != want {
		t.Errorf("Points = %d, want %d", got.Points, want)
	}
}

// A set is a deuce when both players reached points_to_win minus one, because
// only then did it need a two-point gap to end. 11:9 is an ordinary win and
// must not be counted.
func TestComputeCountsDeuceSets(t *testing.T) {
	p := ids(2)

	toEleven := stats.Compute([]domain.Match{
		confirmed(p[0], p[1], [2]int{11, 9}, [2]int{13, 11}, [2]int{12, 10}),
	})
	if toEleven.Deuce != 2 {
		t.Errorf("Deuce = %d, want 2 — 11:9 is not a deuce, 13:11 and 12:10 are", toEleven.Deuce)
	}

	// points_to_win is per match: 20:20 in a set to 21 is the same event.
	toTwentyOne := confirmed(p[0], p[1], [2]int{21, 19}, [2]int{23, 21})
	toTwentyOne.PointsToWin = 21
	if got := stats.Compute([]domain.Match{toTwentyOne}); got.Deuce != 1 {
		t.Errorf("Deuce = %d for a match to 21, want 1", got.Deuce)
	}
}

func TestComputeFindsTheLongestSet(t *testing.T) {
	p := ids(2)
	got := stats.Compute([]domain.Match{
		confirmed(p[0], p[1], [2]int{11, 5}, [2]int{11, 7}),
		confirmed(p[1], p[0], [2]int{19, 17}, [2]int{11, 0}),
	})

	if got.LongestSet != 36 {
		t.Errorf("LongestSet = %d, want 36", got.LongestSet)
	}
	if got.LongestSetHome != 19 || got.LongestSetAway != 17 {
		t.Errorf("longest set = %d:%d, want 19:17", got.LongestSetHome, got.LongestSetAway)
	}
}

func TestComputeCountsWhitewashes(t *testing.T) {
	p := ids(2)
	got := stats.Compute([]domain.Match{
		confirmed(p[0], p[1], [2]int{11, 5}, [2]int{11, 7}),                // 2:0
		confirmed(p[0], p[1], [2]int{11, 5}, [2]int{9, 11}, [2]int{11, 7}), // 2:1
		confirmed(p[1], p[0], [2]int{11, 0}),                               // one set
	})

	if got.Whitewashes != 2 {
		t.Errorf("Whitewashes = %d, want 2 — the three-setter is not one", got.Whitewashes)
	}
}

// A match played over one set is left out, and that is the point of the
// number rather than a rounding of it: there, winning without dropping a set
// is what winning is. An office that plays mostly one-setters would otherwise
// read "41 of 50 one-sided", which describes the mode and not the matches.
func TestASingleSetMatchIsNoWhitewash(t *testing.T) {
	p := ids(2)
	one := confirmed(p[0], p[1], [2]int{11, 2})
	one.BestOf = 1

	if got := stats.Compute([]domain.Match{one}); got.Whitewashes != 0 {
		t.Errorf("Whitewashes = %d, want 0 for a single-set match", got.Whitewashes)
	}

	// The same score over three sets still is one, so the exclusion is about
	// the mode and not about the number of sets that happened to be played.
	three := confirmed(p[0], p[1], [2]int{11, 2}, [2]int{11, 4})
	if got := stats.Compute([]domain.Match{three}); got.Whitewashes != 1 {
		t.Errorf("Whitewashes = %d, want 1 for a 2:0 in best of three", got.Whitewashes)
	}
}

// Before the first confirmed match every figure is zero rather than a number
// nobody earned. The page has to be able to say "nothing yet".
func TestComputeOnNothingIsAllZero(t *testing.T) {
	got := stats.Compute(nil)
	if got != (stats.Totals{}) {
		t.Errorf("Compute(nil) = %+v, want the zero value", got)
	}
}

// A total that moves backwards when somebody disputes a result reads as a bug,
// so unconfirmed matches are not counted provisionally.
func TestComputeIgnoresUnconfirmed(t *testing.T) {
	p := ids(2)
	pending := confirmed(p[0], p[1], [2]int{11, 0}, [2]int{11, 0})
	pending.Status = domain.MatchPending

	if got := stats.Compute([]domain.Match{pending}); got != (stats.Totals{}) {
		t.Errorf("a pending match was counted: %+v", got)
	}
}
