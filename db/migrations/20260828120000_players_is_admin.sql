-- +goose Up
-- Who may act for somebody else, per docs/adr/0008.
--
-- A flag on the player rather than a property of a browser. That is the whole
-- point: the kiosk's cookie is a constant every browser that ever opened the
-- token URL holds, so it can name a machine but never a person — and an
-- administrative capability over other people's identities has to be
-- revocable and traceable, which means it has to belong to somebody.
--
-- One level, not roles. If a second is ever needed, that is a new decision.
--
-- Additive with a false default, per invariant 8: every existing player stays
-- exactly what they were.
alter table players add column is_admin boolean not null default false;

-- +goose Down
alter table players drop column is_admin;
