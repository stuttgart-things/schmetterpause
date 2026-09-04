-- +goose Up
-- Who is typing at this machine, per issue #90.
--
-- The kiosk settles a result the moment it is entered, so the opponent never
-- gets to agree with it. That is right for somebody standing at the table
-- writing down other people's games, and it rests entirely on a trust
-- assumption the code calls "the room": everybody can see who is at the
-- laptop. #91 added a guard, but the kiosk holds no identity of its own, so
-- all that guard can see is a player signed in on the same browser.
--
-- A grant that names its operator is what gives the kiosk an identity without
-- giving it a login. Two things follow from it:
--
--   * matches.reported_by stops lying. A kiosk match is written with the home
--     player as reporter today, which makes it indistinguishable in the data
--     from a result that player entered themselves. It becomes the operator.
--   * the self-entry guard becomes a server-side check rather than a look at
--     the browser's own cookie: the operator may not be one of the two
--     players, and a private window cannot get around that.
--
-- Nullable, because a grant exists from the moment the token is accepted and
-- the operator is picked immediately afterwards. The handlers require it
-- before anything can be entered; the column stays nullable so that unlocking
-- and naming stay two steps, and so this migration is additive over grants
-- that already exist (invariant 8).
--
-- on delete set null rather than cascade: a player who is removed must not
-- take the record of which machine was unlocked with them.
alter table kiosk_grants
    add column operator_id uuid references players (id) on delete set null;

-- +goose Down
alter table kiosk_grants drop column operator_id;
