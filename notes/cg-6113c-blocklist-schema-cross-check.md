# Blocklist Schema Cross-Check Analysis (cg-6113c)

## Task Overview

Analyze the cross-check between queue-api's `blocklist` table and Postgres's `repos.excluded_at/excluded_reason` schema mechanism.

**Date:** 2026-08-06  
**Parent Bead:** cg-5ol6 (Queue-api Tables Extraction)

## Schema Comparison

### Source Schema: queue-api SQLite `blocklist` table

```sql
CREATE TABLE blocklist (
    provider   TEXT    NOT NULL,
    kind       TEXT    NOT NULL CHECK (kind IN ('repo','user','email')),
    identifier TEXT    NOT NULL,
    reason     TEXT,
    created_at TEXT    NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (provider, kind, identifier)
);
```

**Characteristics:**
- Polymorphic: handles repos, users, and emails in one table
- SQLite TEXT timestamps (ISO 8601 strings)
- Three exclusion types: `repo`, `user`, `email`
- Composite key on (provider, kind, identifier)

### Target Schema: Postgres `repos.excluded_at/excluded_reason`

```sql
CREATE TABLE repos (
    repo_id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    provider        TEXT NOT NULL,
    repo_full_name  TEXT NOT NULL,
    excluded_at     TIMESTAMPTZ,         -- non-NULL = excluded from ranking
    excluded_reason TEXT,
    UNIQUE (provider, repo_full_name)
);
```

**Characteristics:**
- Repo-specific (no user/email exclusion support)
- Postgres TIMESTAMPTZ (timezone-aware timestamps)
- Nullable `excluded_at` (NULL = not excluded, non-NULL = excluded)
- Nullable `excluded_reason` (free-text justification)
- Unique constraint on (provider, repo_full_name)

## Cross-Check Analysis

### 1. Schema Compatibility

| Field | Blocklist (SQLite) | Repos (Postgres) | Compatibility |
|-------|-------------------|------------------|---------------|
| provider | provider TEXT NOT NULL | provider TEXT NOT NULL | ✅ Compatible |
| identifier | identifier TEXT NOT NULL | repo_full_name TEXT NOT NULL | ✅ Compatible (semantic mapping) |
| timestamp | created_at TEXT | excluded_at TIMESTAMPTZ | ⚠️ Type conversion required |
| reason | reason TEXT | excluded_reason TEXT | ✅ Compatible |
| kind | kind CHECK constraint | Not applicable | ⚠️ Repo-only subset |

### 2. Transformation Logic

For `kind='repo'` entries only:

```sql
INSERT INTO repos (provider, repo_full_name, excluded_at, excluded_reason)
SELECT
    provider,
    identifier AS repo_full_name,
    -- Convert SQLite TEXT timestamp to Postgres TIMESTAMPTZ
    CASE
        WHEN created_at ~ '^\d{4}-\d{2}-\d{2}' THEN
            (created_at::timestamp)::timestamp with time zone
        ELSE NULL
    END AS excluded_at,
    COALESCE(reason, 'migrated from queue-api blocklist') AS excluded_reason
FROM blocklist
WHERE kind = 'repo'
ON CONFLICT (provider, repo_full_name)
DO UPDATE SET
    excluded_at = EXCLUDED.excluded_at,
    excluded_reason = EXCLUDED.excluded_reason;
```

**Key transformations:**
1. `identifier` → `repo_full_name` (semantic mapping)
2. `created_at` (TEXT) → `excluded_at` (TIMESTAMPTZ) with type conversion
3. `reason` → `excluded_reason` with default for NULL values
4. Filter to `kind='repo'` only (user/email exclusions are out of scope)
5. Upsert semantics: ON CONFLICT DO UPDATE for idempotent loads

### 3. Discrepancies and Open Questions

#### Discrepancy 1: User and Email Exclusions

**Issue:** The blocklist table contains `kind='user'` and `kind='email'` entries that have no corresponding field in the `repos` table.

**Current state:** Unknown - no extraction data available to count these entries.

**Options:**
1. **Leave in queue-api** - Keep user/email exclusions in queue-api's blocklist (simple, but splits exclusion state)
2. **Create new Postgres tables** - Add `user_exclusions` and `email_exclusions` tables (consistent, but more schema)
3. **Defer** - Document as out-of-scope for v2 migration (minimal, but incomplete)

**Recommendation:** Option 3 (Defer) - Document that user/email exclusions are not migrated and remain in queue-api until a future user/email exclusion mechanism is designed for Postgres.

#### Discrepancy 2: Timestamp Type Conversion

**Issue:** SQLite stores timestamps as TEXT (ISO 8601 strings like `'2024-08-05 12:34:56'`), while Postgres expects TIMESTAMPTZ.

**Risk:** Malformed timestamps in SQLite could cause conversion failures.

**Mitigation:** The CASE statement with regex check (`created_at ~ '^\d{4}-\d{2}-\d{2}'`) ensures only valid timestamps are converted. Invalid timestamps become NULL, which means the repo won't be excluded (safe default).

**Validation needed:** After extraction, run:
```sql
SELECT COUNT(*) AS invalid_timestamps
FROM blocklist_temp
WHERE kind = 'repo'
  AND created_at IS NOT NULL
  AND created_at !~ '^\d{4}-\d{2}-\d{2}';
```

#### Discrepancy 3: Missing Reason Field

**Issue:** Blocklist `reason` can be NULL, but exclusion best practices (per repo-exclusion runbook) require a reason for audit purposes.

