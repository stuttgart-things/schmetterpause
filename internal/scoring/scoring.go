// Package scoring settles a confirmed match.
//
// It sits between the pure calculation in internal/ttr and the repositories:
// it decides nothing about ratings itself and stores nothing by hand, it
// coordinates the two. That is also why it is not part of internal/ttr, which
// must stay free of storage so the rating rules remain testable on plain
// values.
//
// Everything here runs inside one transaction. Rating, history and the match
// status have to move together or not at all — a rating raised without the
// history entry that explains it cannot be traced afterwards, and a match
// marked confirmed without the rating is silently lost.
package scoring

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/match"
	"github.com/stuttgart-things/schmetterpause/internal/repository"
	"github.com/stuttgart-things/schmetterpause/internal/ttr"
)

var (
	// ErrNotPending reports a match that has already been settled or
	// contested. Confirming twice would double the rating change, which the
	// unique index on the history would refuse anyway — this catches it
	// earlier and with a usable reason.
	ErrNotPending = errors.New("match is not waiting for confirmation")

	// ErrNotYours reports somebody ruling on a match that is not theirs to
	// rule on: a bystander, or — when confirming — the player who reported
	// it. A result confirmed by whoever entered it is not confirmed at all.
	ErrNotYours = errors.New("this match is not yours to rule on")

	// ErrSamePlayer reports a match somebody plays against themselves. The
	// schema refuses it too (matches_players_differ); catching it here makes
	// the answer a sentence rather than a constraint violation.
	ErrSamePlayer = errors.New("a player cannot play themselves")

	// ErrNotUndoable reports a match that cannot be taken back: not
	// confirmed, or already gone.
	ErrNotUndoable = errors.New("match cannot be taken back")

	// ErrTooLate reports a match confirmed longer ago than UndoWindow. The
	// undo exists for a typo somebody notices at once, not for editing an
	// evening afterwards.
	ErrTooLate = errors.New("match was confirmed too long ago to take back")

	// ErrNotLast reports a match that is no longer the newest one for both
	// of its players. Taking it back writes the ratings from before it
	// straight back, and that is only correct while nothing has happened
	// since — otherwise the later match would be silently undone as well.
	ErrNotLast = errors.New("a newer match has counted since")

	// ErrNotDisputed reports a match that is not contested. Only a contested
	// result can be corrected: a pending one is still waiting for a plain yes
	// or no, and a confirmed one has already moved two ratings.
	ErrNotDisputed = errors.New("match is not contested")
)

// Settlement is what a confirmation amounted to.
type Settlement struct {
	Match domain.Match
	// Home and Away are the players as they were before the match counted,
	// so a caller can show what changed.
	Home, Away domain.Player
	// HomeChange and AwayChange are the rating movements.
	HomeChange, AwayChange ttr.Change
	// HomeWon and the set tally come from re-validating the stored result.
	// They are carried rather than left to the caller because a rating
	// change cannot stand in for them: a strong favourite who wins can move
	// by zero points, and reading that as a loss is exactly the kind of
	// wrong that looks right.
	HomeWon            bool
	HomeSets, AwaySets int
	// Rated is false for a match in a tournament that does not count
	// (docs/adr/0012). The two changes are then both zero, and a caller that
	// prints them without asking would say "±0" where nothing was computed —
	// which reads as a rating that held rather than one that was never
	// touched.
	Rated bool
}

// Confirm settles a pending match and records the result.
//
// by must be the participant who did not report it. The match is re-validated
// from the stored sets rather than trusting a winner recorded earlier: the
// rating follows from the score, and deriving it twice from the same source
// is cheaper than keeping a second copy honest.
func Confirm(
	ctx context.Context,
	store repository.Store,
	matchID, by uuid.UUID,
	at time.Time,
) (Settlement, error) {
	var settlement Settlement

	err := store.InTx(ctx, func(tx repository.Store) error {
		m, err := load(ctx, tx, matchID, by)
		if err != nil {
			return err
		}

		settlement, err = settle(ctx, tx, m, at)
		return err
	})
	if err != nil {
		return Settlement{}, err
	}
	return settlement, nil
}

// Dispute marks a match as contested. Nothing is scored, and nothing is
// scored until somebody corrects it — which either participant can do, see
// Correct.
//
// Disputing and correcting are two steps rather than one on purpose: somebody
// who taps "that is wrong" and walks away has still stopped a wrong number
// from being scored, which is the whole point of the step.
func Dispute(ctx context.Context, store repository.Store, matchID, by uuid.UUID) error {
	return store.InTx(ctx, func(tx repository.Store) error {
		m, err := load(ctx, tx, matchID, by)
		if err != nil {
			return err
		}
		// confirmed_at stays nil: the schema ties it to the confirmed status
		// (matches_confirmed_at_matches_status).
		return tx.Matches().SetStatus(ctx, m.ID, domain.MatchDisputed, nil)
	})
}

