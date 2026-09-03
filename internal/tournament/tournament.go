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
func Draw(players []uuid.UUID, legs int) []Round {
	if len(players) < 2 {
		return nil
	}
	if legs < 1 {
		legs = 1
	}

	// The circle works on an even field. The placeholder is uuid.Nil, which
	// no player can be, so "did somebody draw the bye" is a comparison rather
	// than a parallel bookkeeping slice.
	circle := slices.Clone(players)
	if len(circle)%2 == 1 {
		circle = append(circle, uuid.Nil)
	}

	n := len(circle)
	rounds := make([]Round, 0, (n-1)*legs)

	// The second leg is the same circle again with the sides swapped, its
	// rounds numbered on from the first. Numbering them on is what makes a
	// return match distinguishable from its first leg: the pair is the same,
	// the slot is not (docs/adr/0011).
	for r := range (n - 1) * legs {
		round := Round{No: r + 1, Pairings: make([]Pairing, 0, n/2)}
		// Which round of the circle this is, ignoring the leg. The cosmetic
		// alternation below has to key on this rather than on r: keyed on r
		// it would cancel the return leg's swap in every second round and
		// hand back the first leg's orientation unchanged.
		inLeg := r % (n - 1)
		returnLeg := r >= n-1

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
			if inLeg%2 == 1 {
				home, away = away, home
			}
			// And swapped once more for the whole return leg, so whoever was
			// listed on the left in the first meeting is on the right in the
			// second. That is the only thing a return leg changes.
			if returnLeg {
				home, away = away, home
			}
			round.Pairings = append(round.Pairings, Pairing{Home: home, Away: away})
		}

		rounds = append(rounds, round)
		rotate(circle)

		// The circle returns to its starting position after n-1 rotations, so
		// the second leg repeats the first without any bookkeeping.
	}

	return rounds
}

