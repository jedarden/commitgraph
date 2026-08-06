#!/bin/bash
# Comprehensive queue-api table extraction verification
#
# This script verifies all four queue-api tables once admin access is restored.
# Usage: ./scripts/verify-all-extractions.sh
#
# Requirements:
# - Admin kubeconfig for ord-devimprint (~/.kube/ord-devimprint-admin.kubeconfig)
# - kubectl with access to ord-devimprint cluster
# - sqlite3 in queue-api container

set -e

KUBECONFIG="${KUBECONFIG:-~/.kube/ord-devimprint-admin.kubeconfig}"
POD="${POD:-queue-api-c5894c469-p9rhr}"
NAMESPACE="${NAMESPACE:-commitgraph}"
DB_PATH="${DB_PATH:-/data/queue.db}"

echo "=== Queue-API Table Extraction Verification ==="
echo "Pod: ${POD}"
echo "Namespace: ${NAMESPACE}"
echo "Database: ${DB_PATH}"
echo "Started: $(date -Iseconds)"
echo ""

# Check admin access
if [[ ! -f "${KUBECONFIG/#\~/$HOME}" ]]; then
    echo "❌ ERROR: Admin kubeconfig not found: ${KUBECONFIG}"
    echo "Please refresh from Rackspace Spot UI and try again."
    exit 1
fi

echo "=== Step 1: Verify Preserved Tables (repo_head_cursors, catalog_version) ==="
echo ""

kubectl --kubeconfig="${KUBECONFIG}" exec \
  -n "${NAMESPACE}" "${POD}" -c queue-api \
  -- sqlite3 "${DB_PATH}" <<'EOF'
.mode column
.headers on
.width 30 10

SELECT '--- Preserved Tables Summary ---' as '';
SELECT 'Table Name' as table_name, 'Row Count' as row_count;
SELECT '---', '---';

SELECT 'repo_head_cursors' as table_name, COUNT(*) as row_count
FROM repo_head_cursors
UNION ALL
SELECT 'catalog_version', COUNT(*) FROM catalog_version;

SELECT '';
SELECT '--- repo_head_cursors Schema ---';
PRAGMA table_info(repo_head_cursors);

SELECT '';
SELECT '--- catalog_version Schema ---';
PRAGMA table_info(catalog_version);

SELECT '';
SELECT '--- Sample repo_head_cursors Data (5 rows) ---';
SELECT * FROM repo_head_cursors LIMIT 5;

SELECT '';
SELECT '--- catalog_version Data ---';
SELECT * FROM catalog_version;
EOF

echo ""
echo "✅ Preserved tables verified"
echo ""
echo "⚠️  CRITICAL REMINDER:"
echo "   The queue-api PVC must be preserved permanently."
echo "   These tables are required by the new pipeline:"
echo "   - repo_head_cursors: Warm-start incremental cloning"
echo "   - catalog_version: Detection catalog version tracking"
echo ""

echo "=== Step 2: Verify Extraction Tables (blocklist, tombstones) ==="
echo ""

kubectl --kubeconfig="${KUBECONFIG}" exec \
  -n "${NAMESPACE}" "${POD}" -c queue-api \
  -- sqlite3 "${DB_PATH}" <<'EOF'
.mode column
.headers on
.width 30 10

SELECT '--- Extraction Tables Summary ---' as '';
SELECT 'Table Name' as table_name, 'Row Count' as row_count;
SELECT '---', '---';

SELECT 'blocklist' as table_name, COUNT(*) as row_count
FROM blocklist
UNION ALL
SELECT 'tombstones', COUNT(*) FROM tombstones;

SELECT '';
SELECT '--- blocklist Schema ---';
PRAGMA table_info(blocklist);

SELECT '';
SELECT '--- blocklist by Kind ---';
SELECT kind, COUNT(*) as count FROM blocklist GROUP BY kind;

SELECT '';
SELECT '--- Sample blocklist Data (5 rows) ---';
SELECT * FROM blocklist LIMIT 5;

SELECT '';
SELECT '--- tombstones Schema ---';
PRAGMA table_info(tombstones);

SELECT '';
SELECT '--- Sample tombstones Data (5 rows) ---';
SELECT * FROM tombstones LIMIT 5;
EOF

echo ""
echo "✅ Extraction tables verified"
echo ""

echo "=== Step 3: Check for Export Files ==="
echo ""

EXPORTS_DIR="/home/coding/commitgraph/exports"
BLOCKLIST_CSV="$EXPORTS_DIR/blocklist.csv"
TOMBSTONES_CSV="$EXPORTS_DIR/tombstones.csv"

if [[ -f "$BLOCKLIST_CSV" ]]; then
    BLOCKLIST_LINES=$(wc -l < "$BLOCKLIST_CSV")
    BLOCKLIST_ROWS=$((BLOCKLIST_LINES - 1))  # Exclude header
    echo "✅ blocklist.csv: $BLOCKLIST_ROWS rows (excluding header)"
    echo "   Header: $(head -n 1 "$BLOCKLIST_CSV")"
else
    echo "⚠️  blocklist.csv not found - extraction pending"
fi

if [[ -f "$TOMBSTONES_CSV" ]]; then
    TOMBSTONES_LINES=$(wc -l < "$TOMBSTONES_CSV")
    TOMBSTONES_ROWS=$((TOMBSTONES_LINES - 1))  # Exclude header
    echo "✅ tombstones.csv: $TOMBSTONES_ROWS rows (excluding header)"
    echo "   Header: $(head -n 1 "$TOMBSTONES_CSV")"
else
    echo "⚠️  tombstones.csv not found - extraction pending"
fi

echo ""
echo "=== Step 4: PVC Status ==="
echo ""

kubectl --kubeconfig="${KUBECONFIG}" get pvc -n "${NAMESPACE}" queue-api-data \
  -o jsonpath='{.metadata.name}{"\t"}{.status.phase}{"\t"}{.spec.storageClassName}{"\t"}{.spec.resources.requests.storage}'

echo ""
echo ""
echo "=== Verification Complete ==="
echo "Finished: $(date -Iseconds)"
echo ""
echo "Summary:"
echo "- Preserved tables: Verified in PVC"
echo "- Extraction tables: Schema verified, export status checked above"
echo "- Next steps: Run extraction scripts if CSV files are missing"
echo ""
echo "Extraction scripts:"
echo "  ./scripts/extract-blocklist.sh"
echo "  ./scripts/extract-tombstones.sh"
