#!/usr/bin/env bash
#
# Manual audit script for Invariant 4: Identity referential integrity
#
# This script can be run manually against any environment to check for violations.
# It runs all three queries and reports results.
#
# Usage:
#   export POSTGRES_HOST=localhost
#   export POSTGRES_DB=commitgraph
#   export POSTGRES_USER=postgres
#   export POSTGRES_PASSWORD_FILE=/path/to/password
#   ./scripts/audit-invariant-4.sh [environment]
#
# Environment: optional, defaults to "production"
# Exit codes: 0=pass, 1=violations, 2=error

set -euo pipefail

INVARIANT_SQL="${INVARIANT_SQL:-/home/coding/commitgraph/migrations/invariant_4_identity_referential_integrity.sql}"
ENVIRONMENT="${1:-production}"

# Required environment variables
if [[ -z "${POSTGRES_HOST:-}" ]]; then
  echo "ERROR: POSTGRES_HOST required" >&2
  exit 2
fi

if [[ -z "${POSTGRES_DB:-}" ]]; then
  echo "ERROR: POSTGRES_DB required" >&2
  exit 2
fi

if [[ -z "${POSTGRES_USER:-}" ]]; then
  echo "ERROR: POSTGRES_USER required" >&2
  exit 2
fi

if [[ -z "${POSTGRES_PASSWORD_FILE:-}" ]]; then
  echo "ERROR: POSTGRES_PASSWORD_FILE required" >&2
  exit 2
fi

# Read password
POSTGRES_PASSWORD=$(cat "${POSTGRES_PASSWORD_FILE}")

echo "========================================"
echo "Invariant 4 Audit: ${ENVIRONMENT}"
echo "========================================"
echo "Database: ${POSTGRES_DB}"
echo "Host: ${POSTGRES_HOST}"
echo ""

# Run all three queries from the invariant SQL file
TOTAL_VIOLATIONS=0
QUERY_RESULTS=""

