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

		home, err := tx.Players().ByID(ctx, m.HomeID)
		if err != nil {
			return fmt.Errorf("load the home player: %w", err)
		}
		away, err := tx.Players().ByID(ctx, m.AwayID)
		if err != nil {
			return fmt.Errorf("load the away player: %w", err)
		}

		outcome, err := match.Validate(toResult(m))
		if err != nil {
			// The row is in the database, so this is not a player mistake —
			// it means something wrote a result that cannot have happened.
			return fmt.Errorf("stored match %s is not a possible result: %w", m.ID, err)
		}

		homeChange, awayChange := ttr.RateMatch(home.TTR, away.TTR, outcome.HomeWon)

		if err := tx.Players().UpdateTTR(ctx, home.ID, homeChange.After); err != nil {
			return err
		}
		if err := tx.Players().UpdateTTR(ctx, away.ID, awayChange.After); err != nil {
			return err
		}

		// Written before the status changes, so a failure here leaves the
		// match pending rather than confirmed-but-unexplained.
		err = tx.TTRHistory().Append(ctx, []domain.TTRChange{
			{PlayerID: home.ID, MatchID: m.ID, TTRBefore: homeChange.Before, TTRAfter: homeChange.After},
			{PlayerID: away.ID, MatchID: m.ID, TTRBefore: awayChange.Before, TTRAfter: awayChange.After},
		})
		if err != nil {
			return err
		}

		confirmedAt := at
		if err := tx.Matches().SetStatus(ctx, m.ID, domain.MatchConfirmed, &confirmedAt); err != nil {
			return err
		}

		m.Status = domain.MatchConfirmed
		m.ConfirmedAt = &confirmedAt
		settlement = Settlement{
			Match: m, Home: home, Away: away,
			HomeChange: homeChange, AwayChange: awayChange,
			HomeWon:  outcome.HomeWon,
			HomeSets: outcome.HomeSets, AwaySets: outcome.AwaySets,
		}
		return nil
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
