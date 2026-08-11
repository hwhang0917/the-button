-- Gameplay analytics starter queries. Run against the NDJSON event log:
--   duckdb -c ".read analytics/queries.sql"        (all at once)
-- or paste individual queries into `duckdb` with events/ as cwd context.
-- Adjust the path glob to wherever EVENTS_DIR points (e.g. /data/events/*.ndjson).

-- Event volume by type
SELECT type, count(*) AS n
FROM read_json('events/*.ndjson')
GROUP BY type ORDER BY n DESC;

-- Observed success rate by stars and risk level (validates the chance table in the wild)
SELECT stars_before, risk, count(*) AS rolls,
       round(avg(CASE WHEN success THEN 1 ELSE 0 END) * 100, 1) AS observed_pct,
       any_value(chance) AS designed_pct
FROM read_json('events/*.ndjson')
WHERE type = 'roll'
GROUP BY stars_before, risk ORDER BY stars_before, risk;

-- Risk-level usage distribution
SELECT risk, count(*) AS rolls, round(count(*) * 100.0 / sum(count(*)) OVER (), 1) AS pct
FROM read_json('events/*.ndjson')
WHERE type = 'roll'
GROUP BY risk ORDER BY risk;

-- When do players cash out? (stars-at-sell histogram)
SELECT stars, count(*) AS sells, round(avg(gain), 1) AS avg_gain
FROM read_json('events/*.ndjson')
WHERE type = 'sell'
GROUP BY stars ORDER BY stars;

-- Lottery: observed payback vs the designed 81%
SELECT count(*) AS tickets,
       sum(prize) AS paid_out,
       round(sum(prize) * 100.0 / (count(*) * 15), 1) AS observed_payback_pct
FROM read_json('events/*.ndjson')
WHERE type = 'lottery';

-- Daily actives and engagement
SELECT strftime(ts::TIMESTAMP, '%Y-%m-%d') AS day,
       count(DISTINCT pid) AS players,
       count(*) FILTER (type = 'roll') AS rolls,
       round(count(*) FILTER (type = 'roll') * 1.0 / count(DISTINCT pid), 1) AS rolls_per_player
FROM read_json('events/*.ndjson')
GROUP BY day ORDER BY day;

-- Skill economy: what do players buy?
SELECT skill, count(*) AS buys, sum(price) AS coins_spent
FROM read_json('events/*.ndjson')
WHERE type = 'buy'
GROUP BY skill ORDER BY coins_spent DESC;

-- Shield saves and prestige laps over time
SELECT strftime(ts::TIMESTAMP, '%Y-%m-%d') AS day,
       count(*) FILTER (type = 'roll' AND shield_used) AS shield_saves,
       count(*) FILTER (type = 'prestige') AS prestiges
FROM read_json('events/*.ndjson')
GROUP BY day ORDER BY day;

-- Appetite beyond the hourly quota (rejected clicks per day)
SELECT strftime(ts::TIMESTAMP, '%Y-%m-%d') AS day, count(*) AS rejected_clicks
FROM read_json('events/*.ndjson')
WHERE type = 'quota_empty'
GROUP BY day ORDER BY day;
