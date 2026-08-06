#!/bin/bash
# Verification script for sample_cache_data.csv
# cg-k547t: Verify sample data integrity

set -euo pipefail

SAMPLE_FILE="$(dirname "$0")/sample_cache_data.csv"
ERRORS=0

echo "=== Sample Data Integrity Verification ==="
echo "File: $SAMPLE_FILE"
echo ""

# Check if file exists
if [ ! -f "$SAMPLE_FILE" ]; then
    echo "❌ FAIL: Sample file not found"
    exit 1
fi

echo "✅ Sample file exists"

# Count data rows (excluding header)
ROW_COUNT=$(tail -n +2 "$SAMPLE_FILE" | wc -l)
echo "📊 Data row count: $ROW_COUNT"

# Verify count is between 10-100
if [ "$ROW_COUNT" -ge 10 ] && [ "$ROW_COUNT" -le 100 ]; then
    echo "✅ PASS: Row count within acceptable range (10-100)"
else
    echo "❌ FAIL: Row count $ROW_COUNT outside acceptable range (10-100)"
    ((ERRORS++))
fi

# Count NULL logins
NULL_COUNT=$(tail -n +2 "$SAMPLE_FILE" | cut -d',' -f2 | grep -c '^NULL$' || true)
echo "🔍 NULL login count: $NULL_COUNT"

# Count non-NULL logins
NON_NULL_COUNT=$((ROW_COUNT - NULL_COUNT))
echo "🔍 Non-NULL login count: $NON_NULL_COUNT"

# Verify both types present
if [ "$NULL_COUNT" -gt 0 ] && [ "$NON_NULL_COUNT" -gt 0 ]; then
    echo "✅ PASS: Both NULL and non-NULL logins present"
else
    echo "❌ FAIL: Missing either NULL or non-NULL logins"
    ((ERRORS++))
fi

# Verify NULL logins are properly represented
if tail -n +2 "$SAMPLE_FILE" | cut -d',' -f2 | grep -q '^NULL$'; then
    echo "✅ PASS: NULL logins properly represented as 'NULL' string"
else
    echo "❌ FAIL: NULL logins not properly represented"
    ((ERRORS++))
fi

# Verify email format for non-NULL rows
# Simple regex for email validation
EMAIL_REGEX='^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$'
INVALID_EMAILS=$(tail -n +2 "$SAMPLE_FILE" | cut -d',' -f1 | grep -vE "$EMAIL_REGEX" || true)

if [ -z "$INVALID_EMAILS" ]; then
    echo "✅ PASS: All non-NULL email addresses appear valid"
else
    echo "❌ FAIL: Found potentially invalid email addresses:"
    echo "$INVALID_EMAILS"
    ((ERRORS++))
fi

# Verify timestamp format (ISO 8601 with fractional seconds)
# Check for two acceptable patterns:
# 1. YYYY-MM-DDTHH:MM:SS.microseconds+TZ (e.g., 2026-08-06T10:00:00.000000+00:00)
# 2. YYYY-MM-DDTHH:MM:SS.microsecondsZ (e.g., 2026-06-29T20:14:17.787578Z)
# Note: Allow 5-6 digits for fractional seconds to handle database storage variations
TIMESTAMP_PATTERN='^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}\.[0-9]{5,6}(\+[0-9]{2}:[0-9]{2}|Z)$'
INVALID_TIMESTAMPS=$(tail -n +2 "$SAMPLE_FILE" | cut -d',' -f3 | grep -vE "$TIMESTAMP_PATTERN" || true)

if [ -z "$INVALID_TIMESTAMPS" ]; then
    echo "✅ PASS: All timestamps match ISO 8601 format with microseconds"
else
    echo "❌ FAIL: Found timestamps with invalid format:"
    echo "$INVALID_TIMESTAMPS"
    ((ERRORS++))
fi

# Check for data corruption (no empty columns)
EMPTY_COLUMNS=$(tail -n +2 "$SAMPLE_FILE" | grep -E '^(,|,,|,,?$)' || true)

if [ -z "$EMPTY_COLUMNS" ]; then
    echo "✅ PASS: No empty columns detected (no obvious corruption)"
else
    echo "❌ FAIL: Found rows with empty columns (possible corruption):"
    echo "$EMPTY_COLUMNS"
    ((ERRORS++))
fi

# Verify CSV structure (exactly 3 columns per row)
MALFORMED_ROWS=$(tail -n +2 "$SAMPLE_FILE" | while IFS= read -r line; do
    COLUMN_COUNT=$(echo "$line" | grep -o ',' | wc -l)
    if [ "$COLUMN_COUNT" -ne 2 ]; then
        echo "$line"
    fi
done)

if [ -z "$MALFORMED_ROWS" ]; then
    echo "✅ PASS: All rows have exactly 3 columns (author_email, github_login, resolved_at)"
else
    echo "❌ FAIL: Found rows with incorrect column count:"
    echo "$MALFORMED_ROWS"
    ((ERRORS++))
fi

# Sample data preview
echo ""
echo "=== Sample Data Preview (first 5 rows) ==="
head -6 "$SAMPLE_FILE" | while IFS= read -r line; do
    echo "$line"
done

echo ""
echo "=== Verification Summary ==="
if [ $ERRORS -eq 0 ]; then
    echo "✅ ALL CHECKS PASSED"
    echo "Sample data is valid and ready for use"
    exit 0
else
    echo "❌ $ERRORS check(s) FAILED"
    echo "Please review the errors above"
    exit 1
fi
