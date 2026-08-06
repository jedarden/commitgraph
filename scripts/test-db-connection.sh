#!/bin/bash
# test-db-connection.sh - Database connection test demonstration script
#
# This script demonstrates how to use the test-db-connection tool to verify
# PostgreSQL database connectivity before running seed scripts or data ingestion.
#
# Usage:
#   ./scripts/test-db-connection.sh [db-host] [db-user] [db-password] [db-name]
#
# Examples:
#   # Test with local PostgreSQL
#   ./scripts/test-db-connection.sh localhost postgres mypassword commitgraph
#
#   # Test with remote PostgreSQL
#   ./scripts/test-db-connection.sh db.example.com app_user app_pass commitgraph
#
#   # Test with environment variables
#   export DB_HOST=localhost
#   export DB_USER=postgres
#   export DB_PASSWORD=mypassword
#   ./scripts/test-db-connection.sh

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
TEST_TOOL="$PROJECT_DIR/bin/test-db-connection"

echo "=== Database Connection Test Demo ==="
echo ""

# Check if test tool exists
if [ ! -f "$TEST_TOOL" ]; then
    echo -e "${RED}Error: test-db-connection tool not found at $TEST_TOOL${NC}"
    echo "Please build it first with: go build -o bin/test-db-connection ./cmd/test-db-connection/"
    exit 1
fi

# Function to run connection test
run_test() {
    local db_host="$1"
    local db_user="$2"
    local db_password="$3"
    local db_name="${4:-commitgraph}"

    echo "Testing connection to:"
    echo "  Host: $db_host"
    echo "  Database: $db_name"
    echo "  User: $db_user"
    echo ""

    # Run the test tool
    if "$TEST_TOOL" \
        -db-host="$db_host" \
        -db-user="$db_user" \
        -db-password="$db_password" \
        -db-name="$db_name" \
        -db-port="5432" \
        -sslmode="prefer"; then
        echo ""
        echo -e "${GREEN}✓ Database connection test PASSED${NC}"
        return 0
    else
        echo ""
        echo -e "${RED}✗ Database connection test FAILED${NC}"
        return 1
    fi
}

# Main logic
main() {
    local db_host="${1:-}"
    local db_user="${2:-}"
    local db_password="${3:-}"
    local db_name="${4:-commitgraph}"

    # Check for environment variables if arguments not provided
    if [ -z "$db_host" ] && [ -n "${DB_HOST:-}" ]; then
        db_host="$DB_HOST"
    fi
    if [ -z "$db_user" ] && [ -n "${DB_USER:-}" ]; then
        db_user="$DB_USER"
    fi
    if [ -z "$db_password" ] && [ -n "${DB_PASSWORD:-}" ]; then
        db_password="$DB_PASSWORD"
    fi
    if [ -z "$db_name" ] && [ -n "${DB_NAME:-}" ]; then
        db_name="$DB_NAME"
    fi

    # Check for required parameters
    if [ -z "$db_host" ] || [ -z "$db_user" ] || [ -z "$db_password" ]; then
        echo -e "${YELLOW}Usage:${NC}"
        echo "  $0 [db-host] [db-user] [db-password] [db-name]"
        echo ""
        echo "Or set environment variables:"
        echo "  export DB_HOST=hostname"
        echo "  export DB_USER=username"
        echo "  export DB_PASSWORD=password"
        echo "  export DB_NAME=database_name  # optional"
        echo ""
        echo -e "${YELLOW}Examples:${NC}"
        echo "  # Test connection using arguments"
        echo "  $0 localhost postgres mypass commitgraph"
        echo ""
        echo "  # Test connection using environment variables"
        echo "  export DB_HOST=localhost"
        echo "  export DB_USER=postgres"
        echo "  export DB_PASSWORD=mypass"
        echo "  $0"
        echo ""
        echo -e "${YELLOW}For Kubernetes deployments:${NC}"
        echo "  kubectl get secret commitgraph-app -n commitgraph -o jsonpath='{.data.postgres-url}' | base64 -d"
        exit 1
    fi

    # Run the connection test
    if run_test "$db_host" "$db_user" "$db_password" "$db_name"; then
        exit 0
    else
        exit 1
    fi
}

# Run main function
main "$@"
