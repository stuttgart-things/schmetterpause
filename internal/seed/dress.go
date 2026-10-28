package seed

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand/v2"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/repository"
)

// Dress gives every player who has not chosen a figure a random one, and
// returns how many it dressed (docs/adr/0029).
//
// The fixtures dress their players only when they create them, and only on an
// empty database. A sandbox seeded before figures existed, or one whose
// players were added later by hand, shows a field of identical default
// figures that differ in the blade alone. This is the repair for that: unlike
// the fixtures it runs on a database in use, and it touches nothing but the
// figures of players who have none.
//
// A figure somebody chose is never replaced, and observers are left alone as
// the fixtures leave Pong: they never stand next to a score. So a second run
// is a no-op, and the sandbox's seed Job can run it after every seed.
//
// Each player's figure is rolled from their own id rather than from one
// sequence, so it does not depend on who else is in the database or in which
// order List returns them: the same player gets the same figure on any run.
//
// It is not meant for the office. Nothing stops it there, and it would dress
// everybody who kept the default; the default is a choice too, if a quiet one.
// The namespace the Job runs in is the guard, as it is for seed.
func Dress(ctx context.Context, store repository.Store) (int, error) {
	players, err := store.Players().List(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing the players: %w", err)
	}

	dressed := 0
	for _, p := range players {
		if p.IsObserver {
			continue
		}
		_, err := store.Avatars().Get(ctx, p.ID)
		if err == nil {
			continue
		}
		if !errors.Is(err, domain.ErrNotFound) {
			return dressed, fmt.Errorf("reading the figure of %s: %w", p.DisplayName, err)
		}
		// Between the read and the write a player could save a figure in the
		// editor, and this would replace it. A sandbox has no such player, and
		// a guarded write is not worth a repository method for a repair.
		if err := dress(ctx, store, p.ID, FiguresFor(p.ID)); err != nil {
			return dressed, fmt.Errorf("dressing %s: %w", p.DisplayName, err)
		}
		dressed++
	}
	return dressed, nil
}

// FiguresFor is the generator Dress rolls a player's figure from: seeded by
// the player's id, so it is the same figure on every run.
//
// Exported for the tests, which roll it to know what to expect.
func FiguresFor(id uuid.UUID) *rand.Rand {
	return rand.New(rand.NewPCG(binary.BigEndian.Uint64(id[:8]), binary.BigEndian.Uint64(id[8:])))
}
