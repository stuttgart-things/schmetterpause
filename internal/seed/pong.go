package seed

import (
	"context"
	"fmt"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/repository"
)

// pongBots are the players a sandbox plays against itself (docs/adr/0027,
// "Die Bot-Spieler"), created in this order.
//
// These names are a contract with zaehlwerk-pong: its PONG_PLAYER_SKILL gives
// each bot its strength by display name. Renaming one here does not fail
// anywhere — Pong simply no longer finds the name and that bot silently drops
// to Pong's default strength, which is exactly the spread the league exists to
// measure. Change a name on both sides or not at all.
//
// No strength, flag or column here: to this application a bot is a player
// like any other, and whether it is any good is for the rating to find out.
var pongBots = []string{
	"Ada",
	"Bert",
	"Cleo",
	"Dino",
	"Emil",
	"Frida",
	"Gero",
	"Hanna",
}

// pongOperator is the observer who keeps score for every sandbox match
// (docs/adr/0022, docs/adr/0023). The Zählwerk finds it by this name through
// GET /api/operators, which lists observers.
const pongOperator = "Pong"

// RunPong writes the pong fixture: the eight bots with their figures and the
// observer Pong, and nothing else — no results and no tournament. The league fills itself, and a
// ranking that starts with everybody on the same rating is the clearest view
// of whether the rating sorts by strength.
//
// Everybody starts on domain.DefaultTTR, as a player who joins does, and like
// the demo field nobody gets a PIN: the bots never sign in.
//
// It refuses a database that already has players with ErrNotEmpty.
func RunPong(ctx context.Context, store repository.Store) (Summary, error) {
	if err := refuseInUse(ctx, store); err != nil {
		return Summary{}, err
	}

	summary := Summary{Fixture: FixturePong}

	// The bots get random figures, as the demo players of the Zählwerk do
	// (docs/adr/0029). Pong, the observer, keeps the default: it never stands
	// next to a score.
	figures := Figures(FixturePong)
	for _, name := range pongBots {
		bot, err := store.Players().Create(ctx, name, domain.DefaultTTR)
		if err != nil {
			return Summary{}, fmt.Errorf("creating %s: %w", name, err)
		}
		if err := dress(ctx, store, bot.ID, figures); err != nil {
			return Summary{}, fmt.Errorf("dressing %s: %w", name, err)
		}
		summary.Players++
	}

	// Created as a player and then flagged, which is the only way an observer
	// comes into being (docs/adr/0022): the flag is refused for anybody with a
	// history, and a fresh account has none.
	operator, err := store.Players().Create(ctx, pongOperator, domain.DefaultTTR)
	if err != nil {
		return Summary{}, fmt.Errorf("creating the observer %s: %w", pongOperator, err)
	}
	summary.Players++
	if err := store.Players().SetObserver(ctx, operator.ID, true); err != nil {
		return Summary{}, fmt.Errorf("marking %s as an observer: %w", pongOperator, err)
	}
	summary.Observers++

	return summary, nil
}
