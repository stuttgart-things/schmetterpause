-- Row counts that say whether two copies of the game database hold the same
-- data. scripts/db.sh takes them before and after every dump, writes them next
-- to the dump as FILE.counts, and compares them after every restore.
--
-- Compare sorted. The order of these rows follows the server's collation, and
-- the Alpine images sort bytewise where Flexible Server sorts by locale:
-- match_sets and matches swap places, the numbers do not.
--
-- Every table a migration creates belongs in this list; scripts/db_test.sh
-- fails the build when one is missing.
select 'players', count(*) from players
union all select 'identities', count(*) from identities
union all select 'player_credentials', count(*) from player_credentials
union all select 'matches', count(*) from matches
union all select 'match_sets', count(*) from match_sets
union all select 'ttr_history', count(*) from ttr_history
union all select 'tournaments', count(*) from tournaments
union all select 'tournament_players', count(*) from tournament_players
union all select 'kiosk_grants', count(*) from kiosk_grants
union all select 'goose max version', max(version_id) from goose_db_version where is_applied
order by 1;
select 'matches ' || status, count(*) from matches group by status order by 1;
