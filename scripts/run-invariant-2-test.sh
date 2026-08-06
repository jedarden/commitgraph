#!/usr/bin/env bash
#
# Quick runner for Invariant 2 testing
#
# This script sets up a local Postgres instance, runs the invariant tests,
# and reports results. It's designed for local development and quick verification.
#
# Usage:
#   ./scripts/run-invariant-2-test.sh        # Run all tests
#   ./scripts/run-invariant-2-test.sh fast  # Skip Postgres setup (use existing)
#
# Environment variables:
#   POSTGRES_HOST - Postgres host (default: localhost)
#   POSTGRES_PORT - Postgres port (default: 5432)
#   KEEP_POSTGRES - Set to 1 to keep Postgres running after test

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${PROJECT_ROOT}"

# Configuration
POSTGRES_HOST="${POSTGRES_HOST:-localhost}"
POSTGRES_PORT="${POSTGRES_PORT:-5432}"
MODE="${1:-full}"
KEEP_POSTGRES="${KEEP_POSTGRES:-}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}ℹ${NC} $*"
}

log_warn() {
    echo -e "${YELLOW}⚠${NC} $*"
}

log_error() {
    echo -e "${RED}✖${NC} $*" >&2
}

cleanup() {
    local exit_code=$?
    if [[ -z "${KEEP_POSTGRES}" ]] && [[ -n "${POSTGRES_CONTAINER:-}" ]]; then
        log_info "Stopping Postgres container..."
        docker stop "${POSTGRES_CONTAINER}" &>/dev/null || true
        docker rm "${POSTGRES_CONTAINER}" &>/dev/null || true
    fi
    exit ${exit_code}
}

trap cleanup EXIT INT TERM

echo "=========================================="
echo "Invariant 2 Test Runner"
echo "=========================================="
echo ""

# Check if Docker is available
if ! command -v docker &> /dev/null; then
    log_error "Docker is required but not installed"
    exit 1
fi

# Check if Python is available
if ! command -v python3 &> /dev/null; then
    log_error "Python 3 is required but not installed"
    exit 1
fi

# Setup Postgres if not in fast mode
if [[ "${MODE}" != "fast" ]]; then
    log_info "Starting Postgres container..."

    # Check if port is already in use
    if docker ps -q | grep -q ":5432->5432"; then
        log_warn "Port 5432 is already in use by another container"
        log_info "Trying to use existing Postgres..."
        MODE="fast"
    else
        # Start Postgres
        POSTGRES_CONTAINER="invariant-2-test-postgres-$$"
        docker run -d \
            --name "${POSTGRES_CONTAINER}" \
            -e POSTGRES_HOST_AUTH_METHOD=trust \
            -p "${POSTGRES_PORT}:5432" \
            postgres:16-alpine \
            -c max_connections=200 \
            -c shared_buffers=128MB

        log_info "Waiting for Postgres to be ready..."
        local max_attempts=30
        local attempt=0
        while (( attempt < max_attempts )); do
            if docker exec "${POSTGRES_CONTAINER}" pg_isready &>/dev/null; then
                break
            fi
            ((attempt++)) || true
            sleep 1
        done

        if (( attempt == max_attempts )); then
            log_error "Postgres did not become ready"
            exit 1
        fi

        log_info "Postgres is ready at ${POSTGRES_HOST}:${POSTGRES_PORT}"
    fi
fi

# Install Python dependencies
log_info "Installing Python dependencies..."
pip install psycopg[binary] -q --disable-pip-version-check

# Run tests
log_info "Running invariant tests..."
echo ""

if PGHOST="${POSTGRES_HOST}" \
   PGPORT="${POSTGRES_PORT}" \
   PGUSER="postgres" \
   PGDATABASE="postgres" \
   python3 migration/test_invariant_2.py
then
    echo ""
    log_info "✅ All tests passed!"
    echo ""
    echo "Summary:"
    echo "  - Invariant 2 SQL correctly detects out-of-range dates"
    echo "  - 2170-dated rows are caught (historical incident reproduction)"
    echo "  - Pre-2005 rows are caught (below minimum bound)"
    echo "  - Valid dates are not flagged (no false positives)"
    echo ""
    echo "See docs/invariant-2-implementation.md for details."
    exit 0
else
    echo ""
    log_error "❌ Tests failed!"
    echo ""
    echo "Check the output above for details."
    exit 1
fi
