-- +goose Up
-- A quick tournament: a bracket around matches, not a scoring event.
--
-- Issue #41 sketched this schema; ADR-0009 decides the part it left open.
-- A tournament match settles through scoring.Confirm like any other, one at a
-- time, so nothing here touches ttr_history and there is no settlement state
-- to model. What a tournament owns is who is in it and which matches belong
-- to it.
--
-- Additive throughout, per invariant 8. A casual match keeps tournament_id
-- null and behaves exactly as it does today; nothing existing changes meaning.

create table tournaments (
    id         uuid        primary key default gen_random_uuid(),
    name       text        not null,
    -- The draw format. One value today, and a column rather than an assumption
    -- because Swiss is the named successor for a field past ten (#41) and a
    -- table that cannot say which format it holds cannot hold two.
    format     text        not null default 'round_robin',
    status     text        not null default 'open',
    created_by uuid        not null references players (id),
    created_at timestamptz not null default now(),
    closed_at  timestamptz,

    constraint tournaments_name_not_blank check (btrim(name) <> ''),
    constraint tournaments_format_valid check (format in ('round_robin')),
    constraint tournaments_status_valid check (status in ('open', 'closed')),
    -- closed_at is set exactly when the status is 'closed', the same shape
    -- matches uses for confirmed_at. A state that can disagree with its
    -- timestamp is a state somebody eventually reads from the wrong column.
    constraint tournaments_closed_at_matches_status check (
        (status = 'closed') = (closed_at is not null)
    )
);

create index tournaments_status_idx on tournaments (status, created_at desc);

-- Who is in, and in which order the draw took them.
--
-- position is what makes a draw reproducible: the circle method is
-- deterministic over the order it is given, so storing the order means the
-- pairings can be recomputed rather than stored round by round. That is the
-- reason there is no tournament_pairings table — the draw is a function of
-- this column, and a stored copy of a derived value is a copy that can drift.
create table tournament_players (
    tournament_id uuid not null references tournaments (id) on delete cascade,
    player_id     uuid not null references players (id) on delete cascade,
    position      int  not null,

    primary key (tournament_id, player_id),
    constraint tournament_players_position_not_negative check (position >= 0)
);

create unique index tournament_players_position_key
    on tournament_players (tournament_id, position);

-- The bracket around a match. Null for everything played outside a tournament,
-- which is every row that exists today.
--
-- on delete set null rather than cascade: deleting a tournament must not take
-- the results with it. The matches were played, they are rated, and the
-- ratings they produced are already in ttr_history — removing the bracket has
-- to leave the evening's table tennis alone.
alter table matches
    add column tournament_id uuid references tournaments (id) on delete set null;

create index matches_tournament_idx on matches (tournament_id)
    where tournament_id is not null;

-- +goose Down
drop index if exists matches_tournament_idx;
alter table matches drop column if exists tournament_id;
drop table if exists tournament_players;
drop table if exists tournaments;
