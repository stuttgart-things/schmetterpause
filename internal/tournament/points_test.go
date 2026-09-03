package tournament_test

import (
	"testing"

	"github.com/stuttgart-things/schmetterpause/internal/tournament"
)

// Three a win, and that is the whole arithmetic.
func TestTablePointsAreThreeAWin(t *testing.T) {
	for _, won := range []int{0, 1, 4, 11} {
		row := tournament.TableRow{Won: won, Lost: 3}
		if got, want := row.TablePoints(), 3*won; got != want {
			t.Errorf("TablePoints() with %d wins = %d, want %d", won, got, want)
		}
	}
}

// The claim the whole setting rests on: counting in points cannot reorder a
// table, because a match cannot end level and the draw point is never
// awarded. If this ever fails, the setting stopped being a choice about what
// the table says and became one about who is above whom — which would need a
// decision, not a column.
func TestCountingInPointsCannotReorderATable(t *testing.T) {
	rows := []tournament.TableRow{
		{Won: 3, Lost: 2}, {Won: 3, Lost: 0}, {Won: 4, Lost: 1}, {Won: 0, Lost: 5},
	}
	for i, a := range rows {
		for j, b := range rows {
			byWins := cmp(a.Won, b.Won)
			byPoints := cmp(a.TablePoints(), b.TablePoints())
			if byWins != byPoints {
				t.Errorf("rows %d and %d order %d by wins and %d by points", i, j, byWins, byPoints)
			}
		}
	}
}

func cmp(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
