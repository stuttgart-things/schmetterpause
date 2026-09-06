// Package seed fills an empty database with a small, believable field.
//
// It exists for the preview environments in issue #82. A fresh Schmetterpause
// is a join form and an empty ranking, so a preview of a change to the
// standings, the match list or a profile shows nothing at all — the reviewer
// cannot see whether the change looks right, which is the entire reason a
// preview is worth building. The same fixture serves screenshots and a manual
// look at a branch, which is why #82 counted it as paid for twice.
//
// Two properties are deliberate.
//
// **It writes through the application's own rules.** Matches are created
// pending and settled by scoring.Confirm, exactly as a confirmation in the
// browser does — so the ratings, the history and the standings agree with the
// match list because they were computed from it, not because somebody typed
// consistent-looking numbers into an INSERT. A fixture that can disagree with
// the code it demonstrates is worse than no fixture.
//
// **It is deterministic.** No randomness and no wall clock in the data: every
// preview looks the same, two screenshots of the same commit are identical,
// and a reviewer comparing two branches is looking at the change rather than
// at noise.
package seed

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/repository"
	"github.com/stuttgart-things/schmetterpause/internal/scoring"
)

// ErrNotEmpty is the refusal to seed a database that already holds players.
//
// The guard is the whole safety story of this command: a preview namespace
// starts empty and a real instance does not, so "empty" is what separates the
// two without asking anybody to pass a flag correctly. Running seed against
// the office database is then a message rather than fourteen strangers in the
// ranking.
var ErrNotEmpty = errors.New("the database already has players")

// Summary is what was written, for the log line and for the tests.
type Summary struct {
	Players   int
	Confirmed int
	Pending   int
	Disputed  int
}

// player is one entry of the field.
type player struct {
	name string
	// ttr is where they start. Spread on purpose: an evenly rated field makes
	// every expected value 0.5, and a preview of the rating column then shows
	// nothing about how the rating behaves.
	ttr int
}

// The field. German names because every one of them appears on screen, and a
// preview that says "Player 1" tests the layout against a string nobody will
// ever have.
var field = []player{
	{"Anna", 1080},
	{"Bodo", 1000},
	{"Clara", 1240},
	{"David", 940},
	{"Eva", 1150},
	{"Frank", 1020},
}

// result is one match of the fixture, by index into field.
type result struct {
	home, away int
	// sets are home:away, in the order played.
	sets [][2]int
	// daysAgo is when it was played, counting back from the reference time.
	daysAgo int
	// state is how far it got. Most are confirmed; the two that are not exist
	// so the pending list and the waiting list (issue #159) have something in
	// them — those screens are invisible in an empty preview, and they are
	// exactly the screens somebody reviewing a change to them needs to see.
	state domain.MatchStatus
}

var results = []result{
	{home: 0, away: 1, sets: [][2]int{{11, 7}, {11, 9}}, daysAgo: 18, state: domain.MatchConfirmed},
	{home: 2, away: 3, sets: [][2]int{{11, 4}, {11, 6}}, daysAgo: 17, state: domain.MatchConfirmed},
	{home: 4, away: 5, sets: [][2]int{{9, 11}, {11, 8}, {11, 13}}, daysAgo: 15, state: domain.MatchConfirmed},
	{home: 1, away: 3, sets: [][2]int{{11, 9}, {8, 11}, {12, 10}}, daysAgo: 12, state: domain.MatchConfirmed},
	{home: 0, away: 2, sets: [][2]int{{6, 11}, {9, 11}}, daysAgo: 11, state: domain.MatchConfirmed},
	{home: 5, away: 3, sets: [][2]int{{11, 5}, {11, 7}}, daysAgo: 9, state: domain.MatchConfirmed},
	{home: 4, away: 0, sets: [][2]int{{11, 13}, {11, 6}, {11, 9}}, daysAgo: 8, state: domain.MatchConfirmed},
	{home: 2, away: 4, sets: [][2]int{{11, 8}, {6, 11}, {11, 9}}, daysAgo: 5, state: domain.MatchConfirmed},
	{home: 1, away: 5, sets: [][2]int{{11, 6}, {4, 11}, {11, 8}}, daysAgo: 4, state: domain.MatchConfirmed},
	{home: 3, away: 0, sets: [][2]int{{11, 9}, {11, 7}}, daysAgo: 2, state: domain.MatchConfirmed},

	// Reported and not yet answered. Two days old, so it also carries the
	// "seit 2 Tagen" mark that issue #159 added — visible on both sides.
	{home: 4, away: 1, sets: [][2]int{{11, 5}, {11, 9}}, daysAgo: 2, state: domain.MatchPending},
	// Contested, which is the only state where both participants are offered
	// a correction.
	{home: 5, away: 2, sets: [][2]int{{11, 8}, {11, 6}}, daysAgo: 1, state: domain.MatchDisputed},
}

// Run writes the fixture. now is the reference the match dates count back
// from; pass the real clock in the command and a fixed value in a test.
//
// It refuses a database that already has players with ErrNotEmpty.
func Run(ctx context.Context, store repository.Store, now time.Time) (Summary, error) {
	count, err := store.Players().Count(ctx)
	if err != nil {
		return Summary{}, fmt.Errorf("counting the players: %w", err)
	}
	if count > 0 {
		return Summary{}, fmt.Errorf("%w: %d of them", ErrNotEmpty, count)
	}

	ids := make([]uuid.UUID, 0, len(field))
	for _, p := range field {
		created, err := store.Players().Create(ctx, p.name, p.ttr)
		if err != nil {
			return Summary{}, fmt.Errorf("creating %s: %w", p.name, err)
		}
		ids = append(ids, created.ID)
	}

	summary := Summary{Players: len(ids)}

	for i, r := range results {
		homeID, awayID := ids[r.home], ids[r.away]

		sets := make([]domain.MatchSet, 0, len(r.sets))
		for n, s := range r.sets {
			sets = append(sets, domain.MatchSet{SetNo: n + 1, HomePoints: s[0], AwayPoints: s[1]})
		}

		playedAt := now.AddDate(0, 0, -r.daysAgo)

		// Created pending and reported by the home side, which is what the
		// form does: it asks one player what they played. The confirmation
		// below then comes from the other one, as it must.
		created, err := store.Matches().Create(ctx, domain.Match{
			HomeID:      homeID,
			AwayID:      awayID,
			BestOf:      3,
			PointsToWin: 11,
			Status:      domain.MatchPending,
			ReportedBy:  homeID,
			PlayedAt:    playedAt,
			EnteredVia:  domain.EnteredViaPlayer,
			Sets:        sets,
		})
		if err != nil {
			return Summary{}, fmt.Errorf("creating match %d: %w", i+1, err)
		}

		switch r.state {
		case domain.MatchConfirmed:
			// Through scoring.Confirm rather than SetStatus: this is what
			// writes the rating change and the history entry, so the ranking
			// a preview shows is one the application computed.
			if _, err := scoring.Confirm(ctx, store, created.ID, awayID, playedAt); err != nil {
				return Summary{}, fmt.Errorf("confirming match %d: %w", i+1, err)
			}
			summary.Confirmed++
		case domain.MatchDisputed:
			if err := scoring.Dispute(ctx, store, created.ID, awayID); err != nil {
				return Summary{}, fmt.Errorf("disputing match %d: %w", i+1, err)
			}
			summary.Disputed++
		default:
			summary.Pending++
		}
	}

	return summary, nil
}
