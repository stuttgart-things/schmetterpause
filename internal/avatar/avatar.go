// Package avatar is the figure a player assembles from the mascot's parts:
// fur, headdress, handle and blade colour, and optionally a piece of bling
// (docs/adr/0029).
//
// The face is not a choice. It belongs to the game: the Zählwerk decides it
// from the score, and wherever the figure stands next to a running match it
// shows the face it is told. Everywhere else it is neutral.
//
// The package has no database and no HTTP dependency, as the "Konventionen"
// section of CLAUDE.md asks of domain logic. It knows which choices exist,
// what a player who has not chosen looks like, how to roll a random figure
// and how to put one together as SVG. Where a choice is stored is the
// repository's business, and who may change it the handler's.
package avatar

import (
	"errors"
	"fmt"
	"hash/fnv"
	"math/rand/v2"
)

// Avatar is one figure, as keys into the palettes and option lists. The zero
// value is not a valid figure; Default is the figure of somebody who has not
// chosen.
type Avatar struct {
	Fur       string
	Headdress string
	Handle    string
	Blade     string
	Bling     string
}

// ErrUnknown is returned by Validate for a key no palette has.
var ErrUnknown = errors.New("avatar: unknown choice")

// Default is what a player looks like who has not chosen: the mascot as it is
// drawn, with the blade derived from the id the way templates.PaddleClass has
// always derived it. Nobody looks different the day choosing arrives.
//
// An empty id is nobody, and nobody keeps the drawing's red: Blade is empty.
func Default(playerID string) Avatar {
	a := Avatar{
		Fur:       Furs[0].Key,
		Headdress: SameAsFur,
		Handle:    SameAsFur,
		Bling:     NoBling,
	}
	if playerID != "" {
		a.Blade = Blades[BladeIndex(playerID)].Key
	}
	return a
}

// BladeIndex is the blade a player gets without choosing, as an index into
// Blades. FNV-1a over the id: stable across builds and machines, and the
// same function templates.PaddleClass and zaehlwerk use, which a test holds
// this to.
func BladeIndex(playerID string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(playerID))
	return int(h.Sum32() % uint32(len(Blades)))
}

// Fill replaces every part a does not name, or names with a key that no
// longer exists, by the player's default. A palette entry that is taken out
// again is therefore harmless: what was stored falls back rather than
// breaking the page (docs/adr/0029).
func Fill(a Avatar, playerID string) Avatar {
	d := Default(playerID)
	if _, ok := findColour(Furs, a.Fur); !ok {
		a.Fur = d.Fur
	}
	if _, ok := findColour(Headdresses, a.Headdress); !ok {
		a.Headdress = d.Headdress
	}
	if _, ok := findColour(Handles, a.Handle); !ok {
		a.Handle = d.Handle
	}
	if _, ok := findColour(Blades, a.Blade); !ok {
		a.Blade = d.Blade
	}
	if !hasOption(Blings, a.Bling) {
		a.Bling = d.Bling
	}
	return a
}

// Validate reports whether every part of a is a key the palettes know. It is
// for what a player submits, where an unknown key is a mistake to answer
// rather than something to paper over as Fill does for stored values.
func Validate(a Avatar) error {
	checks := []struct {
		part string
		key  string
		ok   bool
	}{
		{"fur", a.Fur, has(Furs, a.Fur)},
		{"headdress", a.Headdress, has(Headdresses, a.Headdress)},
		{"handle", a.Handle, has(Handles, a.Handle)},
		{"blade", a.Blade, has(Blades, a.Blade)},
		{"bling", a.Bling, hasOption(Blings, a.Bling)},
	}
	for _, c := range checks {
		if !c.ok {
			return fmt.Errorf("%w: %s %q", ErrUnknown, c.part, c.key)
		}
	}
	return nil
}

func has(p []Colour, key string) bool {
	_, ok := findColour(p, key)
	return ok
}

// Random rolls every part uniformly. The caller owns the generator: the demo
// field seeds it so two runs dress the same players the same way, the dice
// button in the editor and zaehlwerk's demo matches do not.
func Random(r *rand.Rand) Avatar {
	return Avatar{
		Fur:       Furs[r.IntN(len(Furs))].Key,
		Headdress: Headdresses[r.IntN(len(Headdresses))].Key,
		Handle:    Handles[r.IntN(len(Handles))].Key,
		Blade:     Blades[r.IntN(len(Blades))].Key,
		Bling:     Blings[r.IntN(len(Blings))].Key,
	}
}
