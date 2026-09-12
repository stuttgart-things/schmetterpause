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
	"github.com/stuttgart-things/schmetterpause/internal/match"
	"github.com/stuttgart-things/schmetterpause/internal/repository"
	"github.com/stuttgart-things/schmetterpause/internal/scoring"
	"github.com/stuttgart-things/schmetterpause/internal/tournament"
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
	// Tournaments is how many brackets the fixture left open, and
	// TournamentMatches how many of their pairings are already played.
	Tournaments      int
	TournamentPlayed int
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

// The evening: an open tournament with its first round played and the rest
// still to come.
//
// It is here for the same reason two of the twelve results are left
// unsettled. Without a tournament a preview shows an empty notice on the start
// page — the card at the top of it — an empty tournament list and no draw at
// all, so a change to any of the three is invisible in exactly the place it
// is reviewed. And a tournament with nothing played shows a table of zeroes,
// which says as little about the table as an empty ranking does.
const tournamentName = "Mittwochsturnier"

// tournamentField is who plays it, by index into field. Four of the six, so
// the list also shows what a player outside the draw sees — and so there is
// somebody left to write the results down.
var tournamentField = []int{2, 0, 4, 1}

// tournamentScorer is who types at the table, by index into field. Somebody
// outside the draw, which is what the kiosk insists on: you may not enter a
// match you are playing in yourself.
const tournamentScorer = 5

// tournamentSides are the ends of the table for this one (docs/adr/0013).
// Named rather than "A" and "B", because the pair that is worth showing is
// the one somebody actually types.
const (
	tournamentSideA = "Fenster"
	tournamentSideB = "Tür"
)

// tournamentPlayed are the results of the first round, in the order the draw
// produces its pairings. The rounds they are booked into are not written down
// here: they come from tournament.Draw, the same function the page draws the
// schedule with, so a result cannot end up in a slot it was not played in.
var tournamentPlayed = [][][2]int{
	{{11, 8}, {9, 11}, {11, 7}},
	{{11, 6}, {11, 9}},
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

	played, err := runTournament(ctx, store, ids, now)
	if err != nil {
		return Summary{}, err
	}
	summary.Tournaments, summary.TournamentPlayed = 1, played

	return summary, nil
}

// runTournament opens the bracket and plays its first round, and reports how
// many of its pairings that was.
//
// The results go through scoring.Record — the kiosk's path — rather than
// through a confirmation. That is what happened at the table: somebody stood
// there and wrote it down, so there is nobody left to ask, and the row says
// so in entered_via. A fixture that recorded them as self-reported would put
// a tournament evening back inside the measurement issue #71 took it out of.
func runTournament(
	ctx context.Context, store repository.Store, ids []uuid.UUID, now time.Time,
) (int, error) {
	field := make([]uuid.UUID, 0, len(tournamentField))
	for _, i := range tournamentField {
		field = append(field, ids[i])
	}

	mode := match.Mode{BestOf: 3, PointsToWin: match.PointsToEleven}

	created, err := store.Tournaments().Create(ctx, domain.Tournament{
		Name:        tournamentName,
		Format:      domain.TournamentRoundRobin,
		Status:      domain.TournamentOpen,
		CreatedBy:   field[0],
		BestOf:      mode.BestOf,
		PointsToWin: mode.PointsToWin,
		Rated:       true,
		SideA:       tournamentSideA,
		SideB:       tournamentSideB,
		Players:     field,
	})
	if err != nil {
		return 0, fmt.Errorf("creating the tournament: %w", err)
	}

	// The draw is a function of the stored order, so the schedule is asked
	// for rather than assumed. The first round is what gets played.
	rounds := tournament.Draw(created.Players, created.Format.Legs())
	if len(rounds) == 0 {
		return 0, fmt.Errorf("the tournament field of %d produced no draw", len(field))
	}
	first := rounds[0]

	// Yesterday, so the evening reads as one that is still running rather
	// than as one somebody forgot to close months ago.
	playedAt := now.AddDate(0, 0, -1)

	played := 0
	for i, sets := range tournamentPlayed {
		if i >= len(first.Pairings) {
			return 0, fmt.Errorf("result %d has no pairing in round 1", i+1)
		}
		pairing := first.Pairings[i]

		result := match.Result{Mode: mode, Sets: make([]match.Set, 0, len(sets))}
		for _, s := range sets {
			result.Sets = append(result.Sets, match.Set{Home: s[0], Away: s[1]})
		}

		round := first.No
		if _, err := scoring.Record(ctx, store, pairing.Home, pairing.Away, result,
			domain.EnteredViaKiosk, &created.ID, &round, ids[tournamentScorer], playedAt,
		); err != nil {
			return 0, fmt.Errorf("recording tournament match %d: %w", i+1, err)
		}
		played++
	}

	return played, nil
}
