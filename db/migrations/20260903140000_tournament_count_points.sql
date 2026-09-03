-- +goose Up
-- Whether the table is counted in points rather than in wins.
--
-- A display choice and not a scoring one, which is worth stating in the place
-- somebody will read it while wondering why the order did not change: a table
-- tennis match cannot end level, so the 1 of a 3/1/0 system is never awarded,
-- and three points a win is a monotone transform of one win a win. Same order,
-- same shared ranks, different column.
--
-- Additive, per invariant 8, and defaulted to false: every tournament that
-- already exists has a table in wins, which is the one it was read in.
alter table tournaments
    add column count_points boolean not null default false;

-- +goose Down
alter table tournaments drop column if exists count_points;
