-- +goose Up
-- The Zählwerk enters results too (docs/adr/0015). It is a third way a row
-- reaches this table, and the column that already distinguishes a phone from
-- the kiosk has to be able to say so.
--
-- Additive, per invariant 8: no data is touched and nothing is narrowed. The
-- constraint is dropped and re-added because Postgres cannot widen a CHECK in
-- place; goose runs this in one transaction, so there is no moment in which
-- the table is unconstrained.
--
-- No backfill, and none is possible. Every existing row predates this path, so
-- 'player' and 'kiosk' are already right for all of them. Unlike the migration
-- that added the column, this one has nothing to guess about.
alter table matches drop constraint matches_entered_via_valid;

alter table matches add constraint matches_entered_via_valid
    check (entered_via in ('player', 'kiosk', 'scoreboard'));

-- +goose Down
-- Narrowing, so it fails while any scoreboard row exists — which is the honest
-- behaviour. Rolling this back means deciding what those results were, and
-- that is not a decision a migration can take.
alter table matches drop constraint matches_entered_via_valid;

alter table matches add constraint matches_entered_via_valid
    check (entered_via in ('player', 'kiosk'));
