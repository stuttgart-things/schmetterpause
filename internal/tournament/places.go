package tournament

import (
	"errors"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// Field is what of a tournament decides its places: who plays, in which
// order, how often, whether a final follows and how the table is read. A
// value of its own so this package stays free of the tournament's storage.
type Field struct {
	Players   []uuid.UUID
	Legs      int
	WithFinal bool
	Scoring   Scoring
}

// Place is one slot of a draw: a pairing in a round. The final is the round
// after the group, and Final says so.
type Place struct {
	Round   int
	Pairing Pairing
	Final   bool
}

// Why a result does not fit a place. The caller words them; the Zählwerk
// gets English, the kiosk German, and both come from the same check.
var (
	// ErrPlaceFilled is a place that already holds a result, in any status.
	// The likeliest cause is a retry of a result that did arrive, or the
	// same place entered at the kiosk meanwhile.
	ErrPlaceFilled = errors.New("this place already holds a result")
	// ErrFinalUndecided is the final's round before the group is played out
	// or while the table cannot separate the top two.
	ErrFinalUndecided = errors.New("the final is not decided yet")
	// ErrNotTheFinalists is the final's round with two players the table did
	// not name.
	ErrNotTheFinalists = errors.New("these two do not play the final")
	// ErrNotInDraw is a pair the draw does not have in that round.
	ErrNotInDraw = errors.New("the draw has no such pairing in this round")
)

// OpenPlaces lists the places nobody has entered a result for yet, in the
// order of the draw, the final last once it is decided.
//
// booked is every match of the tournament, as the repository returns them. A
// place counts as filled whatever its result's status: a pending result holds
// its place, or two would be entered for it before anybody confirmed the
// first.
//
// This is the one computation of "what may still be played". The pairings
// endpoint offers exactly these, and CheckPlace accepts exactly these
// (docs/adr/0028), so the Zählwerk is never offered a place a result could not
// fill.
func OpenPlaces(f Field, booked []domain.Match) []Place {
	filled := filledPlaces(booked)
	groupRounds := GroupRounds(len(f.Players), f.Legs)

	var open []Place
	for _, round := range Draw(f.Players, f.Legs) {
		for _, p := range round.Pairings {
			if !filled.has(round.No, p) {
				open = append(open, Place{Round: round.No, Pairing: p})
			}
		}
	}
	if pairing, ok := decidedFinal(f, booked, groupRounds); ok && !filled.hasFinal(groupRounds+1, pairing) {
		open = append(open, Place{Round: groupRounds + 1, Pairing: pairing, Final: true})
	}
	return open
}

// CheckPlace says whether a result between home and away may fill this round,
// and if not, which of the errors above is the reason. Orientation does not
// matter: a pair entered the other way round still finds its pairing.
func CheckPlace(f Field, booked []domain.Match, home, away uuid.UUID, round int) error {
	pair := Pairing{Home: home, Away: away}
	filled := filledPlaces(booked)
	groupRounds := GroupRounds(len(f.Players), f.Legs)
	if f.WithFinal && round == groupRounds+1 {
		if filled.hasFinal(round, pair) {
			return ErrPlaceFilled
		}
	} else if filled.has(round, pair) {
		return ErrPlaceFilled
	}
	for _, place := range OpenPlaces(f, booked) {
		if place.Round == round && samePair(place.Pairing, pair) {
			return nil
		}
	}

	if f.WithFinal && round == groupRounds+1 {
		if _, ok := decidedFinal(f, booked, groupRounds); !ok {
			return ErrFinalUndecided
		}
		return ErrNotTheFinalists
	}
	return ErrNotInDraw
}

// decidedFinal is the final's pairing once it stands: every group place holds
// a confirmed result and the table separates the top two.
//
// The group has to be played out, not merely led. Final alone would name the
// two in front after the first evening's match, and a final played then is a
// final between whoever happened to start (docs/adr/0011).
func decidedFinal(f Field, booked []domain.Match, groupRounds int) (Pairing, bool) {
	if !f.WithFinal {
		return Pairing{}, false
	}
	confirmed := 0
	for _, m := range booked {
		if m.Status != domain.MatchConfirmed {
			continue
		}
		// A match with no round is from before slots existed, and therefore
		// a group match.
		if m.TournamentRound == nil || *m.TournamentRound <= groupRounds {
			confirmed++
		}
	}
	if confirmed < Matches(len(f.Players), f.Legs, false) {
		return Pairing{}, false
	}
	return Final(Table(f.Players, booked, groupRounds, f.Scoring))
}

// places is the set of filled places, by round and unordered pair.
type places struct {
	slots map[placeKey]bool
	// legacy is the pairs of results from before rounds existed. Those
	// tournaments are single round robins, where each pair meets once, so
	// the pair alone names the place.
	legacy map[[2]uuid.UUID]bool
}

type placeKey struct {
	round int
	pair  [2]uuid.UUID
}

func filledPlaces(booked []domain.Match) places {
	p := places{slots: map[placeKey]bool{}, legacy: map[[2]uuid.UUID]bool{}}
	for _, m := range booked {
		pair := unordered(m.HomeID, m.AwayID)
		if m.TournamentRound == nil {
			p.legacy[pair] = true
			continue
		}
		p.slots[placeKey{round: *m.TournamentRound, pair: pair}] = true
	}
	return p
}

func (p places) has(round int, pairing Pairing) bool {
	pair := unordered(pairing.Home, pairing.Away)
	return p.slots[placeKey{round: round, pair: pair}] || p.legacy[pair]
}

// hasFinal is has for the final's round. A result from before rounds existed
// is never a final: those tournaments had none, so it cannot fill one.
func (p places) hasFinal(round int, pairing Pairing) bool {
	return p.slots[placeKey{round: round, pair: unordered(pairing.Home, pairing.Away)}]
}

func samePair(a, b Pairing) bool {
	return unordered(a.Home, a.Away) == unordered(b.Home, b.Away)
}

func unordered(a, b uuid.UUID) [2]uuid.UUID {
	if a.String() > b.String() {
		a, b = b, a
	}
	return [2]uuid.UUID{a, b}
}
