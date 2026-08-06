# author_login_cache Test Sample Extraction (cg-2sfn6)

## Summary

Successfully extracted a test sample of 50 author_login_cache pairs from the claude-leaderboard database for use by the seed-author-login-cache script.

## Source Data

**Location:** `~/backups/claude-leaderboard/hot.db`  
**Table:** `author_login_cache`  
**Total pairs in source:** 349,425  
**Sample size:** 50 pairs (0.014% of total)

## Sample Database Location

`/home/coding/commitgraph/cmd/seed-author-login-cache/testdata/sample.db`

## Sample Schema

Matches the source database schema exactly:

```sql
CREATE TABLE author_login_cache (
    author_email TEXT NOT NULL PRIMARY KEY,
    github_login TEXT,
    resolved_at TIMESTAMP
);
```

## Sample Contents

- **50 author_login_cache pairs**
- All pairs have non-empty `github_login` values (positive cache entries only)
- Real email addresses from various domains (gmail.com, protonmail.com, outlook.com, organizational domains)
- Real GitHub usernames (mix of individual and bot accounts)
- Real timestamps from 2026-03-14

## Files Created

1. **sample.db** - SQLite database with 50 sample pairs
2. **README.md** - Documentation of the sample data
3. **verify_sample.sh** - Verification script to validate sample database

## Usage

The sample database can be used for testing the seed-author-login-cache script:

```bash
./seed-author-login-cache \
  -claude-leaderboard-db cmd/seed-author-login-cache/testdata/sample.db \
  -db-host localhost \
  -db-user test_user \
  -db-password test_password
```

## Verification

The sample database has been verified:
- ✅ Database file exists and is readable
- ✅ author_login_cache table exists with correct schema
- ✅ All required columns present (author_email, github_login, resolved_at)
- ✅ Contains exactly 50 rows
- ✅ Data format matches source structure
- ✅ Accessible by seed-author-login-cache script

## Acceptance Criteria Met

- [x] Sample file created with 10-100 author_login_cache pairs (50 pairs)
- [x] Sample data format matches source structure
- [x] Sample file is accessible by the seed script
- [x] Sample size is documented (50 pairs, documented in README)
