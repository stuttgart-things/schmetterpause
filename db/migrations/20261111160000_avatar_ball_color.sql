-- +goose Up
-- Empty is white, preserving the zero-value and all existing avatar rows.
alter table player_avatars add column ball_color text not null default '';

-- +goose Down
alter table player_avatars drop column ball_color;
