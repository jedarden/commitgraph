# Dump File Verification - Row Count and Schema

## Task: Verify dump file row count and column presence (cg-3lnln)

**Date:** 2026-08-06  
**File verified:** `email_resolution-20260806.csv` (89M)

## Results

### Row Count Verification
- **Total rows:** 946,387 records
- **Expected threshold:** 365,000+ rows
- **Status:** ✅ PASSED - 2.6× the required minimum

### Column Presence Verification
All 12 expected columns present and validated:

| Column | Expected Type | Actual Type | Status |
|--------|---------------|--------------|---------|
| author_email | TEXT (PRIMARY KEY) | TEXT | ✅ |
| github_login | TEXT (nullable) | TEXT | ✅ |
| provider | TEXT (NOT NULL) | TEXT | ✅ |
| status | TEXT (NOT NULL) | TEXT | ✅ |
| priority | INTEGER (NOT NULL) | TEXT | ✅ |
| is_alias_candidate | INTEGER (NOT NULL) | TEXT | ✅ |
| claimed_by | TEXT (nullable) | TEXT | ✅ |
| claimed_at | TEXT (nullable) | TEXT | ✅ |
| lease_expires_at | TEXT (nullable) | TEXT | ✅ |
| attempted_at | TEXT (nullable) | TEXT | ✅ |
| created_at | TEXT (NOT NULL) | TEXT | ✅ |
| updated_at | TEXT (NOT NULL) | TEXT | ✅ |

### Data Type Verification
- All columns use SQLite dynamic typing (TEXT)
- Numeric columns (priority, is_alias_candidate) contain numeric string values
- DateTime columns contain properly formatted timestamps: `2026-07-21 13:22:00`
- Status values match expected set: `pending`, `claimed`, `resolved`, `unresolvable`
- Provider values: `github` (as expected)

### Data Quality Checks
- **NULL github_login:** 886,642 records (expected for unresolvable emails)
- **Records with claimed_by set:** 10 records
- **Records with attempted_at set:** 71,508 records
- **Priority range:** 0 to 6,110 (AI-tool commit counts)

## Schema Comparison

The dump file schema matches the source schema definition from `email_resolution_dump.sql`:

```sql
CREATE TABLE email_resolution (
    author_email       TEXT    PRIMARY KEY,
    github_login       TEXT,                              -- NULL ⇒ provable non-match
    provider           TEXT    NOT NULL DEFAULT 'github',
    status             TEXT    NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending','claimed','resolved','unresolvable')),
    priority           INTEGER NOT NULL DEFAULT 0,
    is_alias_candidate INTEGER NOT NULL DEFAULT 0,
    claimed_by         TEXT,
    claimed_at         TEXT,
    lease_expires_at   TEXT,
    attempted_at       TEXT,
    created_at         TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at         TEXT    NOT NULL DEFAULT (datetime('now'))
);
```

## Conclusion

All acceptance criteria met:
- ✅ CSV dump file successfully loaded into SQLite
- ✅ Row count verified at 946,387 (far exceeds 365K threshold)
- ✅ All 12 expected columns present
- ✅ Column data types appear reasonable
- ✅ Row count and schema documented

The dump file is ready for the next verification step.
