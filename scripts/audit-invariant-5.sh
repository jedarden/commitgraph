#!/usr/bin/env bash
#
# Production audit script for Invariant 5: Uniform scan time
#
# This script runs the invariant SQL assertion against production Postgres
# and alerts if violations are found. It is designed to be run periodically
# (e.g., via cron or Kubernetes CronJob) to catch any data integrity issues.
#
# A violation of Invariant 5 means a repo has rows with mixed insert_time values,
# which indicates a broken write path - partial writes escaped the whole-slice
# DELETE + INSERT transaction.
#
# Usage: ./scripts/audit-invariant-5.sh [production|staging]
#
# Exit codes:
#   0 - No violations found (audit passed)
#   1 - Violations found (audit failed - ALERT SHOULD BE TRIGGERED)
#   2 - Error running audit
#
# Environment variables:
#   POSTGRES_HOST - Postgres host (default: localhost)
#   POSTGRES_PORT - Postgres port (default: 5432)
#   POSTGRES_DB   - Database name (required)
#   POSTGRES_USER - Postgres user (required)
#   POSTGRES_PASSWORD_FILE - Path to file containing password (required)
#   SLACK_WEBHOOK_URL - Slack webhook for alerts (optional)
#

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
INVARIANT_SQL="${PROJECT_ROOT}/migrations/invariant_5_uniform_scan_time.sql"
ENVIRONMENT="${1:-production}"

# Default values
POSTGRES_HOST="${POSTGRES_HOST:-localhost}"
POSTGRES_PORT="${POSTGRES_PORT:-5432}"

# Required environment variables
if [[ -z "${POSTGRES_DB:-}" ]]; then
    echo "ERROR: POSTGRES_DB environment variable is required" >&2
    exit 2
fi

if [[ -z "${POSTGRES_USER:-}" ]]; then
    echo "ERROR: POSTGRES_USER environment variable is required" >&2
    exit 2
fi

if [[ -z "${POSTGRES_PASSWORD_FILE:-}" ]]; then
    echo "ERROR: POSTGRES_PASSWORD_FILE environment variable is required" >&2
    exit 2
fi

if [[ ! -f "${POSTGRES_PASSWORD_FILE}" ]]; then
    echo "ERROR: Password file not found: ${POSTGRES_PASSWORD_FILE}" >&2
    exit 2
fi

# Read password
POSTGRES_PASSWORD=$(cat "${POSTGRES_PASSWORD_FILE}")

# Check invariant SQL exists
if [[ ! -f "${INVARIANT_SQL}" ]]; then
    echo "ERROR: Invariant SQL not found: ${INVARIANT_SQL}" >&2
    exit 2
fi

echo "========================================"
echo "Invariant 5 Audit: ${ENVIRONMENT}"
echo "========================================"
echo "Host: ${POSTGRES_HOST}:${POSTGRES_PORT}"
echo "Database: ${POSTGRES_DB}"
echo "User: ${POSTGRES_USER}"
echo ""

# Run the invariant SQL and capture violations
# We use psql with -t to get tuples only, -A to disable alignment
VIOLATIONS=$(PGPASSWORD="${POSTGRES_PASSWORD}" psql \
    -h "${POSTGRES_HOST}" \
    -p "${POSTGRES_PORT}" \
    -d "${POSTGRES_DB}" \
    -U "${POSTGRES_USER}" \
    -t \
    -A \
    -F ' ' \
    -f "${INVARIANT_SQL}" \
    2>&1) || true

# Check if query execution failed
if [[ "${VIOLATIONS}" == *"ERROR:"* ]]; then
    echo "❌ ERROR: Failed to execute invariant query" >&2
    echo "${VIOLATIONS}" >&2
    exit 2
fi

# Count violations (split by newlines, filter empty strings)
VIOLATION_COUNT=0
if [[ -n "${VIOLATIONS}" ]]; then
    VIOLATION_COUNT=$(echo "${VIOLATIONS}" | grep -c '^[^[:space:]]' || true)
fi

echo "Audit Results"
echo "-------------"
echo "Violations found: ${VIOLATION_COUNT}"
echo ""

if [[ "${VIOLATION_COUNT}" -eq 0 ]]; then
    echo "✅ PASS: No repos with mixed insert_time values"
    echo ""
    echo "All repos in repo_user_daily_tool have uniform insert_time values."
    echo "This confirms the DELETE + INSERT write path is working correctly."
    exit 0
else
    echo "❌ FAIL: Found ${VIOLATION_COUNT} repos with mixed insert_time values"
    echo ""
    echo "VIOLATIONS:"
    echo "------------"
    
    # Parse and format violations for readability
    # Format: repo_id | provider | repo_full_name | distinct_count | insert_time_samples | total_rows | earliest_day | latest_day
    echo "${VIOLATIONS}" | while IFS=' ' read -r repo_id provider repo_full_name distinct_count insert_time_samples total_rows earliest_day latest_day; do
        echo ""
        echo "Repo: ${provider}/${repo_full_name} (repo_id=${repo_id})"
        echo "  Distinct insert_time values: ${distinct_count}"
        echo "  Total rows affected: ${total_rows}"
        echo "  Date range: ${earliest_day} to ${latest_day}"
        echo "  Sample insert_time values: ${insert_time_samples}"
    done
    
    echo ""

    # Create alert message
    ALERT_MESSAGE="🚨 **Invariant 5 Violation in ${ENVIRONMENT}**

${VIOLATION_COUNT} repos in \`repo_user_daily_tool\` have rows with mixed \`insert_time\` values.

This is a **critical data integrity issue** that indicates a broken write path. Under the correct DELETE + INSERT transaction pattern, all rows for a repo must have the same \`insert_time\` (the timestamp when that repo was last scanned).

**What this means:**
- A partial write escaped the whole-slice-replace transaction
- Rows may have been inserted outside the transactional write path
- Concurrent writes to the same repo (race condition)
- Transaction rollbacks may not be working correctly

**Environment:** ${ENVIRONMENT}
**Database:** ${POSTGRES_DB}
**Violating repos:** ${VIOLATION_COUNT}

**Next steps:**
1. **INVESTIGATE IMMEDIATELY** - This affects data correctness
2. Check the violating repos listed above for pattern analysis
3. Review clone-worker write path for transaction handling bugs
4. Verify no concurrent writes are happening to the same repo
5. Check Postgres logs for transaction errors or rollbacks
6. Consider rolling back recent deployments if this started after a deploy

**Run manually:**
\`\`\`bash
psql -h ${POSTGRES_HOST} -d ${POSTGRES_DB} -U ${POSTGRES_USER} -f ${INVARIANT_SQL}
\`\`\`

**Impact:**
- Rankings may be incorrect for affected repos
- Scan recency metrics (\`MAX(insert_time)\`) will be wrong
- User-facing \"last scanned\" timestamps will be inconsistent"

    # Send Slack alert if webhook configured
    if [[ -n "${SLACK_WEBHOOK_URL:-}" ]]; then
        echo "Sending Slack alert..."
        curl -s -X POST "${SLACK_WEBHOOK_URL}" \
            -H 'Content-Type: application/json' \
            -d "{\"text\": \"${ALERT_MESSAGE}\"}" > /dev/null || echo "Warning: Failed to send Slack alert"
    fi

    # Log to stderr for logging systems
    echo "${ALERT_MESSAGE}" >&2

    exit 1
fi
