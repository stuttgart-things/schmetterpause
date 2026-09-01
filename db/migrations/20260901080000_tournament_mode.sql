-- +goose Up
-- The mode belongs to the tournament, not to each pairing.
--
-- Entry was hardcoded to best of three up to eleven, which made the schedule
-- silent about how the evening was actually played — and a table that cannot
-- say over how many sets it was decided is a table nobody can read back later.
-- Asking once at the draw is the alternative to asking it twenty-eight times,
-- which is the reason the entry form had no control in the first place.
--
-- Additive, per invariant 8. Existing tournaments take the defaults, which are
-- exactly what they were played under.
alter table tournaments
    add column best_of       int not null default 3,
    add column points_to_win int not null default 11;

-- The same values matches allows, and for the same reason: a tournament whose
-- mode no match may carry is a draw that cannot be entered.
alter table tournaments
    add constraint tournaments_best_of_valid check (best_of in (1, 3, 5, 7)),
    add constraint tournaments_points_to_win_valid check (points_to_win in (11, 21));

-- +goose Down
alter table tournaments drop constraint if exists tournaments_points_to_win_valid;
alter table tournaments drop constraint if exists tournaments_best_of_valid;
alter table tournaments drop column if exists points_to_win;
alter table tournaments drop column if exists best_of;
