package templates_test

import (
	"testing"

	"github.com/stuttgart-things/schmetterpause/internal/templates"
)

// The singular is the case that gets forgotten, and "1 Siege" is the kind of
// wrong that a reader hears rather than sees.
func TestARecordIsSpokenWithTheRightNumber(t *testing.T) {
	cases := []struct {
		won, lost int
		want      string
	}{
		{1, 0, "gegen Bodo: 1 Sieg, 0 Niederlagen"},
		{0, 1, "gegen Bodo: 0 Siege, 1 Niederlage"},
		{3, 2, "gegen Bodo: 3 Siege, 2 Niederlagen"},
	}
	for _, c := range cases {
		cell := templates.StatisticsCell{Won: c.won, Lost: c.lost, Opponent: "Bodo"}
		if got := cell.Spoken(); got != c.want {
			t.Errorf("Spoken() = %q, want %q", got, c.want)
		}
	}
}
