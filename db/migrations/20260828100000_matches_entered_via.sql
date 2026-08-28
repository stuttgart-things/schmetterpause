-- +goose Up
-- How a result got into the database. Issue #71: a match entered at the kiosk
-- and one entered from a phone are indistinguishable today, and the Definition
-- of Done asks whether people log their own results — a scorekeeper's evening
-- counted in by accident makes the measurement pass and prove nothing.
--
-- Additive, per invariant 8. Existing readers do not have to know the column
-- exists, and rows written without it take the default.
alter table matches add column entered_via text not null default 'player';

alter table matches add constraint matches_entered_via_valid
    check (entered_via in ('player', 'kiosk'));

-- +goose StatementBegin
-- A guess, and it is written down as one.
--
-- scoring.Record writes played_at and confirmed_at in the same transaction, so
-- a kiosk row has them equal to the microsecond. That is an accident of the
-- implementation rather than a contract — it breaks the moment anything sets
-- the two separately — which is exactly why the column is being added. It is
-- good enough for the rows already in the table and for nothing else.
--
-- The alternative was to leave history unmarked. This was chosen because the
-- tournament data is precisely the data somebody will later want to exclude,
-- and a row that says nothing gets counted.
update matches
   set entered_via = 'kiosk'
 where status = 'confirmed'
   and confirmed_at = played_at;
-- +goose StatementEnd

-- +goose Down
alter table matches drop column entered_via;
