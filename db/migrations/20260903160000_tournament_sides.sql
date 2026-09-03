-- +goose Up
-- Names for the two ends of the table, per tournament (docs/adr/0013).
--
-- Per tournament and not per deployment, because the table does not always
-- stand in the same room: no fixed name can be right, and an environment
-- variable would be a fixed name. Whoever enters "Fenster" and "Tür" gets a
-- schedule that can be read without a sign on the wall.
--
-- Additive, per invariant 8, and defaulted: every tournament that already
-- exists gets A and B, which is the statement it made implicitly anyway — the
-- two ends are told apart, they just have no name.
alter table tournaments
    add column side_a text not null default 'A',
    add column side_b text not null default 'B';

-- Same shape as the name: something a person typed, bounded so a schedule
-- stays readable, and never empty — empty means "use the default", and the
-- default is written here rather than rendered as a blank.
alter table tournaments
    add constraint tournaments_side_a_length check (char_length(side_a) between 1 and 20),
    add constraint tournaments_side_b_length check (char_length(side_b) between 1 and 20);

-- +goose Down
alter table tournaments drop constraint if exists tournaments_side_b_length;
alter table tournaments drop constraint if exists tournaments_side_a_length;
alter table tournaments drop column if exists side_b;
alter table tournaments drop column if exists side_a;