// GroupRounds is how many rounds the group phase of this field has. A final,
// where there is one, is the round after them.
func GroupRounds(n, legs int) int {
	if n < 2 {
		return 0
	}
	if legs < 1 {
		legs = 1
	}
	if n%2 == 1 {
		n++
	}
	return (n - 1) * legs
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
func Matches(n, legs int, withFinal bool) int {
	if n < 2 {
		return 0
	}
	if legs < 1 {
		legs = 1
	}
	total := n * (n - 1) / 2 * legs
	if withFinal {
		total++
	}
	return total
}

// Final is the pairing that decides the tournament: the two best of the
// group, once the table can name them without ambiguity.
//
// rows must be a table as Table returns it, already sorted. The answer is
// false when the cut is genuinely tied — two players sharing first, or sharing
// second — because a decider between two arbitrarily chosen of three equals is
// a draw with an audience rather than a decision (docs/adr/0011). It is also
// false before anybody has played, where "the best two" names nobody.
//
// The pairing is derived, not stored. That is the whole reason a final needs
// no pairings table: the table is a function of the results and the best two
// are a function of the table, so this is as computable as any round of the
// circle method.
func Final(rows []TableRow) (Pairing, bool) {
	if len(rows) < 2 {
		return Pairing{}, false
	}
	first, second := rows[0], rows[1]
	if first.Shared || second.Shared || first.Played == 0 || second.Played == 0 {
		return Pairing{}, false
	}
	return Pairing{Home: first.PlayerID, Away: second.PlayerID}, true
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

// PointsPerWin is what a win is worth where a tournament counts in points
// rather than in wins.
//
// There is no value for a draw, and there is no draw: a match runs until
// somebody has the sets, so the 1 of a 3/1/0 system is never awarded here.
// Which means the two ways of counting cannot disagree — 3·W is a monotone
// transform of W, so it produces the same order and the same shared ranks.
// The choice is what the table says, not who is above whom.
//
// Written down because it is the thing somebody will eventually want to
// change: a bonus point for losing in a deciding set would make the two
// differ, and it would be a new rule rather than a new format for this one.
const PointsPerWin = 3

// SetDiff is sets won less sets lost, the first tie-break the overall table
// applies. Exposed because the table shows it.
func (r TableRow) SetDiff() int { return r.SetsWon - r.SetsLost }

// TablePoints is the row in points. Not to be confused with PointsFor, which
// is the balls: one is what the table awards, the other what was played.
func (r TableRow) TablePoints() int { return PointsPerWin * r.Won }

// PointDiff is points for less points against, the last tie-break before two
// players share a rank.
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
// the caller has to remember is a filter somebody eventually forgets — which
// is why groupRounds is a parameter rather than the caller dropping the final
// itself. Pass 0 where there is no final to leave out.
//
// # How ties are broken
//
// Match wins first. Where that leaves players level, the results *among those
// players* decide — the sub-table — which is the official rule and also the
// one people at the table expect: "ja, aber ich hab gegen dich gewonnen". Only
// when the sub-table is level too do the overall set and point differences
// speak, and players still equal after all four share a rank rather than being
// separated by their names.
func Table(participants []uuid.UUID, matches []domain.Match, groupRounds int) []TableRow {
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
		// The final is not part of the group table. Counting it there would
		// let its result move the standings that decided who plays it, which
		// is a circle (docs/adr/0011). A match with no round is from before
		// slots existed and is therefore a group match.
		if groupRounds > 0 && m.TournamentRound != nil && *m.TournamentRound > groupRounds {
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

	keys := subTableKeys(rows, counted)
	slices.SortStableFunc(rows, func(a, b TableRow) int {
		return compareRows(a, b, keys)
	})
	assignRanks(rows, keys)
	return rows
}

// subKey is a player's standing inside the group of players they are level
// with on match wins.
type subKey struct{ wins, setDiff int }

// subTableKeys computes the sub-table for every group of players tied on
// wins: the results among exactly those players, and nobody else.
//
// This is the whole reason the tie-break is not a pairwise head-to-head
// comparison. Three players can beat each other in a cycle — a beats b beats
// c beats a — and a pairwise comparison is then intransitive, which makes the
// sort order depend on the order the rows arrived in and hands out three
// different ranks where the correct answer is one shared rank. Reducing each
// group to a scalar first is what removes that: comparing numbers is
// transitive whatever the results did.
func subTableKeys(rows []TableRow, matches []domain.Match) map[uuid.UUID]subKey {
	group := make(map[int][]uuid.UUID, len(rows))
	member := make(map[uuid.UUID]int, len(rows))
	for _, r := range rows {
		if r.Played == 0 {
			continue
		}
		group[r.Won] = append(group[r.Won], r.PlayerID)
		member[r.PlayerID] = r.Won
	}

	keys := make(map[uuid.UUID]subKey, len(rows))
	for _, m := range matches {
		home, okHome := member[m.HomeID]
		away, okAway := member[m.AwayID]
		// Only matches inside one group, and only where that group has
		// somebody to be separated from.
		if !okHome || !okAway || home != away || len(group[home]) < 2 {
			continue
		}

		homeSets, awaySets, _, _ := tally(m)
		kHome, kAway := keys[m.HomeID], keys[m.AwayID]
		kHome.setDiff += homeSets - awaySets
		kAway.setDiff += awaySets - homeSets
		if homeSets > awaySets {
			kHome.wins++
		} else {
			kAway.wins++
		}
		keys[m.HomeID], keys[m.AwayID] = kHome, kAway
	}
	return keys
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
func compareRows(a, b TableRow, keys map[uuid.UUID]subKey) int {
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
	// The sub-table: what the players on this many wins did to each other.
	// For exactly two that is the direct encounter — "ja, aber ich hab gegen
	// dich gewonnen" — and for more it is the group's own little table.
	subA, subB := keys[a.PlayerID], keys[b.PlayerID]
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

// assignRanks numbers the sorted rows, sharing a position where nothing
// separates two players and skipping the positions a shared rank used up.
//
// A player who has not played yet gets rank 0 rather than last place, the same
// way the overall ranking treats an untested rating: a placement nobody has
// earned reads as a bug to the person holding it.
func assignRanks(rows []TableRow, keys map[uuid.UUID]subKey) {
	rank := 0
	for i := range rows {
		if rows[i].Played == 0 {
			rows[i].Rank = 0
			continue
		}
		switch {
		case i > 0 && rows[i-1].Played > 0 && compareRows(rows[i-1], rows[i], keys) == 0:
			rows[i].Rank = rows[i-1].Rank
			rows[i].Shared = true
			rows[i-1].Shared = true
		default:
			rank = i + 1
			rows[i].Rank = rank
		}
	}
}
