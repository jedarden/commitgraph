# Test Sample File Verification (cg-1tqth)

## Task Completed: Locate and verify test sample file

## Test Sample File Location

**Full Path:** `/home/coding/commitgraph/cmd/seed-author-login-cache/testdata/sample.db`

## File Characteristics

- **File Format:** SQLite database
- **File Size:** 12KB
- **Created:** 2026-08-06 01:39:00
- **Permissions:** rw-r--r-- (owner: coding:users)
- **Status:** File exists and is readable

## Database Structure

**Schema:**
```sql
CREATE TABLE author_login_cache (
    author_email TEXT NOT NULL PRIMARY KEY,
    github_login TEXT,
    resolved_at TIMESTAMP
);
```

## Database Contents

- **Total Records:** 50 author_login_cache pairs
- **Table Name:** `author_login_cache`
- **Sample Characteristics:**
  - All pairs have non-empty github_login values (positive cache entries only)
  - Email domains include: gmail.com, protonmail.com, outlook.com, organizational domains
  - GitHub usernames include both individual accounts and bot accounts
  - All timestamps from 2026-03-14 (real cache resolution times)

## Sample Data Examples

```
bot@quantifieduncertainty.org → quri-bot (resolved: 2026-03-14T21:20:01.065651+00:00)
lukeleeai@gmail.com → lukeleeai (resolved: 2026-03-14T21:20:03.258360+00:00)
davebuda256@gmail.com → Davebuda (resolved: 2026-03-14T21:20:04.683494+00:00)
```

## Source Information

According to the README in the testdata directory:
- **Source Database:** `~/backups/claude-leaderboard/hot.db`
- **Extraction Query:** `SELECT author_email, github_login, resolved_at FROM author_login_cache WHERE github_login IS NOT NULL AND github_login != '' LIMIT 50;`
- **Source Database Total Pairs:** 349,425
- **Sample Size:** 50 pairs (0.014% of total)
- **Extracted:** 2026-08-06

## Purpose

This sample database is used for testing the `seed-author-login-cache` script, which seeds author_login_cache data from a claude-leaderboard database into the commitgraph database.

## Acceptance Criteria Status

✅ Test sample file location identified  
✅ File exists and is readable  
✅ File format verified (SQLite database with author_login_cache table)  
✅ Full path and characteristics documented
