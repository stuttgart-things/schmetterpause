-- +goose Up
-- Which animal a player's figure is (feat/poc-fynn). Empty is the mascot,
-- sthings, so every row stored before there was a choice stays what it was.
-- No CHECK list, for the reason the table itself gives: which keys exist is
-- internal/avatar's knowledge, and an unknown one falls back to the mascot.
alter table player_avatars add column animal text not null default '';

-- +goose Down
alter table player_avatars drop column animal;