// Correction is what a corrected result amounts to.
type Correction struct {
	Match domain.Match
	// Opponent is who has to confirm now: the participant who did not
	// correct it.
	Opponent domain.Player
	// HomeWon and the set tally come from validating the corrected result,
	// for the same reason Settlement carries them.
	HomeWon            bool
	HomeSets, AwaySets int
}

// Correct replaces the result of a contested match and hands it back for
// confirmation.
//
// by may be either participant. The player who disputed usually knows the
// real score, but so does the one who mistyped it, and insisting on one of
// them would only send people back to the table to swap phones. Whoever
// corrects becomes the reporter, which makes the other one the confirmer
// through the same rule Confirm already applies.
//
// result is in home/away orientation rather than the corrector's: the caller
// owns the viewpoint, the domain owns the sides. A rejection from
// match.Validate is returned as-is so the caller can say what was wrong with
// it — a corrected result is held to exactly the same rules as a fresh one.
func Correct(
	ctx context.Context,
	store repository.Store,
	matchID, by uuid.UUID,
	result match.Result,
) (Correction, error) {
	var correction Correction

	err := store.InTx(ctx, func(tx repository.Store) error {
		m, err := tx.Matches().ByID(ctx, matchID)
		if err != nil {
			return err
		}
		if m.Status != domain.MatchDisputed {
			return fmt.Errorf("match %s is %s: %w", m.ID, m.Status, ErrNotDisputed)
		}
		if by != m.HomeID && by != m.AwayID {
			return fmt.Errorf("player %s did not play match %s: %w", by, m.ID, ErrNotYours)
		}

		outcome, err := match.Validate(result)
		if err != nil {
			return err
		}

		sets := make([]domain.MatchSet, 0, len(result.Sets))
		for i, set := range result.Sets {
			sets = append(sets, domain.MatchSet{
				SetNo: i + 1, HomePoints: set.Home, AwayPoints: set.Away,
			})
		}

		err = tx.Matches().ReplaceResult(ctx, m.ID, domain.Match{
			BestOf:      result.Mode.BestOf,
			PointsToWin: result.Mode.PointsToWin,
			ReportedBy:  by,
			Sets:        sets,
		})
		if err != nil {
			return err
		}

		opponentID := m.AwayID
		if by == m.AwayID {
			opponentID = m.HomeID
		}
		opponent, err := tx.Players().ByID(ctx, opponentID)
		if err != nil {
			return fmt.Errorf("load the opponent: %w", err)
		}

		m.Status = domain.MatchPending
		m.ReportedBy = by
		m.BestOf, m.PointsToWin, m.Sets = result.Mode.BestOf, result.Mode.PointsToWin, sets
		correction = Correction{
			Match: m, Opponent: opponent,
			HomeWon:  outcome.HomeWon,
			HomeSets: outcome.HomeSets, AwaySets: outcome.AwaySets,
		}
		return nil
	})
	if err != nil {
		return Correction{}, err
	}
	return correction, nil
}

// settle rates a match and marks it confirmed. It is the part Confirm and
// Record have in common: how a result turns into two ratings does not depend
// on who decided the result counts.
//
// The caller owns the transaction, so a failure anywhere in here takes the
// whole thing with it.
func settle(ctx context.Context, tx repository.Store, m domain.Match, at time.Time) (Settlement, error) {
	home, err := tx.Players().ByID(ctx, m.HomeID)
	if err != nil {
		return Settlement{}, fmt.Errorf("load the home player: %w", err)
	}
	away, err := tx.Players().ByID(ctx, m.AwayID)
	if err != nil {
		return Settlement{}, fmt.Errorf("load the away player: %w", err)
	}

	outcome, err := match.Validate(toResult(m))
	if err != nil {
		// The row is in the database, so this is not a player mistake — it
		// means something wrote a result that cannot have happened.
		return Settlement{}, fmt.Errorf("stored match %s is not a possible result: %w", m.ID, err)
	}

	rated, err := ratedMatch(ctx, tx, m)
	if err != nil {
		return Settlement{}, err
	}

	// A tournament that does not count still produces matches; it just moves
	// nothing (docs/adr/0012). The match is confirmed either way — it was
	// played, and it belongs in the table, the list and the statistics.
	var homeChange, awayChange ttr.Change
	if rated {
		homeChange, awayChange = ttr.RateMatch(home.TTR, away.TTR, outcome.HomeWon)

		if err := tx.Players().UpdateTTR(ctx, home.ID, homeChange.After); err != nil {
			return Settlement{}, err
		}
		if err := tx.Players().UpdateTTR(ctx, away.ID, awayChange.After); err != nil {
			return Settlement{}, err
		}

		// Written before the status changes, so a failure here leaves the
		// match pending rather than confirmed-but-unexplained.
		err = tx.TTRHistory().Append(ctx, []domain.TTRChange{
			{PlayerID: home.ID, MatchID: m.ID, TTRBefore: homeChange.Before, TTRAfter: homeChange.After},
			{PlayerID: away.ID, MatchID: m.ID, TTRBefore: awayChange.Before, TTRAfter: awayChange.After},
		})
		if err != nil {
			return Settlement{}, err
		}
	} else {
		// Both sides stand where they stood. Zero values here would read as a
		// rating of zero to anybody who takes Before and After without
		// checking Rated first.
		homeChange = ttr.Change{Before: home.TTR, After: home.TTR}
		awayChange = ttr.Change{Before: away.TTR, After: away.TTR}
	}

	confirmedAt := at
	if err := tx.Matches().SetStatus(ctx, m.ID, domain.MatchConfirmed, &confirmedAt); err != nil {
		return Settlement{}, err
	}

	m.Status = domain.MatchConfirmed
	m.ConfirmedAt = &confirmedAt
	return Settlement{
		Match: m, Home: home, Away: away,
		HomeChange: homeChange, AwayChange: awayChange,
		HomeWon:  outcome.HomeWon,
		HomeSets: outcome.HomeSets, AwaySets: outcome.AwaySets,
		Rated: rated,
	}, nil
}

