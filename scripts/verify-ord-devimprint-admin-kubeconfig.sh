#!/bin/bash
# Verification script for ord-devimprint-admin.kubeconfig
# This script tests that the refreshed kubeconfig works correctly

set -e

KUBECONFIG="/home/coding/.kube/ord-devimprint-admin.kubeconfig"

echo "=== Testing ord-devimprint-admin.kubeconfig ==="
echo "Date: $(date -Iseconds)"
echo

# Check if file exists
if [ ! -f "$KUBECONFIG" ]; then
    echo "❌ FAIL: kubeconfig file does not exist at $KUBECONFIG"
    echo "Please refresh the token from Rackspace Spot UI first."
    exit 1
fi

echo "✅ kubeconfig file exists"
echo

# Test 1: Basic node access
echo "Test 1: Getting nodes (cluster access)..."
if kubectl --kubeconfig="$KUBECONFIG" get nodes &>/dev/null; then
    echo "✅ PASS: Can get nodes"
else
    echo "❌ FAIL: Cannot get nodes - token may be expired (401)"
    exit 1
fi
echo

# Test 2: Cluster-admin access (CRDs are cluster-scoped)
echo "Test 2: Listing CRDs (cluster-admin access)..."
if kubectl --kubeconfig="$KUBECONFIG" get crds &>/dev/null; then
    echo "✅ PASS: Can list CRDs (cluster-admin confirmed)"
else
    echo "❌ FAIL: Cannot list CRDs - may not have cluster-admin access"
    exit 1
fi
echo

# Test 3: Access to devimprint namespace
echo "Test 3: Accessing devimprint namespace..."
if kubectl --kubeconfig="$KUBECONFIG" get pods -n devimprint &>/dev/null; then
    echo "✅ PASS: Can access devimprint namespace"
else
    echo "❌ FAIL: Cannot access devimprint namespace"
    exit 1
fi
echo

# Test 4: Access to queue-api pod (specific need for extraction)
echo "Test 4: Checking queue-api pod access..."
QUEUE_API_POD=$(kubectl --kubeconfig="$KUBECONFIG" get pods -n devimprint -l app=queue-api -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -n "$QUEUE_API_POD" ]; then
    echo "✅ PASS: Found queue-api pod: $QUEUE_API_POD"

    # Test exec access (this is what we need for extraction)
    if kubectl --kubeconfig="$KUBECONFIG" exec -n devimprint "$QUEUE_API_POD" -- ls /app &>/dev/null; then
        echo "✅ PASS: Can exec into queue-api pod (extraction access confirmed)"
    else
        echo "⚠️  WARNING: Cannot exec into queue-api pod - may need additional permissions"
    fi
else
    echo "⚠️  WARNING: queue-api pod not found - may not be running"
fi
echo

echo "=== All Critical Tests Passed ==="
echo "The ord-devimprint-admin.kubeconfig is working correctly."
echo
echo "Next steps for email_resolution extraction:"
echo "1. Verify queue-api is running: kubectl --kubeconfig=$KUBECONFIG get pods -n devimprint -l app=queue-api"
echo "2. Access the database: kubectl --kubeconfig=$KUBECONFIG exec -n devimprint -it \$(kubectl --kubeconfig=$KUBECONFIG get pods -n devimprint -l app=queue-api -o jsonpath='{.items[0].metadata.name}') -- sqlite3 /data/queue.db"
echo "3. Export email_resolution table: .dump email_resolution"
echo
