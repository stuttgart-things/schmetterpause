-- The Definition of Done from docs/mvp-plan.md, read out of the database:
-- across five consecutive working days, at least ten matches from at least
-- five different players, logged without anybody being reminded.
--
-- Counted by confirmed_at rather than played_at. An unconfirmed match has not
-- reached the ranking, and a result the opponent never agreed to is not
-- evidence that the thing works.
--
-- Only results players entered themselves count towards the verdict. A kiosk
-- entry is a scorekeeper typing in an evening, which is not the question —
-- eight players round robin is twenty-eight matches, nearly three times the
-- bar, and counted in by accident the measurement passes and proves nothing.
-- They are reported beside it rather than hidden, because "how much did the
-- kiosk do" is worth knowing too.
--
-- Tournament matches drop out too, whoever typed them. That used to happen by
-- itself, because entry was only possible at the kiosk and every row came out
-- marked 'kiosk'; since ADR-0010 a player may enter their own tournament
-- result from their phone, and those rows say 'player'. So the exclusion says
-- what it means: a match belonging to a tournament is not evidence of
-- voluntary logging, because a schedule is exactly the reminder this
-- measurement excludes. They are reported beside it, like kiosk rows.
--
-- The column that makes this possible arrived with issue #71. Rows written
-- before it say 'kiosk' only where the migration guessed from
-- confirmed_at = played_at, and that guess is written down as one in
-- db/migrations/20260828100000_matches_entered_via.sql.
--
-- What this still cannot see, and it inflates the number:
--
--   * Being reminded leaves no trace in the database at all.
--   * An evening that was not a tournament but was still organised — somebody
--     going round saying "trag das mal ein" — leaves no trace either. SINCE
--     stays useful for exactly that case.
--
-- Weekend results are visible in the per-day table but do not count towards a
-- window: the measurement asks about working days.

\set ON_ERROR_STOP on

\echo ''
\echo '  Confirmed results per day'
\echo ''

select to_char(day, 'YYYY-MM-DD Dy') as "day",
       matches,
       reporters,
       kiosk as "of which kiosk",
       tournament as "of which tournament",
       case when extract(isodow from day) > 5 then 'weekend' else '' end as "note"
from (
	select (m.confirmed_at at time zone :'zone')::date                     as day,
	       count(*) filter (where m.entered_via = 'player'
	                          and m.tournament_id is null)                 as matches,
	       count(distinct m.reported_by) filter (where m.entered_via = 'player'
	                                               and m.tournament_id is null)
	                                                                       as reporters,
	       count(*) filter (where m.entered_via = 'kiosk')                 as kiosk,
	       count(*) filter (where m.tournament_id is not null)             as tournament
	from matches m
	where m.status = 'confirmed'
	  and (m.confirmed_at at time zone :'zone')::date >= :'since'::date
	group by 1
) per_day
order by day;

\echo ''
\echo '  Best window of five consecutive working days'
\echo ''

with bounds as (
	-- The span runs to today, not to the last result. Otherwise a week that
	-- started well and went quiet on Thursday reports "fewer than five
	-- working days" when five have plainly passed.
	select min((confirmed_at at time zone :'zone')::date)                        as first_day,
	       greatest(max((confirmed_at at time zone :'zone')::date),
	                (now() at time zone :'zone')::date)                          as last_day
	from matches
	where status = 'confirmed'
	  and entered_via = 'player'
	  and tournament_id is null
	  and (confirmed_at at time zone :'zone')::date >= :'since'::date
),
working_days as (
	-- Days with nothing in them count as days: five consecutive working days
	-- is a span, not a list of the days somebody happened to play on.
	select d::date as day
	from bounds, generate_series(bounds.first_day, bounds.last_day, interval '1 day') as d
	where extract(isodow from d) < 6
),
numbered as (
	select day, row_number() over (order by day) as n from working_days
),
windows as (
	select a.day as from_day, b.day as to_day
	from numbered a
	join numbered b on b.n = a.n + 4
),
scored as (
	select w.from_day,
	       w.to_day,
	       count(m.id)                   as matches,
	       count(distinct m.reported_by) as reporters
	from windows w
	left join matches m
	       on m.status = 'confirmed'
	      -- The whole point of the column: a scorekeeper's evening is not
	      -- somebody logging their own match (issue #71).
	      and m.entered_via = 'player'
	      -- And a schedule is a reminder, whoever held the phone (ADR-0010).
	      and m.tournament_id is null
	      and (m.confirmed_at at time zone :'zone')::date between w.from_day and w.to_day
	      and extract(isodow from (m.confirmed_at at time zone :'zone')) < 6
	group by 1, 2
),
best as (
	select *
	from scored
	order by (matches >= 10 and reporters >= 5) desc, matches desc, reporters desc
	limit 1
)
select coalesce(to_char(best.from_day, 'YYYY-MM-DD') || ' .. ' || to_char(best.to_day, 'YYYY-MM-DD'),
                '-') as "window",
       coalesce(best.matches, 0)   as "matches (need 10)",
       coalesce(best.reporters, 0) as "players (need 5)",
       case
	       when best.from_day is null then 'not yet - fewer than five working days of confirmed results'
	       when best.matches >= 10 and best.reporters >= 5 then 'PASSED'
	       else 'not yet'
       end as "verdict"
-- The left join is what guarantees a row: an empty result set reads as a
-- broken query, and this is read by somebody who wants an answer, not a
-- diagnosis.
from (select 1) one
left join best on true;

\echo ''
\echo '  Kiosk entries are excluded from the verdict and shown per day.'
\echo '  Being reminded is still in no column. See issue #43.'
\echo ''
