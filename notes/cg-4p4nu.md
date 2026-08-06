# author_login_cache Data Structure Documentation

## Location
- **Source Database**: `/home/coding/backups/claude-leaderboard/hot.db` (SQLite)
- **Test Data**: `/home/coding/commitgraph/cmd/seed-author-login-cache/testdata/author_login_cache_sample.csv`

## File Format
**SQLite Database** with a single table `author_login_cache`

## Schema
```sql
CREATE TABLE author_login_cache (
    author_email TEXT PRIMARY KEY,
    github_login TEXT NOT NULL,
    resolved_at TIMESTAMP NOT NULL
);
```

## Data Structure

### Fields
1. **author_email** (TEXT, PRIMARY KEY)
   - Email address of the author
   - Serves as the primary key
   - Example: `bot@quantifieduncertainty.org`

2. **github_login** (TEXT, NOT NULL)
   - GitHub username/login
   - Can be "NULL" (literal string) for unresolved author identities
   - Example: `quri-bot` or `NULL`

3. **resolved_at** (TIMESTAMP, NOT NULL)
   - Timestamp when the email-to-github login mapping was resolved
   - ISO 8601 format with microsecond precision
   - Example: `2026-03-14T21:20:01.065651+00:00`

## NULL Login Representation
- **Literal string "NULL"** in the `github_login` column
- Example rows from test data:
  ```
  unknown.user1@example.com,NULL,2026-08-06T10:00:00.000000+00:00
  unresolved@email.com,NULL,2026-08-06T10:20:00.000000+00:00
  orphan.email@unknown.com,NULL,2026-08-06T10:40:00.000000+00:00
  ```

## Timestamp Format
- **ISO 8601** with microseconds and timezone
- Pattern: `YYYY-MM-DDTHH:MM:SS.mmmmmm+00:00`
- Always uses `+00:00` timezone (UTC)
- Microsecond precision (6 decimal places)

## Sample Data
```
author_email                          github_login              resolved_at
bot@quantifieduncertainty.org         quri-bot                  2026-03-14T21:20:01.065651+00:00
lukeleeai@gmail.com                   lukeleeai                 2026-03-14T21:20:03.258360+00:00
davebuda256@gmail.com                 Davebuda                  2026-03-14T21:20:04.683494+00:00
unknown.user1@example.com             NULL                      2026-08-06T10:00:00.000000+00:00
```

## Database Statistics
- **Total rows**: 349,425 (from production database)
- **NULL logins**: 0 in production (may be cleaned or fully resolved)
- **Test data**: Includes NULL examples for testing

## Notes
- The seed-author-login-cache binary at `/home/coding/commitgraph/seed-author-login-cache` is ~10.8MB and likely generates or processes this data
- Multiple emails can map to the same github_login (e.g., `github@jedarden.com` and `coder@jedarden.com` both map to `jedarden`)
- The `invalid-email-address` value is used as a github_login placeholder for emails like `root@localhost.localdomain`
