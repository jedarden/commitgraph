#!/bin/bash
# Test script for the audit log API endpoint
# This script tests the REST API endpoint for audit log queries

set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test configuration
SERVER_HOST="localhost"
SERVER_PORT="8080"
BASE_URL="http://${SERVER_HOST}:${SERVER_PORT}"
DB_HOST="${DB_HOST:-localhost}"
DB_USER="${DB_USER:-postgres}"
DB_PASSWORD="${DB_PASSWORD:-postgres}"
DB_NAME="${DB_NAME:-commitgraph}"

# Counter for tests
PASSED=0
FAILED=0

# Function to print test results
print_result() {
    local test_name="$1"
    local status="$2"
    local message="$3"

    if [ "$status" = "PASS" ]; then
        echo -e "${GREEN}✓${NC} $test_name"
        PASSED=$((PASSED + 1))
    else
        echo -e "${RED}✗${NC} $test_name"
        echo -e "  ${RED}Error:${NC} $message"
        FAILED=$((FAILED + 1))
    fi
}

# Function to make API request and check response
test_api_request() {
    local test_name="$1"
    local url="$2"
    local expected_status="$3"
    local check_field="$4"
    local check_value="$5"

    echo "Testing: $test_name"

    response=$(curl -s -w "\n%{http_code}" "$BASE_URL$url")
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')

    if [ "$http_code" != "$expected_status" ]; then
        print_result "$test_name" "FAIL" "Expected HTTP $expected_status, got $http_code"
        echo "  Response body: $body"
        return 1
    fi

    if [ -n "$check_field" ]; then
        if echo "$body" | grep -q "\"$check_field\"[[:space:]]*:[[:space:]]*$check_value"; then
            print_result "$test_name" "PASS" ""
        else
            print_result "$test_name" "FAIL" "Expected field $check_field to have value $check_value"
            echo "  Response body: $body"
        fi
    else
        print_result "$test_name" "PASS" ""
    fi
}

# Function to start the server
start_server() {
    echo "Starting audit log server..."
    ./bin/audit-log-server \
        -db-host "$DB_HOST" \
        -db-user "$DB_USER" \
        -db-password "$DB_PASSWORD" \
        -db-name "$DB_NAME" \
        -port "$SERVER_PORT" \
        -sslmode disable &
    SERVER_PID=$!

    # Wait for server to start
    sleep 2

    # Check if server is running
    if ! kill -0 $SERVER_PID 2>/dev/null; then
        echo -e "${RED}Failed to start server${NC}"
        exit 1
    fi

    echo -e "${GREEN}Server started with PID $SERVER_PID${NC}"
}

# Function to stop the server
stop_server() {
    echo "Stopping server..."
    kill $SERVER_PID 2>/dev/null || true
    wait $SERVER_PID 2>/dev/null || true
    echo -e "${GREEN}Server stopped${NC}"
}

# Check if server binary exists
if [ ! -f "./bin/audit-log-server" ]; then
    echo -e "${RED}Error: Server binary not found at ./bin/audit-log-server${NC}"
    echo "Please build the server first: go build -o bin/audit-log-server ./cmd/audit-log-server/"
    exit 1
fi

# Check if database is accessible
echo "Checking database connection..."
if ! PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1" > /dev/null 2>&1; then
    echo -e "${YELLOW}Warning: Cannot connect to database. Some tests may fail.${NC}"
fi

# Start the server
start_server

# Trap to ensure server is stopped
trap stop_server EXIT INT TERM

echo -e "\n${YELLOW}Running API tests...${NC}\n"

# Health check
echo -e "${YELLOW}--- Health Check Tests ---${NC}"
test_api_request "Health check" "/health" "200" ""

# Basic query tests
echo -e "\n${YELLOW}--- Basic Query Tests ---${NC}"
test_api_request "Basic query with repo_id" "/api/audit-logs?repo_id=1" "200" ""
test_api_request "Query with limit" "/api/audit-logs?repo_id=1&limit=10" "200" "\"limit\"[[:space:]]*:10"

