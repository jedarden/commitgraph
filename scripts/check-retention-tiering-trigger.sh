#!/usr/bin/env bash
#
# Retention Tiering Trigger Check
#
# This script checks the size of repo_user_daily_tool and alerts when it
# crosses the threshold that should trigger a retention tiering design discussion.
#
# This does NOT implement tiering — it only measures when it would be worth considering.
# See docs/plan.md section "Retention tiering -- gated on measurement" for full context.
#
# Usage: ./scripts/check-retention-tiering-trigger.sh [production|staging]
#
# Exit codes:
#   0 - Size below threshold (no action needed)
#   1 - Size above threshold (RETENTION TIERING DESIGN SHOULD BE TRIGGERED)
#   2 - Error running check
#
# Environment variables:
#   POSTGRES_HOST - Postgres host (default: localhost)
#   POSTGRES_PORT - Postgres port (default: 5432)
#   POSTGRES_DB   - Database name (required)
#   POSTGRES_USER - Postgres user (required)
#   POSTGRES_PASSWORD_FILE - Path to file containing password (required)
#   SLACK_WEBHOOK_URL - Slack webhook for alerts (optional)
#   THRESHOLD_BYTES - Size threshold in bytes (default: 524288000 = 500MB)
#

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
CHECK_SQL="${PROJECT_ROOT}/migrations/check_retention_tiering_trigger.sql"
ENVIRONMENT="${1:-production}"

# Default values
POSTGRES_HOST="${POSTGRES_HOST:-localhost}"
POSTGRES_PORT="${POSTGRES_PORT:-5432}"
THRESHOLD_BYTES="${THRESHOLD_BYTES:-524288000}"  # 500MB default

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

# Check SQL file exists
if [[ ! -f "${CHECK_SQL}" ]]; then
    echo "ERROR: SQL check file not found: ${CHECK_SQL}" >&2
    exit 2
fi

echo "========================================"
echo "Retention Tiering Trigger Check: ${ENVIRONMENT}"
echo "========================================"
echo "Host: ${POSTGRES_HOST}:${POSTGRES_PORT}"
echo "Database: ${POSTGRES_DB}"
echo "User: ${POSTGRES_USER}"
echo "Threshold: $(numfmt --to=iec-i --suffix=B ${THRESHOLD_BYTES})" 2>/dev/null || echo "Threshold: ${THRESHOLD_BYTES} bytes"
echo ""

# Run the size check and capture output
# We use psql with -t to get tuples only, -A to disable alignment
RESULT=$(PGPASSWORD="${POSTGRES_PASSWORD}" psql \
    -h "${POSTGRES_HOST}" \
    -p "${POSTGRES_PORT}" \
    -d "${POSTGRES_DB}" \
    -U "${POSTGRES_USER}" \
    -t \
    -A \
    -F ' ' \
    -f "${CHECK_SQL}" \
    2>&1) || true

# Check if query execution failed
if [[ "${RESULT}" == *"ERROR:"* ]]; then
    echo "❌ ERROR: Failed to execute size check query" >&2
    echo "${RESULT}" >&2
    exit 2
fi

# Parse result (format: size_bytes | size_pretty | timestamp)
SIZE_BYTES=$(echo "${RESULT}" | cut -d'|' -f1 | tr -d ' ')
SIZE_PRETTY=$(echo "${RESULT}" | cut -d'|' -f2 | tr -d ' ')
MEASURED_AT=$(echo "${RESULT}" | cut -d'|' -f3 | tr -d ' ')

# Validate we got numbers
if ! [[ "${SIZE_BYTES}" =~ ^[0-9]+$ ]]; then
    echo "❌ ERROR: Failed to parse size bytes from result: ${SIZE_BYTES}" >&2
    echo "Full result: ${RESULT}" >&2
    exit 2
fi

