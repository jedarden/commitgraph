-- Invariant 2: No out-of-range days in repo_user_daily_tool
--
-- This invariant validates that no row in repo_user_daily_tool has a day
-- outside [2005-01-01, current_date + 1].
--
-- This is the 2170-incident guard: a single 2170-dated commit once zeroed
-- the board-wide AI-commit count (quarantine bf-jyctj/93dc8d1; aggregator
-- fix 946e815). The clamp must be applied before any day value reaches
-- Postgres.
--
-- Usage:
--   Run this assertion in CI against fixture databases
--   Run periodically against production as an audit
--
-- Expected: 0 rows (no violations)

-- Current date + 1 bound (run in production environment)
DO $$
DECLARE
    max_allowed_date DATE := CURRENT_DATE + INTERVAL '1 day';
    min_allowed_date DATE := '2005-01-01'::DATE;
    violation_count BIGINT;
BEGIN
    -- Count violations
    SELECT COUNT(*) INTO violation_count
    FROM repo_user_daily_tool
    WHERE day < min_allowed_date OR day > max_allowed_date;

    -- Log violations
    IF violation_count > 0 THEN
        RAISE NOTICE 'Invariant 2 violation: % rows with day outside [%, %]',
            violation_count, min_allowed_date, max_allowed_date;

        -- Show sample violations for debugging
        RAISE NOTICE 'Sample violations (up to 10):';
        FOR v IN
            SELECT repo_id, user_id, tool, day, commits
            FROM repo_user_daily_tool
            WHERE day < min_allowed_date OR day > max_allowed_date
            LIMIT 10
        LOOP
            RAISE NOTICE '  repo_id=%, user_id=%, tool=%, day=%, commits=%',
                v.repo_id, v.user_id, v.tool, v.day, v.commits;
        END LOOP;
    ELSE
        RAISE NOTICE 'Invariant 2: PASS (0 violations)';
    END IF;
END $$;

-- Direct query for CI (returns 0 rows on pass, violation rows on fail)
SELECT
    rut.repo_id,
    r.provider,
    r.repo_full_name,
    rut.user_id,
    u.login,
    rut.tool,
    rut.day,
    rut.commits,
    rut.insert_time
FROM repo_user_daily_tool rut
JOIN repos r ON rut.repo_id = r.repo_id
JOIN users u ON rut.user_id = u.user_id
WHERE
    rut.day < '2005-01-01'::DATE
    OR rut.day > (CURRENT_DATE + INTERVAL '1 day')::DATE
ORDER BY rut.day DESC;
