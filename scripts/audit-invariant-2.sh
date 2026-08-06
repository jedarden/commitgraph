#!/usr/bin/env bash
#
# Production audit script for Invariant 2: No out-of-range days
#
# This script runs the invariant SQL assertion against production Postgres
# and alerts if violations are found. It is designed to be run periodically
# (e.g., via cron or Kubernetes CronJob) to catch any data integrity issues.
#
# Usage: ./scripts/audit-invariant-2.sh [production|staging]
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
INVARIANT_SQL="${PROJECT_ROOT}/migrations/invariant_2_no_out_of_range_days.sql"
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
echo "Invariant 2 Audit: ${ENVIRONMENT}"
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
    echo "✅ PASS: No out-of-range dates detected"
    echo ""
    echo "All rows in repo_user_daily_tool have day values within [2005-01-01, current_date + 1]"
    exit 0
else
    echo "❌ FAIL: Found ${VIOLATION_COUNT} violations of Invariant 2"
    echo ""
    echo "VIOLATIONS:"
    echo "------------"
    echo "${VIOLATIONS}"
    echo ""

    # Create alert message
    ALERT_MESSAGE="🚨 **Invariant 2 Violation in ${ENVIRONMENT}**

${VIOLATION_COUNT} rows in \`repo_user_daily_tool\` have \`day\` outside the valid range [2005-01-01, current_date + 1].

This is a **data integrity issue** that must be investigated immediately.

**Environment:** ${ENVIRONMENT}
**Database:** ${POSTGRES_DB}
**Violations:** ${VIOLATION_COUNT}

**Next steps:**
1. Investigate the source of these bad dates
2. Check if clone-worker is applying the date clamp correctly
3. Review recent code changes to migration/clone-worker
4. Consider rolling back recent deployments if this started after a deploy

**Run manually:**
\`\`\`bash
psql -h ${POSTGRES_HOST} -d ${POSTGRES_DB} -U ${POSTGRES_USER} -f ${INVARIANT_SQL}
\`\`\`

This incident is related to historical bead bf-jyctj (commit 93dc8d1) where a 2170-dated commit zeroed the board-wide AI-commit count."

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
