package seed

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/stuttgart-things/schmetterpause/internal/repository"
)

// Fixture names what schmetterpause seed writes. It is chosen by
// SP_SEED_FIXTURE (docs/adr/0027).
type Fixture string

const (
	// FixtureDemo is the believable office field for previews and
	// screenshots: players, results in every state and an open tournament.
	// It is the default, so a seed Job that sets nothing keeps writing what
	// it always wrote.
	FixtureDemo Fixture = "demo"
	// FixturePong is the sandbox's field: eight bot players with no results,
	// and the observer Pong who keeps score for every one of their matches.
	FixturePong Fixture = "pong"
)

// ErrUnknownFixture is the refusal of a fixture name this binary does not
// have.
//
// A refusal rather than a fallback to demo: a typo in SP_SEED_FIXTURE that
// quietly wrote the demo field into a sandbox would fill it with six strangers
// and a tournament — and the guard would then refuse every later, corrected
// run, because the database is no longer empty.
var ErrUnknownFixture = errors.New("unknown seed fixture")

// ParseFixture resolves a fixture name. The empty string is the default,
// FixtureDemo; anything else that is not a fixture's exact name is refused
// with ErrUnknownFixture.
func ParseFixture(name string) (Fixture, error) {
	switch f := Fixture(name); f {
	case "":
		return FixtureDemo, nil
	case FixtureDemo, FixturePong:
		return f, nil
	default:
		return "", fmt.Errorf("%w %q, want %q or %q", ErrUnknownFixture, name, FixtureDemo, FixturePong)
	}
}

// RunFixture writes the given fixture. now is passed on to the fixtures that
// date anything; pong dates nothing.
//
// Every fixture refuses a database that already has players with ErrNotEmpty,
// and an unknown fixture is refused with ErrUnknownFixture before the
// database is asked anything.
func RunFixture(ctx context.Context, store repository.Store, fixture Fixture, now time.Time) (Summary, error) {
	switch fixture {
	case FixtureDemo:
		return Run(ctx, store, now)
	case FixturePong:
		return RunPong(ctx, store)
	default:
		return Summary{}, fmt.Errorf("%w %q", ErrUnknownFixture, fixture)
	}
}