# Parameter validation tests
echo -e "\n${YELLOW}--- Parameter Validation Tests ---${NC}"
test_api_request "Invalid date format (slash)" "/api/audit-logs?repo_id=1&start_date=2024/01/01" "400" ""
test_api_request "Invalid date format (text)" "/api/audit-logs?repo_id=1&start_date=invalid" "400" ""
test_api_request "Invalid date (Feb 30)" "/api/audit-logs?repo_id=1&start_date=2024-02-30" "400" ""
test_api_request "Invalid limit (negative)" "/api/audit-logs?repo_id=1&limit=-1" "400" ""
test_api_request "Invalid limit (too large)" "/api/audit-logs?repo_id=1&limit=1001" "400" ""
test_api_request "Invalid offset (negative)" "/api/audit-logs?repo_id=1&offset=-1" "400" ""
test_api_request "Invalid event_type" "/api/audit-logs?repo_id=1&event_type=invalid" "400" ""

# Date range tests
echo -e "\n${YELLOW}--- Date Range Tests ---${NC}"
test_api_request "Valid start_date" "/api/audit-logs?repo_id=1&start_date=2024-01-01" "200" ""
test_api_request "Valid end_date" "/api/audit-logs?repo_id=1&end_date=2024-12-31" "200" ""
test_api_request "Valid date range" "/api/audit-logs?repo_id=1&start_date=2024-01-01&end_date=2024-12-31" "200" ""
test_api_request "Start date after end date" "/api/audit-logs?repo_id=1&start_date=2024-12-31&end_date=2024-01-01" "400" ""

# Pagination tests
echo -e "\n${YELLOW}--- Pagination Tests ---${NC}"
test_api_request "Default pagination" "/api/audit-logs?repo_id=1" "200" "\"limit\"[[:space:]]*:100"
test_api_request "Custom limit" "/api/audit-logs?repo_id=1&limit=50" "200" "\"limit\"[[:space:]]*:50"
test_api_request "Custom offset" "/api/audit-logs?repo_id=1&offset=10" "200" "\"offset\"[[:space:]]*:10"

# Filter tests
echo -e "\n${YELLOW}--- Filter Tests ---${NC}"
test_api_request "Filter by actor" "/api/audit-logs?repo_id=1&actor=admin@example.com" "200" ""
test_api_request "Filter by event_type (exclude)" "/api/audit-logs?repo_id=1&event_type=exclude" "200" ""
test_api_request "Filter by event_type (unexclude)" "/api/audit-logs?repo_id=1&event_type=unexclude" "200" ""

# Response structure tests
echo -e "\n${YELLOW}--- Response Structure Tests ---${NC}"

echo "Testing: Response headers"
response=$(curl -s -I "$BASE_URL/api/audit-logs?repo_id=1")
if echo "$response" | grep -q "Content-Type: application/json"; then
    print_result "Content-Type header" "PASS" ""
else
    print_result "Content-Type header" "FAIL" "Missing Content-Type: application/json"
fi

# Complex query tests
echo -e "\n${YELLOW}--- Complex Query Tests ---${NC}"
test_api_request "Complex query (multiple filters)" "/api/audit-logs?repo_id=1&start_date=2024-01-01&end_date=2024-12-31&actor=admin@example.com&event_type=exclude&limit=50&offset=0" "200" ""

# Test results summary
echo -e "\n${YELLOW}--- Test Summary ---${NC}"
echo -e "Total tests: $((PASSED + FAILED))"
echo -e "${GREEN}Passed: $PASSED${NC}"
echo -e "${RED}Failed: $FAILED${NC}"

if [ $FAILED -eq 0 ]; then
    echo -e "\n${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "\n${RED}Some tests failed.${NC}"
    exit 1
fi
