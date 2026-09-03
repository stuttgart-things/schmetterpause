package templates_test

import (
	"testing"

	"github.com/stuttgart-things/schmetterpause/internal/templates"
)

// The button is shut until the form describes a tournament that could exist.
// The server refuses either way; a form that only says no after being
// submitted is one somebody submits.
func TestTheFormOnlyOpensForAFieldThatCouldPlay(t *testing.T) {
	cases := []struct {
		players int
		want    bool
	}{
		{0, false},
		{1, false},
		{2, true},
		{12, true},
		// Above the cap the picker should not have let them through, and the
		// button agrees rather than trusting it.
		{13, false},
	}
	for _, c := range cases {
		v := templates.TournamentSizeView{Players: c.players, MaxPlayers: 12}
		if got := v.Enough(); got != c.want {
			t.Errorf("Enough() with %d players = %v, want %v", c.players, got, c.want)
		}
	}
}

// The mode travels with the form, not the label: the fragment picks its own
// words from a fixed set rather than printing back what a request handed it.
func TestTheSubmitLabelComesFromAFixedSet(t *testing.T) {
	if got := templates.TournamentSubmitLabel(templates.TournamentFormEdit); got != "Änderung speichern" {
		t.Errorf("edit label = %q", got)
	}
	if got := templates.TournamentSubmitLabel(templates.TournamentFormCreate); got != "Turnier anlegen" {
		t.Errorf("create label = %q", got)
	}
	// Anything else is the form somebody would have arrived from.
	if got := templates.TournamentSubmitLabel(`"><script>`); got != "Turnier anlegen" {
		t.Errorf("an unknown mode printed %q instead of falling back", got)
	}
}