// ratedMatch answers whether this match moves ratings.
//
// A match outside a tournament always does — casual play is what the rating
// is of. Inside one, the tournament decides, once, for the whole draw
// (docs/adr/0012).
func ratedMatch(ctx context.Context, tx repository.Store, m domain.Match) (bool, error) {
	if m.TournamentID == nil {
		return true, nil
	}
	tour, err := tx.Tournaments().ByID(ctx, *m.TournamentID)
	if err != nil {
		return false, fmt.Errorf("load the tournament of match %s: %w", m.ID, err)
	}
	return tour.Rated, nil
}

// Record stores a result between two players and settles it at once.
//
// It is what a scorekeeper does: somebody watched the match, both players
// were standing at the table, and the sheet is the authority. There is no
// third party left to ask, so the confirmation step Record skips would only
// be a question nobody is in a position to answer differently.
//
// Everything else is the ordinary path — the same validation, the same
// rating, the same history — and it all happens in one transaction, so a
// failure leaves no half-recorded match behind.
//
// via is a parameter rather than a constant in here, even though the kiosk is
// the only caller today. The column exists so that the measurement can tell a
// tournament evening from a normal Tuesday (issue #71), and a second caller
// that silently inherited "kiosk" would put back exactly the confusion it was
// added to remove.
//
// tournamentID books the result to a tournament, or is nil for a match played
// outside one. It changes nothing about how the match is rated — a tournament
// match settles here like any other (docs/adr/0009) — and exists so a table
// can be built from the results afterwards.
func Record(
	ctx context.Context,
	store repository.Store,
	homeID, awayID uuid.UUID,
	result match.Result,
	via domain.EnteredVia,
	tournamentID *uuid.UUID,
	tournamentRound *int,
	at time.Time,
) (Settlement, error) {
	if homeID == awayID {
		return Settlement{}, ErrSamePlayer
	}

	var settlement Settlement

	err := store.InTx(ctx, func(tx repository.Store) error {
		if _, err := match.Validate(result); err != nil {
			return err
		}

		sets := make([]domain.MatchSet, 0, len(result.Sets))
		for i, set := range result.Sets {
			sets = append(sets, domain.MatchSet{
				SetNo: i + 1, HomePoints: set.Home, AwayPoints: set.Away,
			})
		}

		created, err := tx.Matches().Create(ctx, domain.Match{
			HomeID:      homeID,
			AwayID:      awayID,
			BestOf:      result.Mode.BestOf,
			PointsToWin: result.Mode.PointsToWin,
			// Recorded rather than reported: the match never waits on
			// anybody, so reported_by names who it is credited to and not
			// who has to agree.
			Status:          domain.MatchPending,
			ReportedBy:      homeID,
			PlayedAt:        at,
			EnteredVia:      via,
			TournamentID:    tournamentID,
			TournamentRound: tournamentRound,
			Sets:            sets,
		})
		if err != nil {
			return err
		}

		settlement, err = settle(ctx, tx, created, at)
		return err
	})
	if err != nil {
		return Settlement{}, err
	}
	return settlement, nil
}

