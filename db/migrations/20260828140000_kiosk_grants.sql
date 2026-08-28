-- +goose Up
-- One row per machine that has been unlocked as a kiosk, per issue #77 and
-- open point 5 in docs/adr/0008.
--
-- What it replaces: the kiosk cookie used to be a derived constant,
-- base64(HMAC(session key, "kiosk:" + token)). Every browser that had ever
-- opened the token URL held the identical value, so the laptop at the table
-- and the phone of somebody who read the token over a shoulder were the same
-- thing to the server — and taking one back meant changing the token and
-- restarting, which logged out the table along with everybody else.
--
-- A row per machine answers the two questions a constant cannot: which
-- machines are kiosks right now, and how do I take one of them back.

create table kiosk_grants (
    id           uuid        primary key default gen_random_uuid(),
    -- SHA-256 of the secret in the cookie. A plain hash and not Argon2id on
    -- purpose: the secret is 32 bytes this server generated, so there is no
    -- guess to slow down, and a memory-hard hash on every kiosk request would
    -- make the page at the table unusable. The reasoning in docs/adr/0007
    -- runs the other way because a PIN is six digits somebody chose.
    secret_hash  bytea       not null,
    created_at   timestamptz not null default now(),
    -- When this machine last showed the cookie. It is what makes the list
    -- readable: a grant nobody has used since Tuesday is a laptop somebody
    -- took home.
    last_seen_at timestamptz not null default now(),
    expires_at   timestamptz not null,
    -- What the browser said it was. Not a fact and not identity — a label, so
    -- the list reads as machines rather than as a column of UUIDs.
    user_agent   text        not null default '',
    -- Set when somebody took this one back. Kept rather than deleted, so the
    -- list can say what happened instead of quietly having one row fewer.
    revoked_at   timestamptz,

    constraint kiosk_grants_expires_after_creation check (expires_at > created_at)
);

create unique index kiosk_grants_secret_key on kiosk_grants (secret_hash);
-- The lookup the admin page makes: what is unlocked right now.
create index kiosk_grants_active_idx on kiosk_grants (expires_at desc) where revoked_at is null;

-- +goose Down
drop table kiosk_grants;
