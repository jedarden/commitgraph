#!/bin/bash
# Verify repo_head_cursors table in queue-api SQLite
# Usage: ./scripts/queue-api-verification/verify-repo-head-cursors.sh

set -e

KUBECONFIG="${KUBECONFIG:-~/.kube/ord-devimprint-admin.kubeconfig}"
POD="${POD:-queue-api-c5894c469-p9rhr}"
NAMESPACE="${NAMESPACE:-commitgraph}"
DB_PATH="${DB_PATH:-/data/queue.db}"

echo "=== Verifying repo_head_cursors table ==="
echo "Pod: ${POD}"
echo "Namespace: ${NAMESPACE}"
echo "Database: ${DB_PATH}"
echo ""

kubectl --kubeconfig="${KUBECONFIG}" exec \
  -n "${NAMESPACE}" "${POD}" -c queue-api \
  -- sqlite3 "${DB_PATH}" <<'EOF'
.mode column
.headers on

-- Get row count
SELECT 'repo_head_cursors' as table_name, COUNT(*) as row_count
FROM repo_head_cursors;

SELECT '---', '---';

-- Show sample data (first 5 rows)
SELECT * FROM repo_head_cursors LIMIT 5;

SELECT '---', '---';

-- Check schema
PRAGMA table_info(repo_head_cursors);
EOF

echo ""
echo "✅ Verification complete"