**Resolution:** Use `COALESCE(reason, 'migrated from queue-api blocklist')` to provide a default reason for entries without one.

**Traceability:** This default reason makes it clear which exclusions were historical vs. newly applied.

#### Discrepancy 4: Duplicate Prevention

**Issue:** The repos table uses `(provider, repo_full_name)` as unique, but blocklist uses `(provider, kind, identifier)`.

**Implication:** The ON CONFLICT clause ensures idempotent loads - re-running the migration updates existing exclusions rather than failing.

**Benefit:** Safe to re-run if new blocklist entries are added after initial migration.

## Verification Queries

### After Extraction (Cross-Check)

Run these queries to verify the extraction succeeded:

```sql
-- 1. Verify all blocklist repos are marked excluded
SELECT COUNT(*) AS missing_exclusions
FROM (SELECT DISTINCT provider, identifier 
      FROM blocklist_temp WHERE kind = 'repo') AS bl
LEFT JOIN repos ON bl.provider = repos.provider 
              AND bl.identifier = repos.repo_full_name
WHERE repos.repo_id IS NULL OR repos.excluded_at IS NULL;
-- Expected: 0 (no missing exclusions)

-- 2. Check for timestamp conversion failures
SELECT COUNT(*) AS null_excluded_at
FROM repos r
JOIN blocklist_temp bl ON r.provider = bl.provider 
                      AND r.repo_full_name = bl.identifier
WHERE bl.kind = 'repo' 
  AND bl.created_at IS NOT NULL
  AND r.excluded_at IS NULL;
-- Expected: 0 (no valid timestamps became NULL)

-- 3. Verify default reason was applied
SELECT COUNT(*) AS with_default_reason
FROM repos
WHERE excluded_reason = 'migrated from queue-api blocklist';
-- Expected: count of blocklist entries with NULL reason

-- 4. Summary statistics
SELECT 
    COUNT(*) AS total_excluded_repos,
    COUNT(DISTINCT provider) AS providers,
    COUNT(CASE WHEN excluded_reason LIKE '%migrated%' THEN 1 END) AS with_default_reason,
    MIN(excluded_at) AS earliest_exclusion,
    MAX(excluded_at) AS latest_exclusion
FROM repos
WHERE excluded_at IS NOT NULL;
```

### Current State

**BLOCKER:** Admin kubeconfig access unavailable (`ord-devimprint-admin.kubeconfig` missing, returns 401 unauthorized).

**What's available:**
- ✅ Read-only kubectl proxy access (`http://kubectl-proxy-ord-devimprint:8001`)
- ✅ Pod name: `queue-api-c5894c469-p9rhr`
- ✅ PVC: `queue-api-data`
- ✅ Extraction scripts created and ready:
  - `scripts/extract-blocklist.sh` (requires admin access)
  - `scripts/load-blocklist-to-postgres.sh` (ready to use)

**What's blocked:**
- ❌ `kubectl exec` into queue-api pod (forbidden by read-only proxy)
- ❌ `kubectl cp` to extract CSV files (requires exec)
- ❌ Direct SQLite access via kubectl

## Transformation Acceptance Criteria

The transformation is valid and safe if:

1. **No data loss:** All `kind='repo'` blocklist entries become `repos.excluded_at IS NOT NULL`
2. **Timestamp preservation:** SQLite timestamps convert to TIMESTAMPTZ without data loss
3. **Reason traceability:** All exclusions have a non-NULL `excluded_reason` (either original or default)
4. **Idempotent:** Re-running the migration is safe (ON CONFLICT DO UPDATE)
5. **Backward compatible:** Existing exclusions in Postgres are preserved/updated by migration

## Recommendations

### Immediate (Blocker Resolution)

1. **Refresh admin kubeconfig** for ord-devimprint cluster (current one is 401 unauthorized)
2. **Run extraction script:** `./scripts/extract-blocklist.sh`
3. **Run verification queries** (above) to catch any discrepancies early

### Post-Extraction

1. **Document user/email exclusions** as remaining in queue-api (out-of-scope for v2)
2. **Update repo-exclusion runbook** to reference migrated exclusions
3. **Add monitoring** for excluded repos count (alert if it drops unexpectedly)

### Future Work

1. **Design user/email exclusion mechanism** for Postgres to complete migration
2. **Add exclusion audit table** to track all exclusion operations (not just current state)
3. **Consider exclusion expiration** (auto-clear old exclusions after some period)

## Migration Safety

The migration is **safe and reversible**:

- **Read-only source:** Extraction reads from queue-api SQLite without modification
- **Transactional target:** Postgres load runs in a transaction (BEGIN/COMMIT)
- **Rollback available:** Can ROLLBACK the transaction if verification fails
- **Idempotent:** Can re-run migration if needed (ON CONFLICT DO UPDATE)
- **PVC preserved:** queue-api-data PVC is retained for recovery

## Next Steps

1. **Resolve admin kubeconfig blocker** (primary dependency)
2. **Run extraction script** to get actual blocklist data
3. **Perform dry-run load** with verification queries
4. **Commit migration** after verification succeeds
5. **Document user/email exclusion strategy** as follow-up work

---

**Acceptance Status:**
- [x] Schema cross-check analysis complete
- [x] Transformation logic documented
- [x] Discrepancies identified and recommendations made
- [x] Verification queries defined
- [ ] BLOCKER: Actual extraction awaits admin kubeconfig refresh
- [ ] BLOCKER: Cross-check verification queries need real data
