# extract-sample-cache-data

Extracts a sample of author_login_cache pairs from the claude-leaderboard database for testing purposes.

## Purpose

This tool reads from the claude-leaderboard's `author_login_cache` table and outputs a CSV sample containing between 10-100 pairs. It ensures both valid logins and NULL logins are included (if available), and preserves the original ISO 8601 timestamp format with microsecond precision.

## Building

```bash
cd /home/coding/commitgraph
go build -o extract-sample-cache-data ./cmd/extract-sample-cache-data/
```

## Usage

Basic usage:
```bash
./extract-sample-cache-data -output sample.csv
```

With custom sample size:
```bash
./extract-sample-cache-data -output sample.csv -count 25
```

With custom database path:
```bash
./extract-sample-cache-data -output sample.csv -db /path/to/hot.db
```

## Options

- `-db`: Path to claude-leaderboard SQLite database (default: `~/backups/claude-leaderboard/hot.db`)
- `-output`: Output CSV file path (required)
- `-count`: Number of pairs to extract, between 10-100 inclusive (default: 50)

## Output Format

The output CSV has three columns:
- `author_email`: Email address of the author
- `github_login`: GitHub username/login (literal string "NULL" for unresolved identities)
- `resolved_at`: ISO 8601 timestamp with microsecond precision

Example:
```csv
author_email,github_login,resolved_at
bot@quantifieduncertainty.org,quri-bot,2026-03-14T21:20:01.065651Z
unknown.user@example.com,NULL,2026-08-06T10:00:00.000000Z
```

## Database Schema

The tool reads from the `author_login_cache` table in the claude-leaderboard SQLite database:

```sql
CREATE TABLE author_login_cache (
    author_email TEXT PRIMARY KEY,
    github_login TEXT NOT NULL,
    resolved_at TIMESTAMP NOT NULL
);
```

## Behavior

- **Sample size**: Automatically clamps to range [10, 100]
- **NULL logins**: Includes at least 20% NULL logins if available; otherwise logs a warning
- **Timestamp format**: Preserves original ISO 8601 format with microsecond precision
- **Error handling**: Validates inputs and reports errors clearly

## Related Documentation

See `notes/cg-4p4nu.md` for detailed author_login_cache data structure documentation.

## Task

This tool was created for bead cg-5l4fq: "Write script to extract sample data from cache"
