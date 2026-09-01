// Package tournament owns the two pieces of a round robin that are easy to
// get quietly wrong: who plays whom in which round, and where everybody
// stands once results arrive.
//
// No database, no HTTP, plain values only — the same separation the ttr,
// match and standings packages keep (CLAUDE.md, "Konventionen").
//
// What this package deliberately does NOT do is rate anything. A tournament
// match settles through scoring.Confirm like any other, one match at a time.
// That departs from the "veranstaltungsweise Wertung" in CLAUDE.md and is
// recorded as a decision in docs/adr/0009 rather than left to be discovered
// here.
package tournament

import (
	"cmp"
	"slices"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// Pairing is one encounter in the draw.
//
// Home and Away are an orientation, not a venue: they decide which player's
// points go in the left column of the entry form. The draw alternates them so
// the same person is not listed first in every round.
type Pairing struct {
	Home uuid.UUID
	Away uuid.UUID
}

// Round is the set of pairings that can be played at the same time. With one
// table they are a queue rather than a schedule (#41: a draw is not a
// schedule, and a calendar for one table is mostly ceremony), but the grouping
// is still what tells everybody how far along the evening is.
type Round struct {
	// No is the round number, counting from 1.
	No int
	// Pairings are the encounters of this round.
	Pairings []Pairing
	// Bye is the player sitting this round out, or uuid.Nil when nobody
	// does. Only an odd field can produce one.
	Bye uuid.UUID
}

// Draw builds a full round robin: everybody plays everybody exactly once.
//
// The circle method — fix the first player, rotate the rest — which is the
// reason this is twenty lines rather than a scheduling problem. With an odd
// number of players a placeholder joins the circle and whoever draws it sits
// that round out, so every player gets exactly one bye across the tournament.
//
// The order of players is taken as given and NOT shuffled. That keeps this
// function deterministic and therefore testable; a caller who wants a fresh
// draw shuffles before calling. Randomness inside a pure function is the one
// thing that would make this package need a seed to be worth testing.
//
// Fewer than two players is an empty draw rather than an error: a tournament
// somebody is still adding people to is a normal intermediate state, and a
// form that refuses to show a preview until it is complete is worse than one
// that shows nothing yet.
func Draw(players []uuid.UUID) []Round {
	if len(players) < 2 {
		return nil
	}

	// The circle works on an even field. The placeholder is uuid.Nil, which
	// no player can be, so "did somebody draw the bye" is a comparison rather
	// than a parallel bookkeeping slice.
	circle := slices.Clone(players)
	if len(circle)%2 == 1 {
		circle = append(circle, uuid.Nil)
	}

	n := len(circle)
	rounds := make([]Round, 0, n-1)

	for r := range n - 1 {
		round := Round{No: r + 1, Pairings: make([]Pairing, 0, n/2)}

		for i := range n / 2 {
			home, away := circle[i], circle[n-1-i]

			if home == uuid.Nil || away == uuid.Nil {
				// Whoever was paired with the placeholder sits out.
				round.Bye = home
				if home == uuid.Nil {
					round.Bye = away
				}
				continue
			}

			// Alternate the orientation by round so the same player is not
			// listed on the left every time. Cosmetic for the result, but the
			// draw is printed and read by people.
			if r%2 == 1 {
				home, away = away, home
			}
			round.Pairings = append(round.Pairings, Pairing{Home: home, Away: away})
		}

		rounds = append(rounds, round)
		rotate(circle)
	}

	return rounds
}

// rotate turns the circle one step: the first entry stays put, the rest move
// round. That fixed point is what makes the method produce every pair exactly
// once instead of repeating.
func rotate(circle []uuid.UUID) {
	if len(circle) < 3 {
		return
	}
	last := circle[len(circle)-1]
	copy(circle[2:], circle[1:len(circle)-1])
	circle[1] = last
}

// Matches is how many encounters a field of n produces. n*(n-1)/2, and it is
// here because the number is worth showing before anybody commits to an
// evening: eight players is twenty-eight matches, which is seven hours of
// table time at a quarter of an hour each (#41).
func Matches(n int) int {
	if n < 2 {
		return 0
	}
	return n * (n - 1) / 2
}

// TableRow is one line of the tournament table.
type TableRow struct {
	PlayerID uuid.UUID
	// Played, Won and Lost count confirmed matches only, for the same reason
	// the ranking does: a result nobody has agreed to is not a result.
	Played, Won, Lost int
	SetsWon, SetsLost int
	PointsFor         int
	PointsAgainst     int
	// Rank counts from 1. Players who cannot be separated share one, and the
	// next distinct position skips the ones they used up — two players on
	// rank 1 are followed by rank 3. Same convention as the overall ranking.
	Rank int
	// Shared marks a rank more than one player holds, so the table can say so
	// rather than leaving two identical numbers to explain themselves.
	Shared bool
}

// SetDiff and PointDiff are the tie-breakers, exposed because the table shows
// them.
func (r TableRow) SetDiff() int   { return r.SetsWon - r.SetsLost }
func (r TableRow) PointDiff() int { return r.PointsFor - r.PointsAgainst }

// Table builds the tournament table from the confirmed matches played so far.
//
// participants decides who appears: somebody who has not played yet is a row
// of zeroes, not an absence. A table that shows only the people with results
// answers "who is in this tournament" wrongly for the first twenty minutes of
// every evening.
//
// matches may contain anything; only confirmed ones between two participants
// are counted. Passing the unfiltered list is the ordinary case, and a filter
// the caller has to remember is a filter somebody eventually forgets.
//
// # How ties are broken
//
// Match wins first. Where that leaves players level, the results *among those
// players* decide — the sub-table — which is the official rule and also the
// one people at the table expect: "ja, aber ich hab gegen dich gewonnen". Only
// when the sub-table is level too do the overall set and point differences
// speak, and players still equal after all four share a rank rather than being
// separated by their names.
func Table(participants []uuid.UUID, matches []domain.Match) []TableRow {
	index := make(map[uuid.UUID]int, len(participants))
	rows := make([]TableRow, 0, len(participants))
	for _, id := range participants {
		if _, seen := index[id]; seen {
			continue
		}
		index[id] = len(rows)
		rows = append(rows, TableRow{PlayerID: id})
	}

	counted := make([]domain.Match, 0, len(matches))
	for _, m := range matches {
		home, okHome := index[m.HomeID]
		away, okAway := index[m.AwayID]
		if m.Status != domain.MatchConfirmed || !okHome || !okAway {
			continue
		}
		counted = append(counted, m)

		homeSets, awaySets, homePoints, awayPoints := tally(m)

		rows[home].Played++
		rows[away].Played++
		rows[home].SetsWon += homeSets
		rows[home].SetsLost += awaySets
		rows[away].SetsWon += awaySets
		rows[away].SetsLost += homeSets
		rows[home].PointsFor += homePoints
		rows[home].PointsAgainst += awayPoints
		rows[away].PointsFor += awayPoints
		rows[away].PointsAgainst += homePoints

		if homeSets > awaySets {
			rows[home].Won++
			rows[away].Lost++
		} else {
			rows[away].Won++
			rows[home].Lost++
		}
	}

	slices.SortStableFunc(rows, func(a, b TableRow) int {
		return compareRows(a, b, counted)
	})
	assignRanks(rows, counted)
	return rows
}

// tally reduces a match to the four numbers the table is built from.
func tally(m domain.Match) (homeSets, awaySets, homePoints, awayPoints int) {
	for _, s := range m.Sets {
		homePoints += s.HomePoints
		awayPoints += s.AwayPoints
		switch {
		case s.HomePoints > s.AwayPoints:
			homeSets++
		case s.AwayPoints > s.HomePoints:
			awaySets++
		}
	}
	return homeSets, awaySets, homePoints, awayPoints
}

// compareRows orders two rows, most successful first. It returns 0 for
// players nothing separates, which is what produces a shared rank.
func compareRows(a, b TableRow, matches []domain.Match) int {
	// Everybody who has played comes first, whatever the numbers say. A
	// player with no results has a set difference of zero, which would put
	// them above somebody who has lost three times — and a table that ranks
	// the person who has not turned up above the person who did is not a
	// table. The overall ranking makes the same exception for the same
	// reason (internal/standings).
	if c := cmp.Compare(hasPlayed(b), hasPlayed(a)); c != 0 {
		return c
	}
	if c := cmp.Compare(b.Won, a.Won); c != 0 {
		return c
	}
	// The sub-table: only what these two did to each other. For exactly two
	// players that is the direct encounter; the same expression generalises
	// to a larger group, where head-to-head alone would not.
	subA, subB := headToHead(a.PlayerID, b.PlayerID, matches)
	if c := cmp.Compare(subB.wins, subA.wins); c != 0 {
		return c
	}
	if c := cmp.Compare(subB.setDiff, subA.setDiff); c != 0 {
		return c
	}
	if c := cmp.Compare(b.SetDiff(), a.SetDiff()); c != 0 {
		return c
	}
	return cmp.Compare(b.PointDiff(), a.PointDiff())
}

// hasPlayed is 1 for a player with results and 0 for one without, so the
// distinction can be a comparison rather than a branch.
func hasPlayed(r TableRow) int {
	if r.Played > 0 {
		return 1
	}
	return 0
}

// side is one player's account of the matches between two people.
type side struct{ wins, setDiff int }

// headToHead sums what x and y did to each other, ignoring everybody else.
func headToHead(x, y uuid.UUID, matches []domain.Match) (forX, forY side) {
	for _, m := range matches {
		if !((m.HomeID == x && m.AwayID == y) || (m.HomeID == y && m.AwayID == x)) {
			continue
		}
		homeSets, awaySets, _, _ := tally(m)

		xSets, ySets := homeSets, awaySets
		if m.HomeID == y {
			xSets, ySets = awaySets, homeSets
		}

		forX.setDiff += xSets - ySets
		forY.setDiff += ySets - xSets
		if xSets > ySets {
			forX.wins++
		} else {
			forY.wins++
		}
	}
	return forX, forY
}

// assignRanks numbers the sorted rows, sharing a position where nothing
// separates two players and skipping the positions a shared rank used up.
//
// A player who has not played yet gets rank 0 rather than last place, the same
// way the overall ranking treats an untested rating: a placement nobody has
// earned reads as a bug to the person holding it.
func assignRanks(rows []TableRow, matches []domain.Match) {
	rank := 0
	for i := range rows {
		if rows[i].Played == 0 {
			rows[i].Rank = 0
			continue
		}
		switch {
		case i > 0 && rows[i-1].Played > 0 && compareRows(rows[i-1], rows[i], matches) == 0:
			rows[i].Rank = rows[i-1].Rank
			rows[i].Shared = true
			rows[i-1].Shared = true
		default:
			rank = i + 1
			rows[i].Rank = rank
		}
	}
}