echo "Current Size Measurements"
echo "-------------------------"
echo "Size: ${SIZE_PRETTY} (${SIZE_BYTES} bytes)"
echo "Threshold: $(numfmt --to=iec-i --suffix=B ${THRESHOLD_BYTES} 2>/dev/null || echo "${THRESHOLD_BYTES} bytes")"
echo "Measured at: ${MEASURED_AT}"
echo ""

# Check threshold
if [[ "${SIZE_BYTES}" -lt "${THRESHOLD_BYTES}" ]]; then
    PERCENT_OF_THRESHOLD=$(echo "scale=1; ${SIZE_BYTES} * 100 / ${THRESHOLD_BYTES}" | bc 2>/dev/null || echo "N/A")
    echo "✅ PASS: Table size is below threshold"
    echo ""
    echo "repo_user_daily_tool is at ${PERCENT_OF_THRESHOLD}% of the threshold."
    echo "No retention tiering action needed at this time."
    echo ""
    echo "Context from docs/plan.md:"
    echo "  - Current projected size at 234K AI commits: ~15MB (data + indexes)"
    echo "  - With 2x bloat multiplier: ~30MB"
    echo "  - This threshold (500MB) is ~33x larger than current size"
    echo "  - At this size, retention tiering would have material value"
    exit 0
else
    PERCENT_OF_THRESHOLD=$(echo "scale=1; ${SIZE_BYTES} * 100 / ${THRESHOLD_BYTES}" | bc 2>/dev/null || echo "N/A")
    echo "❌ THRESHOLD EXCEEDED: Table size is ${PERCENT_OF_THRESHOLD}% of trigger threshold"
    echo ""
    echo "⚠️  **RETENTION TIERING DESIGN SHOULD BE TRIGGERED** ⚠️"
    echo ""
    echo "repo_user_daily_tool has grown to ${SIZE_PRETTY}, exceeding the"
    echo "threshold of $(numfmt --to=iec-i --suffix=B ${THRESHOLD_BYTES} 2>/dev/null || echo "${THRESHOLD_BYTES} bytes")."
    echo ""
    echo "This indicates the table has grown enough that retention tiering would"
    echo "have material storage and performance value."
    echo ""

    # Create alert message
    ALERT_MESSAGE="🚨 **Retention Tiering Threshold Exceeded in ${ENVIRONMENT}**

\`repo_user_daily_tool\` has grown to **${SIZE_PRETTY}**, exceeding the **$(numfmt --to=iec-i --suffix=B ${THRESHOLD_BYTES} 2>/dev/null || echo "${THRESHOLD_BYTES} bytes")** trigger threshold.

**Current status:**
- Size: ${SIZE_PRETTY} (${SIZE_BYTES} bytes)
- Threshold: $(numfmt --to=iec-i --suffix=B ${THRESHOLD_BYTES} 2>/dev/null || echo "${THRESHOLD_BYTES} bytes")
- Percent of threshold: ${PERCENT_OF_THRESHOLD}%

**What this means:**
The table has grown enough that retention tiering would have material value.
This is the trigger to design and implement tiering, as described in
docs/plan.md section \"Retention tiering -- gated on measurement.\"

**IMPORTANT HARD CONSTRAINT (from plan.md):**
Any future tiering implementation **MUST preserve the trailing 30 days at
daily granularity**. The per-user activity histogram reads exactly that
window day-by-day, so collapsing it would break a shipped feature.

**Proposed design (from plan.md):**
- Create a \`(repo_id, user_id, tool, month)\` tier for older data
- Keep daily granularity for the most recent 30+ days (400+ day boundary)
- Maintain the whole-slice-replace idempotency property
- The leaderboard needs a 30-day window and all-time totals; all-time can be
  served from the monthly tier without loss

**Next steps:**
1. Review the actual growth curve and usage patterns
2. Design the tiering schema and migration strategy
3. Ensure the trailing 30 days remain at daily granularity (HARD CONSTRAINT)
4. Plan the migration path (clone-worker transaction change, backfill, etc.)
5. Implement and test in staging before production

**This check was created via bead cg-462u**
**See docs/plan.md section \"Retention tiering -- gated on measurement\" for full context**"

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
