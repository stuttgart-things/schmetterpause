-- +goose Up
-- Whether a tournament moves ratings at all (docs/adr/0012).
--
-- There are two kinds of tournament in an office and the form could not tell
-- them apart: the championship, which counts and is played because it counts,
-- and the Friday afternoon one, which nobody wants to risk their rating on.
-- Without the question, whoever takes their position seriously simply does not
-- turn up to the second kind.
--
-- Additive, per invariant 8, and defaulted to true: every tournament that
-- already exists was played for the rating, because there was no other way to
-- play one.
alter table tournaments
    add column rated boolean not null default true;

-- +goose Down
alter table tournaments drop column if exists rated;
