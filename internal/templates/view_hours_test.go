package templates_test

import (
	"testing"

	"github.com/stuttgart-things/schmetterpause/internal/templates"
)

// The number the cap on the field exists for: eight players against each
// other once is 28 matches, and that is seven hours of table time (#41).
func TestTournamentHoursReadsLikeSomebodySayingIt(t *testing.T) {
	for _, tc := range []struct {
		matches int
		want    string
	}{
		{0, "keine halbe Stunde"},
		{1, "eine halbe Stunde"},
		{2, "eine halbe Stunde"},
		{3, "eine Stunde"},
		{6, "1½ Stunden"},
		{28, "7 Stunden"},
		{56, "14 Stunden"},
	} {
		if got := templates.TournamentHours(tc.matches); got != tc.want {
			t.Errorf("TournamentHours(%d) = %q, want %q", tc.matches, got, tc.want)
		}
	}
}
