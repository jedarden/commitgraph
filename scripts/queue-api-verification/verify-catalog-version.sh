#!/bin/bash
# Verify catalog_version table in queue-api SQLite
# Usage: ./scripts/queue-api-verification/verify-catalog-version.sh

set -e

KUBECONFIG="${KUBECONFIG:-~/.kube/ord-devimprint-admin.kubeconfig}"
POD="${POD:-queue-api-c5894c469-p9rhr}"
NAMESPACE="${NAMESPACE:-commitgraph}"
DB_PATH="${DB_PATH:-/data/queue.db}"

echo "=== Verifying catalog_version table ==="
echo "Pod: ${POD}"
echo "Namespace: ${NAMESPACE}"
echo "Database: ${DB_PATH}"
echo ""

kubectl --kubeconfig="${KUBECONFIG}" exec \
  -n "${NAMESPACE}" "${POD}" -c queue-api \
  -- sqlite3 "${DB_PATH}" <<'EOF'
.mode column
.headers on

-- Verify singleton row exists
SELECT * FROM catalog_version;

SELECT '---', '---';

-- Check schema
PRAGMA table_info(catalog_version);
EOF

echo ""
echo "✅ Verification complete"
