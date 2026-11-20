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
		rated   bool
		want    bool
	}{
		{0, true, false},
		{1, true, false},
		{2, true, true},
		{12, true, true},
		// Above the cap the picker should not have let them through, and the
		// button agrees rather than trusting it.
		{13, true, false},
		// Without a rating the cap is sixteen (docs/adr/0033).
		{13, false, true},
		{16, false, true},
		{17, false, false},
	}
	for _, c := range cases {
		v := templates.TournamentSizeView{Players: c.players, Rated: c.rated}
		if got := v.Enough(); got != c.want {
			t.Errorf("Enough() with %d players, rated=%v = %v, want %v", c.players, c.rated, got, c.want)
		}
	}
}

// Between the two caps a rated field is one tick from playable, and the
// sentence says so instead of only that it is too many.
func TestTheSizeNamesTheUnratedWayOut(t *testing.T) {
	cases := []struct {
		players int
		rated   bool
		want    bool
	}{
		{12, true, false},
		{13, true, true},
		{16, true, true},
		{17, true, false},
		{13, false, false},
	}
	for _, c := range cases {
		v := templates.TournamentSizeView{Players: c.players, Rated: c.rated}
		if got := v.OnlyUnrated(); got != c.want {
			t.Errorf("OnlyUnrated() with %d players, rated=%v = %v, want %v", c.players, c.rated, got, c.want)
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
