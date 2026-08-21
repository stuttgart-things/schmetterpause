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

	// ErrNotYours reports somebody confirming a match that is not theirs to
	// confirm: a bystander, or the player who reported it. A result confirmed
	// by whoever entered it is not confirmed at all.
	ErrNotYours = errors.New("this match is not yours to confirm")
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
// scored later either until somebody resolves it by hand — the MVP has no
// interface for that on purpose (issue #18).
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
