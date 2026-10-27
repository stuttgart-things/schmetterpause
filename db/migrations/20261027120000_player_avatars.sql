-- +goose Up
-- The figure a player has assembled (docs/adr/0029). One row per player who
-- has chosen; no row is the default derived from the id, so nobody looks
-- different on the day this table arrives.
--
-- The columns are plain text without a CHECK list. Which keys exist is
-- internal/avatar's knowledge: a new palette colour would otherwise need a
-- migration, and taking one out again would be a destructive change. A key the
-- code no longer knows falls back to the default for that part (avatar.Fill).
--
-- No face column. The face belongs to the game, not to the figure.
create table player_avatars (
    player_id  uuid        primary key references players (id) on delete cascade,
    fur        text        not null,
    headdress  text        not null,
    handle     text        not null,
    blade      text        not null,
    bling      text        not null default '',
    updated_at timestamptz not null default now()
);

-- +goose Down
drop table player_avatars;