# Query (a): Rollup user_id FK integrity
echo "Running Query (a): Rollup user_id FK integrity..."
QUERY_A=$(PGPASSWORD="${POSTGRES_PASSWORD}" psql \
    -h "${POSTGRES_HOST}" \
    -p "${POSTGRES_PORT:-5432}" \
    -d "${POSTGRES_DB}" \
    -U "${POSTGRES_USER}" \
    -t -A \
    -c "
      SELECT
          rut.repo_id,
          r.provider,
          r.repo_full_name,
          rut.user_id,
          rut.tool,
          rut.day,
          rut.commits,
          rut.insert_time
      FROM repo_user_daily_tool rut
      JOIN repos r ON rut.repo_id = r.repo_id
      LEFT JOIN users u ON rut.user_id = u.user_id
      WHERE u.user_id IS NULL
      ORDER BY rut.repo_id, rut.user_id, rut.day;
    " \
    2>&1) || true

if [[ "${QUERY_A}" == *"ERROR:"* ]]; then
  echo "❌ ERROR: Query (a) failed" >&2
  echo "${QUERY_A}" >&2
  exit 2
fi

COUNT_A=0
if [[ -n "${QUERY_A}" ]]; then
  COUNT_A=$(echo "${QUERY_A}" | grep -c '^[^[:space:]]' || true)
fi

echo "  Query (a) violations: ${COUNT_A}"
TOTAL_VIOLATIONS=$((TOTAL_VIOLATIONS + COUNT_A))

if [[ ${COUNT_A} -gt 0 ]]; then
  QUERY_RESULTS="${QUERY_RESULTS}
Query (a) - Orphan user_id in rollup (${COUNT_A} rows):
${QUERY_A}
"
fi

# Query (b): user_aliases.target_login exists in users.login
echo "Running Query (b): user_aliases.target_login exists in users.login..."
QUERY_B=$(PGPASSWORD="${POSTGRES_PASSWORD}" psql \
    -h "${POSTGRES_HOST}" \
    -p "${POSTGRES_PORT:-5432}" \
    -d "${POSTGRES_DB}" \
    -U "${POSTGRES_USER}" \
    -t -A \
    -c "
      SELECT
          ua.source_login,
          ua.target_login,
          ua.reason,
          ua.created_at
      FROM user_aliases ua
      LEFT JOIN users u ON ua.target_login = u.login
      WHERE u.login IS NULL
      ORDER BY ua.source_login;
    " \
    2>&1) || true

if [[ "${QUERY_B}" == *"ERROR:"* ]]; then
  echo "❌ ERROR: Query (b) failed" >&2
  echo "${QUERY_B}" >&2
  exit 2
fi

COUNT_B=0
if [[ -n "${QUERY_B}" ]]; then
  COUNT_B=$(echo "${QUERY_B}" | grep -c '^[^[:space:]]' || true)
fi

echo "  Query (b) violations: ${COUNT_B}"
TOTAL_VIOLATIONS=$((TOTAL_VIOLATIONS + COUNT_B))

if [[ ${COUNT_B} -gt 0 ]]; then
  QUERY_RESULTS="${QUERY_RESULTS}
Query (b) - Aliases targeting non-existent logins (${COUNT_B} rows):
${QUERY_B}
"
fi

# Query (c): Alias graph acyclic + one-level-deep
echo "Running Query (c): Alias graph acyclic + one-level-deep..."
QUERY_C=$(PGPASSWORD="${POSTGRES_PASSWORD}" psql \
    -h "${POSTGRES_HOST}" \
    -p "${POSTGRES_PORT:-5432}" \
    -d "${POSTGRES_DB}" \
    -U "${POSTGRES_USER}" \
    -t -A \
    -c "
      SELECT
          ua1.source_login AS level1_source,
          ua1.target_login AS level1_target,
          ua2.source_login AS level2_source,
          ua2.target_login AS level2_target,
          ua1.reason AS level1_reason,
          ua2.reason AS level2_reason,
          ua1.created_at AS level1_created,
          ua2.created_at AS level2_created
      FROM user_aliases ua1
      JOIN user_aliases ua2 ON ua1.source_login = ua2.target_login
      ORDER BY ua1.source_login, ua2.source_login;
    " \
    2>&1) || true

if [[ "${QUERY_C}" == *"ERROR:"* ]]; then
  echo "❌ ERROR: Query (c) failed" >&2
  echo "${QUERY_C}" >&2
  exit 2
fi

COUNT_C=0
if [[ -n "${QUERY_C}" ]]; then
  COUNT_C=$(echo "${QUERY_C}" | grep -c '^[^[:space:]]' || true)
fi

echo "  Query (c) violations: ${COUNT_C}"
TOTAL_VIOLATIONS=$((TOTAL_VIOLATIONS + COUNT_C))

if [[ ${COUNT_C} -gt 0 ]]; then
  QUERY_RESULTS="${QUERY_RESULTS}
Query (c) - Chained aliases or cycles (${COUNT_C} rows):
${QUERY_C}
"
fi

echo ""
echo "========================================"
echo "Total violations: ${TOTAL_VIOLATIONS}"
echo "========================================"
echo ""

if [[ "${TOTAL_VIOLATIONS}" -eq 0 ]]; then
  echo "✅ PASS: Identity referential integrity maintained"
  echo "   - All rollup user_id values reference valid users"
  echo "   - All alias targets exist in users table"
  echo "   - Alias graph is acyclic and one-level-deep"
  exit 0
else
  echo "❌ FAIL: ${TOTAL_VIOLATIONS} total violations"
  echo ""
  echo "Breakdown:"
  echo "  Query (a): ${COUNT_A} orphan user_id violation(s)"
  echo "  Query (b): ${COUNT_B} non-existent target_login violation(s)"
  echo "  Query (c): ${COUNT_C} chain/cycle violation(s)"
  echo ""
  echo "Violation details:"
  echo "${QUERY_RESULTS}"

  # Create alert if SLACK_WEBHOOK_URL is set
  if [[ -n "${SLACK_WEBHOOK_URL:-}" ]]; then
    ALERT_MESSAGE="🚨 Invariant 4 Violation in ${ENVIRONMENT}: ${TOTAL_VIOLATIONS} total violations
- Query (a): ${COUNT_A} orphan user_id
- Query (b): ${COUNT_B} bad target_login
- Query (c): ${COUNT_C} chain/cycle violations"

    curl -s -X POST "${SLACK_WEBHOOK_URL}" \
        -H 'Content-Type: application/json' \
        -d "{\"text\": \"${ALERT_MESSAGE}\"}" || true
  fi

  exit 1
fi
