-- +goose Up
-- A player checked in at one half of the table (docs/adr/0024, issue #321).
--
-- Every row is created here, never asserted by the Zählwerk: a result that
-- reports without an operator names two of these, and this application checks
-- them against its own records. So the table is the proof, and a row says who,
-- where, how and when.
--
-- Nothing points at it yet. Which result a check-in carried, and so whether it
-- is used up, is the next step (#320) and gets its own additive migration.
create table checkins (
    id         uuid        primary key default gen_random_uuid(),
    -- Cascade: a player who can be deleted at all has never played (see the
    -- foreign keys on matches), so their check-ins proved nothing that
    -- outlives them.
    player_id  uuid        not null references players (id) on delete cascade,
    -- The half as the Zählwerk names it (its docs/adr/0005). Left is side a.
    half       text        not null,
    -- How the player proved they were there: a fob at the reader, or their
    -- own signed-in phone.
    method     text        not null,
    created_at timestamptz not null default now(),

    constraint checkins_half_valid check (half in ('left', 'right')),
    constraint checkins_method_valid check (method in ('phone', 'rfid'))
);

create index checkins_player_id_idx on checkins (player_id);

-- +goose Down
drop table checkins;
