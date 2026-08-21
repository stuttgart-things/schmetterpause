// Package standings turns player records into a ranking.
//
// The counting is the database's job — this package does not know what a
// match is. What it owns is the part that is easy to get quietly wrong:
// where two players on the same rating stand relative to each other and to
// everyone below them.
//
// No database, no HTTP, plain values only.
package standings

import (
	"cmp"
	"slices"
	"strings"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// Row is one line of the ranking.
type Row struct {
	// Rank is the position, counting from 1. Players on the same rating
	// share a rank and the next distinct rating skips the ones they used
	// up — two players on rank 1 are followed by rank 3, not rank 2. That is
	// how a sports table reads, and the alternative quietly promotes the
	// third player.
	Rank int
	// Shared marks a rank held by more than one player, so the table can say
	// so rather than leaving two identical numbers to explain themselves.
	Shared bool
	Record domain.PlayerRecord
}

// Build ranks the records: best rating first, name as the tie-breaker.
//
// It sorts the input itself rather than trusting the order it arrives in.
// The repository already returns them sorted, but a ranking that silently
// depends on that is one refactor away from being wrong, and sorting a few
// dozen rows costs nothing.
func Build(records []domain.PlayerRecord) []Row {
	sorted := slices.Clone(records)
	slices.SortStableFunc(sorted, func(a, b domain.PlayerRecord) int {
		if c := cmp.Compare(b.Player.TTR, a.Player.TTR); c != 0 {
			return c
		}
		return cmp.Compare(
			strings.ToLower(strings.TrimSpace(a.Player.DisplayName)),
			strings.ToLower(strings.TrimSpace(b.Player.DisplayName)),
		)
	})

	rows := make([]Row, 0, len(sorted))
	for i, record := range sorted {
		rank := i + 1
		if i > 0 && sorted[i-1].Player.TTR == record.Player.TTR {
			rank = rows[i-1].Rank
		}
		rows = append(rows, Row{Rank: rank, Record: record})
	}

	markShared(rows)
	return rows
}

// markShared flags every row whose rank another row also holds.
func markShared(rows []Row) {
	for i := range rows {
		switch {
		case i > 0 && rows[i-1].Rank == rows[i].Rank:
			rows[i].Shared = true
			rows[i-1].Shared = true
		case i+1 < len(rows) && rows[i+1].Rank == rows[i].Rank:
			rows[i].Shared = true
		}
	}
}
