# Test Sample File Verification (cg-5s71b)

## Summary

Located and verified the test sample file for the seed-author-login-cache script. All acceptance criteria met.

## Test Sample File Location

**Full Path:** `/home/coding/commitgraph/cmd/seed-author-login-cache/testdata/sample.db`

## File Details

### Format
- **File Type:** SQLite database
- **Schema:** Single table `author_login_cache` with three columns:
  - `author_email` (TEXT, NOT NULL, PRIMARY KEY)
  - `github_login` (TEXT)
  - `resolved_at` (TIMESTAMP)

### Contents
- **Row Count:** 50 author_login_cache pairs
- **Data Quality:** All pairs have non-empty github_login values (positive cache entries only)
- **Sample Characteristics:**
  - Email domains: gmail.com, protonmail.com, outlook.com, organizational domains
  - GitHub usernames: Mix of individual and bot accounts
  - Timestamps: All from 2026-03-14 (real cache resolution times)

### Source Information
- **Source Database:** `~/backups/claude-leaderboard/hot.db`
- **Source Table:** `author_login_cache`
- **Total Source Pairs:** 349,425
- **Sample Size:** 50 pairs (0.014% of total)
- **Extraction Method:** `SELECT author_email, github_login, resolved_at FROM author_login_cache WHERE github_login IS NOT NULL AND github_login != '' LIMIT 50;`

## Verification Results

✅ **Database file exists and is readable**
- Location: `/home/coding/commitgraph/cmd/seed-author-login-cache/testdata/sample.db`
- Permissions: `-rw-r--r--` (644)
- Size: 12,288 bytes

✅ **File format is identified**
- SQLite database format
- Matches source database schema exactly

✅ **File path is documented**
- Full path documented in this file and in `/home/coding/commitgraph/cmd/seed-author-login-cache/testdata/README.md`

✅ **File is accessible for reading**
- Verification script executed successfully
- Database queries work correctly
- Sample data readable and well-formed

## Usage with Seed Script

The sample database can be used to test the seed-author-login-cache script:

```bash
./seed-author-login-cache \
  -claude-leaderboard-db cmd/seed-author-login-cache/testdata/sample.db \
  -db-host localhost \
  -db-user test_user \
  -db-password test_password
```

Or from the repository root:

```bash
go run cmd/seed-author-login-cache/main.go \
  -claude-leaderboard-db cmd/seed-author-login-cache/testdata/sample.db \
  -db-host localhost \
  -db-user test_user \
  -db-password test_password
```

## Verification Script

A verification script exists at:
`/home/coding/commitgraph/cmd/seed-author-login-cache/testdata/verify_sample.sh`

This script validates:
- Database file existence
- Table schema correctness
- Row count (expected: 50)
- Column presence (author_email, github_login, resolved_at)
- Sample data integrity

## Documentation Files

1. **This file:** `/home/coding/commitgraph/notes/cg-5s71b.md` (verification report)
2. **Extraction notes:** `/home/coding/commitgraph/docs/notes/cg-2sfn6-sample-extraction.md` (extraction details)
3. **Sample README:** `/home/coding/commitgraph/cmd/seed-author-login-cache/testdata/README.md` (usage guide)

## Acceptance Criteria Status

- ✅ Test sample file is located
- ✅ File format is identified
- ✅ File path is documented
- ✅ File is accessible for reading

All acceptance criteria have been met. The test sample file is ready for use by the seed-author-login-cache script.
