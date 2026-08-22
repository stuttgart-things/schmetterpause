package standings_test

import (
	"slices"
	"strconv"
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/standings"
)

func record(name string, ttr, played, won int) domain.PlayerRecord {
	return domain.PlayerRecord{
		Player: domain.Player{ID: uuid.New(), DisplayName: name, TTR: ttr},
		Played: played, Won: won, Lost: played - won,
	}
}

// ranksOf reduces a table to "name:rank" pairs, which is what the tests are
// actually about.
func ranksOf(rows []standings.Row) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Record.Player.DisplayName+":"+strconv.Itoa(r.Rank))
	}
	return out
}

func TestBuild(t *testing.T) {
	tests := []struct {
		name    string
		records []domain.PlayerRecord
		want    []string
	}{
		{
			name: "nobody",
			want: nil,
		},
		{
			name:    "one player who has played",
			records: []domain.PlayerRecord{record("Anna", 1000, 1, 1)},
			want:    []string{"Anna:1"},
		},
		{
			// Rank 0: a starting rating nobody has tested is not a placement.
			name:    "a player who has not played holds no rank",
			records: []domain.PlayerRecord{record("Anna", 1000, 0, 0)},
			want:    []string{"Anna:0"},
		},
		{
			name: "distinct ratings",
			records: []domain.PlayerRecord{
				record("Anna", 1100, 3, 2), record("Bodo", 1000, 3, 1), record("Cleo", 900, 2, 0),
			},
			want: []string{"Anna:1", "Bodo:2", "Cleo:3"},
		},
		{
			// Two on top are both first, and the next player is third. Giving
			// them rank 2 would quietly promote somebody nobody beat.
			name: "a tie at the top skips a rank",
			records: []domain.PlayerRecord{
				record("Anna", 1100, 3, 2), record("Bodo", 1100, 3, 2), record("Cleo", 900, 2, 0),
			},
			want: []string{"Anna:1", "Bodo:1", "Cleo:3"},
		},
		{
			name: "a tie in the middle",
			records: []domain.PlayerRecord{
				record("Anna", 1100, 1, 1), record("Bodo", 1000, 1, 0),
				record("Cleo", 1000, 1, 0), record("Dora", 900, 1, 0),
			},
			want: []string{"Anna:1", "Bodo:2", "Cleo:2", "Dora:4"},
		},
		{
			name: "everybody level",
			records: []domain.PlayerRecord{
				record("Anna", 1000, 2, 1), record("Bodo", 1000, 2, 1), record("Cleo", 1000, 2, 1),
			},
			want: []string{"Anna:1", "Bodo:1", "Cleo:1"},
		},
		{
			// The state a fresh evening starts in: four names, no matches,
			// and four number ones would read as a defect rather than a tie.
			name: "a fresh table ranks nobody",
			records: []domain.PlayerRecord{
				record("Anna", 1000, 0, 0), record("Bodo", 1000, 0, 0),
				record("Cleo", 1000, 0, 0), record("Dora", 1000, 0, 0),
			},
			want: []string{"Anna:0", "Bodo:0", "Cleo:0", "Dora:0"},
		},
		{
			// Bodo lost and is below the starting rating; Cleo has not
			// played. Rating alone would put Cleo above him, which is not
			// something a table may claim.
			name: "whoever has played stands above whoever has not",
			records: []domain.PlayerRecord{
				record("Cleo", 1000, 0, 0), record("Bodo", 992, 1, 0), record("Anna", 1008, 1, 1),
			},
			want: []string{"Anna:1", "Bodo:2", "Cleo:0"},
		},
		{
			// The repository sorts already, but a ranking that depends on
			// that is one refactor away from being wrong.
			name: "input in any order",
			records: []domain.PlayerRecord{
				record("Cleo", 900, 2, 0), record("Anna", 1100, 3, 2), record("Bodo", 1000, 3, 1),
			},
			want: []string{"Anna:1", "Bodo:2", "Cleo:3"},
		},
		{
			// Same rating: alphabetical, and case must not decide it.
			name: "level players sort by name",
			records: []domain.PlayerRecord{
				record("bodo", 1000, 1, 0), record("Anna", 1000, 1, 0),
			},
			want: []string{"Anna:1", "bodo:1"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ranksOf(standings.Build(tc.records))
			if !slices.Equal(got, tc.want) {
				t.Errorf("Build() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSharedMarksEveryPlayerOnATiedRank(t *testing.T) {
	rows := standings.Build([]domain.PlayerRecord{
		record("Anna", 1100, 1, 1), record("Bodo", 1000, 1, 0),
		record("Cleo", 1000, 1, 0), record("Dora", 900, 1, 0),
	})

	want := []bool{false, true, true, false}
	for i, row := range rows {
		if row.Shared != want[i] {
			t.Errorf("%s: Shared = %v, want %v", row.Record.Player.DisplayName, row.Shared, want[i])
		}
	}
}

func TestBuildDoesNotTouchTheInput(t *testing.T) {
	records := []domain.PlayerRecord{
		record("Cleo", 900, 0, 0), record("Anna", 1100, 0, 0),
	}

	standings.Build(records)

	if records[0].Player.DisplayName != "Cleo" {
		t.Error("Build() reordered the slice it was given")
	}
}

func TestBuildCarriesTheRecord(t *testing.T) {
	rows := standings.Build([]domain.PlayerRecord{record("Anna", 1100, 5, 3)})

	got := rows[0].Record
	if got.Played != 5 || got.Won != 3 || got.Lost != 2 {
		t.Errorf("record = %d played, %d won, %d lost; want 5/3/2", got.Played, got.Won, got.Lost)
	}
}
