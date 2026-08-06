# author_login_cache Test Sample

## Overview

This directory contains a test sample of the author_login_cache data extracted from the claude-leaderboard database.

## Files

### sample.db

A SQLite database containing a sample of 50 author_login_cache pairs extracted from the source database at `~/backups/claude-leaderboard/hot.db`.

**Source:** ~/backups/claude-leaderboard/hot.db (author_login_cache table)  
**Sample Size:** 50 pairs  
**Extracted:** 2026-08-06

## Schema

The sample database matches the source schema:

```sql
CREATE TABLE author_login_cache (
    author_email TEXT NOT NULL PRIMARY KEY,
    github_login TEXT,
    resolved_at TIMESTAMP
);
```

## Usage

This sample database can be used for testing the seed-author-login-cache script:

```bash
./seed-author-login-cache \
  -claude-leaderboard-db cmd/seed-author-login-cache/testdata/sample.db \
  -db-host localhost \
  -db-user test_user \
  -db-password test_password
```

## Sample Data Characteristics

- **Total pairs:** 50
- **All pairs have:** Non-empty github_login values (positive cache entries only)
- **Email domains:** Various (gmail.com, protonmail.com, outlook.com, organizational domains)
- **GitHub usernames:** Mix of individual accounts and bot accounts
- **Timestamps:** All from 2026-03-14 (real cache resolution times)

## Source Database Statistics

For reference, the source database contains:
- **Total pairs:** 349,425
- **This sample:** 50 (0.014% of total)

The sample was extracted using:
```sql
SELECT author_email, github_login, resolved_at 
FROM author_login_cache 
WHERE github_login IS NOT NULL AND github_login != '' 
LIMIT 50;
```
