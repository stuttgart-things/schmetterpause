package server

import (
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// history builds a rating history, oldest first, from a starting value and
// the ratings after each match.
func history(start int, after ...int) []domain.TTRChange {
	changes := make([]domain.TTRChange, 0, len(after))
	before := start
	for _, a := range after {
		changes = append(changes, domain.TTRChange{
			ID: uuid.New(), TTRBefore: before, TTRAfter: a,
		})
		before = a
	}
	return changes
}

func pointsOf(t *testing.T, s string) [][2]float64 {
	t.Helper()

	var out [][2]float64
	for _, pair := range strings.Fields(s) {
		x, y, ok := strings.Cut(pair, ",")
		if !ok {
			t.Fatalf("point %q has no comma", pair)
		}
		px, err := strconv.ParseFloat(x, 64)
		if err != nil {
			t.Fatalf("x of %q: %v", pair, err)
		}
		py, err := strconv.ParseFloat(y, 64)
		if err != nil {
			t.Fatalf("y of %q: %v", pair, err)
		}
		out = append(out, [2]float64{px, py})
	}
	return out
}

func TestSparkWithoutHistory(t *testing.T) {
	if got := buildSpark(nil); got.Show {
		t.Error("a player with no matches got a chart")
	}
}

// TestSparkStartsWhereThePlayerStarted checks the extra point: the series
// opens with the rating before the first match, not after it, or a first win
// would look like the starting position.
func TestSparkStartsWhereThePlayerStarted(t *testing.T) {
	spark := buildSpark(history(1000, 1008))

	if !spark.Show {
		t.Fatal("Show = false for a player with one match")
	}
	points := pointsOf(t, spark.Points)
	if len(points) != 2 {
		t.Fatalf("%d points for one match, want 2", len(points))
	}
	if spark.Low != 1000 || spark.High != 1008 {
		t.Errorf("range = %d to %d, want 1000 to 1008", spark.Low, spark.High)
	}
}

func TestSparkSpansTheBox(t *testing.T) {
	spark := buildSpark(history(1000, 1008, 992, 1015))

	points := pointsOf(t, spark.Points)
	if len(points) != 4 {
		t.Fatalf("%d points for three matches, want 4", len(points))
	}

	// The line runs from the left padding to the right, and the highest
	// rating sits at the top of the usable area while the lowest sits at the
	// bottom. Anything else means the scaling is inverted or clipped.
	if points[0][0] != sparkPad {
		t.Errorf("first x = %v, want %d", points[0][0], sparkPad)
	}
	if points[len(points)-1][0] != sparkWidth-sparkPad {
		t.Errorf("last x = %v, want %d", points[len(points)-1][0], sparkWidth-sparkPad)
	}

	var lowestY, highestY float64 = -1, -1
	for i, v := range []int{1000, 1008, 992, 1015} {
		switch v {
		case spark.High:
			highestY = points[i][1]
		case spark.Low:
			lowestY = points[i][1]
		}
	}
	if highestY != sparkPad {
		t.Errorf("the highest rating sits at y=%v, want %d", highestY, sparkPad)
	}
	if lowestY != sparkHeight-sparkPad {
		t.Errorf("the lowest rating sits at y=%v, want %d", lowestY, sparkHeight-sparkPad)
	}
}

// TestSparkWithAFlatHistory is the division-by-zero case: every rating the
// same leaves no range to scale against.
func TestSparkWithAFlatHistory(t *testing.T) {
	spark := buildSpark(history(1000, 1000, 1000))

	if !spark.Show {
		t.Fatal("Show = false for a flat history")
	}
	if spark.Low != 1000 || spark.High != 1000 {
		t.Errorf("range = %d to %d, want 1000 to 1000", spark.Low, spark.High)
	}
	for _, p := range pointsOf(t, spark.Points) {
		if p[1] != sparkHeight/2 {
			t.Errorf("a flat history sits at y=%v, want the middle at %d", p[1], sparkHeight/2)
		}
	}
}

func TestSparkMarksTheCurrentRating(t *testing.T) {
	spark := buildSpark(history(1000, 1008, 1015))

	points := pointsOf(t, spark.Points)
	last := points[len(points)-1]

	if spark.LastX != coord(last[0]) || spark.LastY != coord(last[1]) {
		t.Errorf("the marker sits at (%s, %s), want the last point (%v, %v)",
			spark.LastX, spark.LastY, last[0], last[1])
	}
}
