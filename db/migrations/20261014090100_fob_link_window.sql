-- +goose Up
-- Linking a fob to a player happens only inside a window the player opened
-- themselves (docs/adr/0025). Outside one, an unknown UID is answered and
-- forgotten.
--
-- At most one window exists, and the primary key says so: the only possible
-- key is true. Opening is an upsert that replaces the row only once it has
-- expired, so two players pressing "Fob koppeln" at the same moment cannot
-- both get one, on one replica or several, without a lock and without Redis
-- (invariant 3; trigger 2 of docs/adr/0002 is about booking slots, not this).
--
-- Once there is more than one table, the key becomes the table (#280).
create table fob_link_window (
    only_one   boolean     primary key default true,
    player_id  uuid        not null references players (id) on delete cascade,
    opened_at  timestamptz not null,
    expires_at timestamptz not null,

    constraint fob_link_window_single check (only_one),
    constraint fob_link_window_expires_after_opening check (expires_at > opened_at)
);

-- When a proof was last used. The profile shows it for each fob, so a player
-- can tell the one on their keys from the one they lost. Nullable, because no
-- existing row knows, and the other providers have no use for it yet.
alter table identities add column last_used_at timestamptz;

-- +goose Down
alter table identities drop column last_used_at;
drop table fob_link_window;
