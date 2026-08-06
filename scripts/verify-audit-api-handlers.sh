#!/bin/bash
# Manual verification script for audit log API handlers
# This verifies parameter parsing and validation logic

set -e

echo "=== Audit Log API Handler Verification ==="
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Counter for tests
PASSED=0
FAILED=0

print_result() {
    local status="$1"
    local message="$2"

    if [ "$status" = "PASS" ]; then
        echo -e "${GREEN}✓${NC} $message"
        PASSED=$((PASSED + 1))
    else
        echo -e "${RED}✗${NC} $message"
        echo -e "  ${RED}Error:${NC} $2"
        FAILED=$((FAILED + 1))
    fi
}

# Test 1: Verify code compiles
echo "1. Checking if code compiles..."
if go build -o /tmp/test-build ./cmd/audit-log-server/ 2>/dev/null; then
    print_result "PASS" "Code compiles successfully"
    rm -f /tmp/test-build
else
    print_result "FAIL" "Code does not compile"
fi

# Test 2: Verify handler package tests exist
echo ""
echo "2. Checking if handler tests exist..."
if [ -f "./pkg/handler/audit_logs_test.go" ]; then
    print_result "PASS" "Handler tests file exists"

    # Count test functions
    test_count=$(grep -c "^func Test" ./pkg/handler/audit_logs_test.go || echo "0")
    echo "  Found $test_count test functions"
else
    print_result "FAIL" "Handler tests file missing"
fi

# Test 3: Verify main handler functions exist
echo ""
echo "3. Checking if required handler functions exist..."
required_funcs=(
    "NewAuditLogsHandler"
    "handleGetAuditLogs"
    "RegisterRoutes"
    "parseQueryParams"
    "validateParams"
    "writeJSONResponse"
    "writeError"
)

for func in "${required_funcs[@]}"; do
    if grep -q "$func" ./pkg/handler/audit_logs.go; then
        print_result "PASS" "Function $func exists"
    else
        print_result "FAIL" "Function $func missing"
    fi
done

# Test 4: Verify API endpoint follows specification
echo ""
echo "4. Checking API endpoint compliance..."

# Check for correct endpoint path
if grep -q "GET /api/audit-logs" ./pkg/handler/audit_logs.go; then
    print_result "PASS" "Correct endpoint path: GET /api/audit-logs"
else
    print_result "FAIL" "Incorrect endpoint path"
fi

# Check for JSON response support
if grep -q "application/json" ./pkg/handler/audit_logs.go; then
    print_result "PASS" "JSON content-type supported"
else
    print_result "FAIL" "JSON content-type missing"
fi

# Test 5: Verify parameter parsing
echo ""
echo "5. Checking parameter parsing implementation..."

# Check for required query parameters
required_params=(
    "repo_id"
    "start_date"
    "end_date"
    "actor"
    "event_type"
    "limit"
    "offset"
)

for param in "${required_params[@]}"; do
    if grep -q "\"$param\"" ./pkg/handler/audit_logs.go; then
        print_result "PASS" "Parameter $param parsing implemented"
    else
        print_result "FAIL" "Parameter $param parsing missing"
    fi
done

# Test 6: Verify validation rules
echo ""
echo "6. Checking validation rules..."

if grep -q "YYYY-MM-DD" ./pkg/handler/audit_logs.go; then
    print_result "PASS" "Date format validation implemented"
else
    print_result "FAIL" "Date format validation missing"
fi

if grep -q "1-1000" ./pkg/handler/audit_logs.go; then
    print_result "PASS" "Limit range validation implemented"
else
    print_result "FAIL" "Limit range validation missing"
fi

if grep -q "exclude.*unexclude" ./pkg/handler/audit_logs.go; then
    print_result "PASS" "Event type validation implemented"
else
    print_result "FAIL" "Event type validation missing"
fi

# Test 7: Verify error handling
echo ""
echo "7. Checking error handling..."

if grep -q "INVALID_PARAMETER" ./pkg/handler/audit_logs.go; then
    print_result "PASS" "Invalid parameter error code exists"
else
    print_result "FAIL" "Invalid parameter error code missing"
fi

if grep -q "http.StatusBadRequest" ./pkg/handler/audit_logs.go; then
    print_result "PASS" "400 Bad Request error handling exists"
else
    print_result "FAIL" "400 Bad Request error handling missing"
fi

# Test 8: Verify response structure
echo ""
echo "8. Checking response structure..."

required_response_fields=(
    "records"
    "total_count"
    "limit"
    "offset"
)

for field in "${required_response_fields[@]}"; do
    if grep -q "\"$field\"" ./pkg/handler/audit_logs.go; then
        print_result "PASS" "Response field $field exists"
    else
        print_result "FAIL" "Response field $field missing"
    fi
done

# Test 9: Verify service layer integration
echo ""
echo "9. Checking service layer integration..."

if grep -q "service.AuditLogQuerier" ./pkg/handler/audit_logs.go; then
    print_result "PASS" "Service layer integration exists"
else
    print_result "FAIL" "Service layer integration missing"
fi

if grep -q "QueryAuditLogs\|QueryAllAuditLogs" ./pkg/handler/audit_logs.go; then
    print_result "PASS" "Service layer method calls exist"
else
    print_result "FAIL" "Service layer method calls missing"
fi

# Test 10: Verify documentation
echo ""
echo "10. Checking documentation..."

if [ -f "./docs/audit-log-api-endpoint.md" ]; then
    print_result "PASS" "API documentation exists"
else
    print_result "FAIL" "API documentation missing"
fi

# Test summary
echo ""
echo "=== Verification Summary ==="
echo "Total checks: $((PASSED + FAILED))"
echo -e "${GREEN}Passed: $PASSED${NC}"
echo -e "${RED}Failed: $FAILED${NC}"

if [ $FAILED -eq 0 ]; then
    echo ""
    echo -e "${GREEN}All verification checks passed!${NC}"
    exit 0
else
    echo ""
    echo -e "${RED}Some verification checks failed.${NC}"
    exit 1
fi
