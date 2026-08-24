-- The Definition of Done from docs/mvp-plan.md, read out of the database:
-- across five consecutive working days, at least ten matches from at least
-- five different players, logged without anybody being reminded.
--
-- Counted by confirmed_at rather than played_at. An unconfirmed match has not
-- reached the ranking, and a result the opponent never agreed to is not
-- evidence that the thing works.
--
-- Two halves of the question this cannot see, and both inflate the number:
--
--   * Being reminded leaves no trace in the database at all.
--   * Neither does a tournament. A kiosk entry credits reported_by to the
--     home player, so an evening one scorekeeper typed in looks exactly like
--     ten people logging their own matches. Start the window after such a day
--     with SINCE=<date>.
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
       case when extract(isodow from day) > 5 then 'weekend' else '' end as "note"
from (
	select (m.confirmed_at at time zone :'zone')::date as day,
	       count(*)                                    as matches,
	       count(distinct m.reported_by)               as reporters
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
\echo '  A tournament is not this measurement. See issue #43.'
\echo ''
