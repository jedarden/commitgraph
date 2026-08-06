#!/usr/bin/env bash
# verify_sample.sh - Verify the author_login_cache sample database

set -e

SAMPLE_DB="$(dirname "$0")/sample.db"

echo "=== Verifying author_login_cache Sample Database ==="
echo "Location: $SAMPLE_DB"
echo ""

# Check if database exists
if [ ! -f "$SAMPLE_DB" ]; then
    echo "❌ Error: Sample database not found at $SAMPLE_DB"
    exit 1
fi

echo "✅ Database file exists"

# Verify table exists
TABLE_CHECK=$(sqlite3 "$SAMPLE_DB" "
SELECT name FROM sqlite_master
WHERE type='table' AND name='author_login_cache';
")

if [ "$TABLE_CHECK" != "author_login_cache" ]; then
    echo "❌ Error: author_login_cache table not found"
    exit 1
fi

echo "✅ author_login_cache table exists"

# Verify schema
SCHEMA_CHECK=$(sqlite3 "$SAMPLE_DB" "PRAGMA table_info(author_login_cache);")
echo ""
echo "Schema:"
echo "$SCHEMA_CHECK"
echo ""

# Count rows
ROW_COUNT=$(sqlite3 "$SAMPLE_DB" "SELECT COUNT(*) FROM author_login_cache;")
echo "✅ Row count: $ROW_COUNT"

# Check for required columns
EMAIL_COL=$(sqlite3 "$SAMPLE_DB" "PRAGMA table_info(author_login_cache);" | grep -c "author_email")
LOGIN_COL=$(sqlite3 "$SAMPLE_DB" "PRAGMA table_info(author_login_cache);" | grep -c "github_login")
RESOLVED_COL=$(sqlite3 "$SAMPLE_DB" "PRAGMA table_info(author_login_cache);" | grep -c "resolved_at")

if [ "$EMAIL_COL" -lt 1 ] || [ "$LOGIN_COL" -lt 1 ] || [ "$RESOLVED_COL" -lt 1 ]; then
    echo "❌ Error: Missing required columns"
    exit 1
fi

echo "✅ All required columns present (author_email, github_login, resolved_at)"

# Sample a few rows
echo ""
echo "Sample data (first 3 rows):"
sqlite3 "$SAMPLE_DB" "SELECT author_email, github_login, resolved_at FROM author_login_cache LIMIT 3;"

echo ""
echo "=== Verification Complete ==="
echo "Sample database is ready for use by seed-author-login-cache"
