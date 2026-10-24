package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/stuttgart-things/schmetterpause/internal/seed"
)

// TestSeedRefusesAnUnknownFixtureBeforeTheDatabase runs the command the way a
// seed Job does, with a database address nothing listens on. An unknown
// SP_SEED_FIXTURE has to stop it with an error naming the variable — and
// before the connection, so the refusal is immediate rather than the end of a
// thirty-second wait for a database it would never have written to.
func TestSeedRefusesAnUnknownFixtureBeforeTheDatabase(t *testing.T) {
	t.Setenv("SP_DATABASE_URL", "postgres://nobody@127.0.0.1:1/nothing_test")
	t.Setenv("SP_DB_CONNECT_TIMEOUT", "1ms")
	t.Setenv("SP_SEED_FIXTURE", "pnog")

	err := run(t.Context(), []string{"seed"})
	if !errors.Is(err, seed.ErrUnknownFixture) {
		t.Fatalf("seed with SP_SEED_FIXTURE=pnog = %v, want ErrUnknownFixture", err)
	}
	if !strings.Contains(err.Error(), "SP_SEED_FIXTURE") {
		t.Errorf("the error %q does not name SP_SEED_FIXTURE", err)
	}
}
