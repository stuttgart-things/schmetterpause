-- +goose Up
-- Somebody who does not play, per docs/adr/0022.
--
-- A second flag beside is_admin rather than a role column: whether somebody
-- plays and whether they may act for others are two questions, and the
-- account this exists for answers yes to one and no to the other.
--
-- Additive with a false default, per invariant 8: every existing player keeps
-- playing.
alter table players add column is_observer boolean not null default false;

-- +goose Down
alter table players drop column is_observer;
