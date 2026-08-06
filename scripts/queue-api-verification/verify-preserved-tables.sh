#!/bin/bash
# Verify both preserved tables: repo_head_cursors and catalog_version
# Usage: ./scripts/queue-api-verification/verify-preserved-tables.sh

set -e

KUBECONFIG="${KUBECONFIG:-~/.kube/ord-devimprint-admin.kubeconfig}"
POD="${POD:-queue-api-c5894c469-p9rhr}"
NAMESPACE="${NAMESPACE:-commitgraph}"
DB_PATH="${DB_PATH:-/data/queue.db}"

echo "=== Verifying Preserved queue-api Tables ==="
echo "Pod: ${POD}"
echo "Namespace: ${NAMESPACE}"
echo "Database: ${DB_PATH}"
echo ""

kubectl --kubeconfig="${KUBECONFIG}" exec \
  -n "${NAMESPACE}" "${POD}" -c queue-api \
  -- sqlite3 "${DB_PATH}" <<'EOF'
.mode column
.headers on
.width 30 10

echo "--- Summary ---";
SELECT 'Table Name' as table_name, 'Row Count' as row_count;
SELECT '---', '---';

-- Get row counts for both preserved tables
SELECT 'repo_head_cursors' as table_name, COUNT(*) as row_count
FROM repo_head_cursors
UNION ALL
SELECT 'catalog_version', COUNT(*) FROM catalog_version;

SELECT '';

echo "--- repo_head_cursors Schema ---";
PRAGMA table_info(repo_head_cursors);

SELECT '';

echo "--- catalog_version Schema ---";
PRAGMA table_info(catalog_version);

SELECT '';

echo "--- Sample repo_head_cursors Data (5 rows) ---";
SELECT * FROM repo_head_cursors LIMIT 5;

SELECT '';

echo "--- catalog_version Data ---";
SELECT * FROM catalog_version;
EOF

echo ""
echo "✅ All preserved tables verified"
echo ""
echo "⚠️  CRITICAL REMINDER:"
echo "   The queue-api PVC must be preserved permanently."
echo "   These tables are required by the new pipeline:"
echo "   - repo_head_cursors: Warm-start incremental cloning"
echo "   - catalog_version: Detection catalog version tracking"