// load fetches the match and checks that by is allowed to rule on it.
func load(ctx context.Context, tx repository.Store, matchID, by uuid.UUID) (domain.Match, error) {
	m, err := tx.Matches().ByID(ctx, matchID)
	if err != nil {
		return domain.Match{}, err
	}

	if m.Status != domain.MatchPending {
		return domain.Match{}, fmt.Errorf("match %s is %s: %w", m.ID, m.Status, ErrNotPending)
	}

	participant := by == m.HomeID || by == m.AwayID
	if !participant || by == m.ReportedBy {
		return domain.Match{}, fmt.Errorf("player %s cannot rule on match %s: %w", by, m.ID, ErrNotYours)
	}
	return m, nil
}

// toResult turns a stored match back into something match.Validate accepts.
func toResult(m domain.Match) match.Result {
	sets := make([]match.Set, 0, len(m.Sets))
	for _, s := range m.Sets {
		sets = append(sets, match.Set{Home: s.HomePoints, Away: s.AwayPoints})
	}
	return match.Result{
		Mode: match.Mode{BestOf: m.BestOf, PointsToWin: m.PointsToWin},
		Sets: sets,
	}
}

// UndoWindow is how long a settled result can be taken back.
//
// Ten minutes: long enough for "wrong player, wrong set" to be noticed by the
// person still standing at the table, short enough that nobody edits the
// evening afterwards. The other guard is the one that keeps the arithmetic
// honest, and it has no clock in it — see ErrNotLast.
const UndoWindow = 10 * time.Minute

// Undone is what taking a result back amounted to.
type Undone struct {
	// Home and Away are the players as they are again, with the ratings
	// from before the match restored.
	Home, Away domain.Player
	// HomeSets and AwaySets are what the deleted result said, so a caller
	// can name what disappeared.
	HomeSets, AwaySets int
}

// Undo removes a settled match and puts both ratings back where they were.
//
// It exists because a result entered at the kiosk counts immediately: there is
// no pending state to dispute and nothing left to correct, so a typo would
// otherwise stand for good.
//
// Two conditions, and the second is the one that matters. The window is a
// question of manners. The check that this is still the newest match for both
// players is a question of correctness: the ratings are restored by writing
// ttr_before back, which is right only while nothing has counted since. A
// match played in between would be undone along with it, silently.
func Undo(ctx context.Context, store repository.Store, matchID uuid.UUID, at time.Time) (Undone, error) {
	var undone Undone

	err := store.InTx(ctx, func(tx repository.Store) error {
		m, err := tx.Matches().ByID(ctx, matchID)
		if err != nil {
			return err
		}
		if m.Status != domain.MatchConfirmed || m.ConfirmedAt == nil {
			return fmt.Errorf("match %s is %s: %w", m.ID, m.Status, ErrNotUndoable)
		}
		if at.Sub(*m.ConfirmedAt) > UndoWindow {
			return fmt.Errorf("match %s was confirmed at %s: %w", m.ID, m.ConfirmedAt, ErrTooLate)
		}

		rated, err := ratedMatch(ctx, tx, m)
		if err != nil {
			return err
		}

		changes, err := tx.TTRHistory().ForMatch(ctx, m.ID)
		if err != nil {
			return err
		}
		if rated && len(changes) == 0 {
			// Confirmed without history is a match that was never settled,
			// which means something else wrote that status. In a tournament
			// that does not count it is the ordinary state (docs/adr/0012),
			// which is why the question is asked first.
			return fmt.Errorf("match %s has no rating history: %w", m.ID, ErrNotUndoable)
		}

		// An unrated match is in nobody's chain, so there is nothing to
		// restore and nothing a later match could have been built on: the
		// ErrNotLast guard below has no work to do and no right to refuse.
		for _, change := range changes {
			newest, err := tx.TTRHistory().ForPlayer(ctx, change.PlayerID, 1)
			if err != nil {
				return err
			}
			if len(newest) == 0 || newest[0].MatchID != m.ID {
				return fmt.Errorf("player %s has counted a newer match: %w", change.PlayerID, ErrNotLast)
			}
			if err := tx.Players().UpdateTTR(ctx, change.PlayerID, change.TTRBefore); err != nil {
				return err
			}
		}

		// The sets and the history go with it, cascaded by the schema.
		if err := tx.Matches().Delete(ctx, m.ID); err != nil {
			return err
		}

		outcome, err := match.Validate(toResult(m))
		if err != nil {
			return fmt.Errorf("stored match %s is not a possible result: %w", m.ID, err)
		}
		undone.HomeSets, undone.AwaySets = outcome.HomeSets, outcome.AwaySets

		if undone.Home, err = tx.Players().ByID(ctx, m.HomeID); err != nil {
			return err
		}
		if undone.Away, err = tx.Players().ByID(ctx, m.AwayID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return Undone{}, err
	}
	return undone, nil
}
