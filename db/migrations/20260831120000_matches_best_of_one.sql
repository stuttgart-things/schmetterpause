-- +goose Up
-- Allow a match played over a single set. Issue #114: in a break most games
-- are one set, and today recording one means claiming a best-of-3 that never
-- happened — or not recording it, which is the outcome #7 measures.
--
-- Widening a check constraint is additive in the sense invariant 8 means: no
-- row that was valid before becomes invalid, and no reader has to change.
-- Postgres has no "alter constraint" for a check, so the old one is dropped
-- and the wider one added in its place.
alter table matches drop constraint matches_best_of_valid;

alter table matches add constraint matches_best_of_valid
    check (best_of in (1, 3, 5, 7));

-- +goose Down
-- Only reversible while no single-set match has been recorded. That is the
-- shape invariant 8 warns about, and the reason the down step exists for
-- development only (see internal/repository/postgres/migrate.go).
alter table matches drop constraint matches_best_of_valid;

alter table matches add constraint matches_best_of_valid
    check (best_of in (3, 5, 7));
