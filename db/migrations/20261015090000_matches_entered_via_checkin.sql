-- +goose Up
-- A result from a match both players checked in to reports without an
-- operator (docs/adr/0024, issue #320). The two check-ins are the proof, and
-- the match points at them, so years later it still says who stood where and
-- when.
--
-- Additive, per invariant 8. The entered_via constraint is widened the way
-- 20260916060000 widened it. reported_by loses its NOT NULL, but only one rule
-- is relaxed and no row changes: a CHECK keeps the column required for every
-- other path, so a forgotten reporter is still refused unless the row is a
-- checked-in one.
alter table matches drop constraint matches_entered_via_valid;

alter table matches add constraint matches_entered_via_valid
    check (entered_via in ('player', 'kiosk', 'scoreboard', 'checkin'));

alter table matches alter column reported_by drop not null;

alter table matches add constraint matches_reported_by_unless_checkin
    check (reported_by is not null or entered_via = 'checkin');

-- No ON DELETE: a check-in that carried a result is the proof behind it and
-- must not disappear underneath it. The cascade from players never reaches
-- one, because a player with matches cannot be deleted at all.
alter table matches add column home_checkin_id uuid references checkins (id);
alter table matches add column away_checkin_id uuid references checkins (id);

-- A checked-in match names both, and nothing else names either. The reverse
-- holds too: entered_via says 'checkin' exactly when the proof is there, so a
-- report cannot claim the path without it.
alter table matches add constraint matches_checkins_iff_checkin
    check ((entered_via = 'checkin') = (home_checkin_id is not null)
       and (entered_via = 'checkin') = (away_checkin_id is not null));

alter table matches add constraint matches_checkins_differ
    check (home_checkin_id <> away_checkin_id);

-- One check-in carries one result. The application locks both rows and asks
-- before it writes (CheckinRepository.Lock); these indexes are the backstop
-- for the same check-in in the same role. The same check-in as home in one
-- match and away in another is the lock's to prevent: no index spans two
-- columns that way.
create unique index matches_home_checkin_id_key on matches (home_checkin_id)
    where home_checkin_id is not null;
create unique index matches_away_checkin_id_key on matches (away_checkin_id)
    where away_checkin_id is not null;

-- +goose Down
-- Narrowing, so it fails while any checked-in row exists: those have no
-- reporter, and which one they should have is not a migration's decision.
drop index matches_away_checkin_id_key;
drop index matches_home_checkin_id_key;
alter table matches drop constraint matches_checkins_differ;
alter table matches drop constraint matches_checkins_iff_checkin;
alter table matches drop column away_checkin_id;
alter table matches drop column home_checkin_id;
alter table matches drop constraint matches_reported_by_unless_checkin;
alter table matches alter column reported_by set not null;
alter table matches drop constraint matches_entered_via_valid;
alter table matches add constraint matches_entered_via_valid
    check (entered_via in ('player', 'kiosk', 'scoreboard'));
