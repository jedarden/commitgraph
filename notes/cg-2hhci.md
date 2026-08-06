# author_login_cache Schema Exploration

## Database Location
`~/backups/claude-leaderboard/hot.db`

## Table Schema

```sql
CREATE TABLE author_login_cache (
    author_email TEXT PRIMARY KEY,
    github_login TEXT NOT NULL,
    resolved_at TIMESTAMP NOT NULL
);
```

## Schema Details

| Column | Data Type | Constraints | Description |
|--------|-----------|-------------|-------------|
| author_email | TEXT | PRIMARY KEY | Email address as primary key |
| github_login | TEXT | NOT NULL | Resolved GitHub login (no NULLs) |
| resolved_at | TIMESTAMP | NOT NULL | Resolution timestamp |

## Data Statistics

- **Row count:** 349,425 (matches expected count from parent bead cg-3i96)
- **NULL logins:** 0 (confirmed positive resolutions only)
- **Timestamp format:** ISO 8601 string with timezone offset
- **Timestamp range:** 2026-03-14 to 2026-06-29

## Sample Data

```
author_email                  | github_login   | resolved_at
------------------------------|----------------|---------------------------
bot@quantifieduncertainty.org | quri-bot       | 2026-03-14T21:20:01.065651+00:00
lukeleeai@gmail.com           | lukeleeai      | 2026-03-14T21:20:03.258360+00:00
davebuda256@gmail.com         | Davebuda       | 2026-03-14T21:20:04.683494+00:00
smigolsmigol@protonmail.com   | smigolsmigol   | 2026-03-14T21:20:06.474761+00:00
andrewmbourne@gmail.com       | andrewmichael  | 2026-03-14T21:20:08.059084+00:00
```

## Key Findings for email_resolution Seeding

1. **Source column mapping:**
   - `author_email` → email field in commitgraph
   - `github_login` → login field for email_resolution
   - `resolved_at` → resolution timestamp

2. **Data quality:**
   - All 349,425 rows have non-NULL github_login values
   - Primary key on author_email ensures uniqueness
   - Timestamps are ISO 8601 formatted strings with microsecond precision

3. **Constraints:**
   - This represents positive resolutions only (confirmed by NULL check)
   - Email addresses are unique (PRIMARY KEY)
   - No missing data points

## Next Steps

Use this schema to:
1. Export data from `author_login_cache`
2. Transform to commitgraph's `email_resolution` table format
3. Seed the initial email resolution cache
