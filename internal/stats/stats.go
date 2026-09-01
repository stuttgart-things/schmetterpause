// Package stats answers questions about results that have already been
// agreed: who beat whom, and how much table tennis the office has actually
// played.
//
// No database, no HTTP, plain values only — the same separation ttr, match,
// standings and tournament keep (CLAUDE.md, "Konventionen").
//
// # Counts, not rates
//
// Everything here is a count or a total, and that is a decision rather than a
// starting point. With fewer than twenty matches on record a percentage is
// noise wearing a percent sign: "100% against Bodo" out of two matches is an
// anecdote formatted as a finding, and it gets believed because of the
// formatting. Counts do not have that problem — three wins to one is three
// wins to one whether the office has played twenty matches or two thousand.
//
// The rate-based figures — win rate, set rate, deciding-set record — are
// catalogued in issue #121 and wait for the volume that makes them true.
package stats

import (
	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// Cell is one player's record against one other player.
type Cell struct {
	// Won and Lost are confirmed matches only. A result nobody has agreed to
	// is not a result, the same rule the ranking and the tournament table
	// apply.
	Won, Lost int
	// Self marks the diagonal, where a player meets themselves. It is a cell
	// that exists for the layout and holds nothing.
	Self bool
}

// Played reports whether these two have met at all.
func (c Cell) Played() bool { return c.Won+c.Lost > 0 }

// Row is one player's line of the matrix.
type Row struct {
	PlayerID uuid.UUID
	Cells    []Cell
	// Won and Lost are the row's totals, so the matrix can carry the record
	// it already implies rather than making the reader add it up.
	Won, Lost int
}

// Matrix is everybody against everybody: row beat column this often.
//
// The order of players is taken as given and used for both axes, so a caller
// that hands over the ranking gets a matrix in ranking order. Duplicate ids
// are dropped rather than producing two rows for one person.
//
// Reading it: the cell in Anna's row and Bodo's column is Anna's record
// against Bodo. Bodo's row against Anna's column is the mirror image, which is
// redundant on purpose — somebody looking up "how do I stand against her"
// should find it on their own line, not have to transpose.
func Matrix(players []uuid.UUID, matches []domain.Match) []Row {
	index := make(map[uuid.UUID]int, len(players))
	rows := make([]Row, 0, len(players))
	for _, id := range players {
		if _, seen := index[id]; seen {
			continue
		}
		index[id] = len(rows)
		rows = append(rows, Row{PlayerID: id})
	}

	for i := range rows {
		rows[i].Cells = make([]Cell, len(rows))
		rows[i].Cells[i].Self = true
	}

	for _, m := range matches {
		home, okHome := index[m.HomeID]
		away, okAway := index[m.AwayID]
		if m.Status != domain.MatchConfirmed || !okHome || !okAway {
			continue
		}

		winner, loser := home, away
		if !homeWon(m) {
			winner, loser = away, home
		}

		rows[winner].Cells[loser].Won++
		rows[loser].Cells[winner].Lost++
		rows[winner].Won++
		rows[loser].Lost++
	}
	return rows
}

// Totals is what the office has played, as plain counts.
type Totals struct {
	// Matches, Sets and Points count confirmed matches only.
	Matches, Sets, Points int
	// Deuce is how many sets went past the ordinary finish — both players on
	// at least points_to_win minus one, so the set needed a two-point gap to
	// end. It is the closest thing to "how evenly matched is this group"
	// that a count can express, and it speaks to the starting-rating
	// question in issue #17.
	Deuce int
	// LongestSet is the highest combined score of any single set;
	// LongestSetHome and LongestSetAway state it the way it was played. All
	// three are zero before the first confirmed match.
	LongestSet     int
	LongestSetHome int
	LongestSetAway int
	// Whitewashes are matches won without dropping a set. A count, and the
	// counterpart to Deuce: one says how close it gets, the other how
	// one-sided.
	Whitewashes int
}

// Compute reduces confirmed matches to the office totals.
//
// Unconfirmed matches are skipped rather than counted provisionally: a result
// waiting on its opponent may still change, and a total that moves backwards
// when somebody disputes reads as a bug.
func Compute(matches []domain.Match) Totals {
	var t Totals

	for _, m := range matches {
		if m.Status != domain.MatchConfirmed {
			continue
		}
		t.Matches++

		// points_to_win is per match rather than global: 21 is an option
		// beside 11, and a set to 21 reaching 20:20 is the same event as a
		// set to 11 reaching 10:10.
		threshold := m.PointsToWin - 1

		var homeSets, awaySets int
		for _, s := range m.Sets {
			t.Sets++
			t.Points += s.HomePoints + s.AwayPoints

			if s.HomePoints >= threshold && s.AwayPoints >= threshold {
				t.Deuce++
			}
			if total := s.HomePoints + s.AwayPoints; total > t.LongestSet {
				t.LongestSet = total
				t.LongestSetHome, t.LongestSetAway = s.HomePoints, s.AwayPoints
			}

			switch {
			case s.HomePoints > s.AwayPoints:
				homeSets++
			case s.AwayPoints > s.HomePoints:
				awaySets++
			}
		}

		if homeSets == 0 || awaySets == 0 {
			t.Whitewashes++
		}
	}
	return t
}

// homeWon reports which side took the match, by counting sets. The stored
// match carries no winner column — the sets are the record — so every reader
// works it out the same way.
func homeWon(m domain.Match) bool {
	var homeSets, awaySets int
	for _, s := range m.Sets {
		switch {
		case s.HomePoints > s.AwayPoints:
			homeSets++
		case s.AwayPoints > s.HomePoints:
			awaySets++
		}
	}
	return homeSets > awaySets
}
