-- +goose Up
-- Empty is the original mascot: existing choices and legacy clients keep
-- their current appearance. Keys are validated by internal/avatar (ADR-0031).
alter table player_avatars add column character text not null default '';

-- +goose Down
alter table player_avatars drop column character;
