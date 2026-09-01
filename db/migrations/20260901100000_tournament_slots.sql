-- +goose Up
-- Which slot of the draw a result fills (docs/adr/0011).
--
-- Until now a result was found by its pair, which is exact for a single round
-- robin — every pair occurs once — and wrong for anything else. A return leg
-- would find the first leg's result under its own key and never be playable;
-- a final between two who already met collides the same way.
--
-- This is not a stored copy of a derived value, which is what ADR-0009 refuses
-- when it argues against a pairings table. The slots are still computed from
-- tournament_players.position; this says which of them a match belongs to.
--
-- Existing rows stay null, and that is exact rather than a gap: they are all
-- single round robins, where (round, pair) and (pair) are the same key.
alter table matches
    add column tournament_round int;

alter table matches
    add constraint matches_tournament_round_positive
        check (tournament_round is null or tournament_round > 0),
    -- A round without a tournament is a slot in a draw that does not exist.
    add constraint matches_tournament_round_needs_tournament
        check (tournament_round is null or tournament_id is not null);

-- Whether the two best of the group play a decider afterwards.
--
-- A flag rather than a fourth format name: four names for four combinations of
-- group shape and endgame grows quadratically at the next variant.
alter table tournaments
    add column with_final boolean not null default false;

-- The second group shape. Twice the matches, which is why the form has to show
-- the number it actually produces.
alter table tournaments drop constraint tournaments_format_valid;
alter table tournaments add constraint tournaments_format_valid
    check (format in ('round_robin', 'double_round_robin'));

-- +goose Down
alter table tournaments drop constraint if exists tournaments_format_valid;
alter table tournaments add constraint tournaments_format_valid
    check (format in ('round_robin'));
alter table tournaments drop column if exists with_final;
alter table matches drop constraint if exists matches_tournament_round_needs_tournament;
alter table matches drop constraint if exists matches_tournament_round_positive;
alter table matches drop column if exists tournament_round;
