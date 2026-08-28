-- +goose Up
-- The shared secrets a player proves themselves with: the recovery code from
-- docs/adr/0006 and the PIN from docs/adr/0007. Both live in one table
-- because both are hashed the same way and read on the same path.
--
-- Additive, per invariant 8: nothing existing changes, and a player without a
-- row here is exactly as recognisable as before through their cookie.

create table player_credentials (
    player_id  uuid        not null references players (id) on delete cascade,
    kind       text        not null,
    -- The encoded Argon2id digest, parameters and salt included. Never the
    -- secret itself, in any form.
    hash       text        not null,
    updated_at timestamptz not null default now(),

    -- One row per player and kind, so replacing a secret is an upsert rather
    -- than an insert plus a delete somebody could forget. That is what makes
    -- "a new code invalidates the old one immediately" (docs/adr/0006) a
    -- property of the schema instead of a property of the caller.
    primary key (player_id, kind),

    constraint player_credentials_kind_valid check (kind in ('recovery', 'pin')),
    constraint player_credentials_hash_not_blank check (length(btrim(hash)) > 0)
);

-- No index beyond the primary key on purpose. Every lookup starts from a
-- player: with a salt per row there is no way to find a row from the secret
-- alone, which is why sign-in asks for the name first (docs/adr/0007).

-- +goose Down
drop table player_credentials;
