package seed_test

import (
	"errors"
	"testing"

	"github.com/stuttgart-things/schmetterpause/internal/seed"
)

// These need no database, unlike the rest of the package's tests: choosing a
// fixture happens before anything is written, which is the point of it.

func TestAnUnsetFixtureIsTheDemo(t *testing.T) {
	got, err := seed.ParseFixture("")
	if err != nil {
		t.Fatalf("ParseFixture(\"\"): %v", err)
	}
	if got != seed.FixtureDemo {
		t.Errorf("ParseFixture(\"\") = %q, want %q: a seed Job that sets nothing keeps writing the demo field",
			got, seed.FixtureDemo)
	}
}

func TestBothFixturesAreChosenByName(t *testing.T) {
	for _, want := range []seed.Fixture{seed.FixtureDemo, seed.FixturePong} {
		got, err := seed.ParseFixture(string(want))
		if err != nil {
			t.Errorf("ParseFixture(%q): %v", want, err)
			continue
		}
		if got != want {
			t.Errorf("ParseFixture(%q) = %q", want, got)
		}
	}
}

// TestAnUnknownFixtureIsRefusedRatherThanReplacedByTheDemo is the reason
// there is no fallback: a typo that wrote the demo field into a sandbox would
// leave a database the guard then refuses to correct.
func TestAnUnknownFixtureIsRefusedRatherThanReplacedByTheDemo(t *testing.T) {
	for _, name := range []string{"pnog", "Pong", "DEMO", "demo ", "bots"} {
		got, err := seed.ParseFixture(name)
		if !errors.Is(err, seed.ErrUnknownFixture) {
			t.Errorf("ParseFixture(%q) = %q, %v; want ErrUnknownFixture", name, got, err)
		}
	}
}

// TestRunFixtureRefusesAnUnknownFixtureBeforeTheStore passes no store at all:
// a fixture that does not exist must not get as far as asking the database
// whether it is empty.
func TestRunFixtureRefusesAnUnknownFixtureBeforeTheStore(t *testing.T) {
	_, err := seed.RunFixture(t.Context(), nil, seed.Fixture("pnog"), reference)
	if !errors.Is(err, seed.ErrUnknownFixture) {
		t.Fatalf("RunFixture(pnog) = %v, want ErrUnknownFixture", err)
	}
}
