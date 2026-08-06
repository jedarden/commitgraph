# Database Connection Testing

This document describes the database connection testing infrastructure for the commitgraph project.

## Overview

The `test-db-connection` tool provides comprehensive validation of PostgreSQL database connectivity before running seed scripts or data ingestion operations. It validates connection setup, connection pool configuration, timeout behavior, and retry logic.

## Tool Location

- **Binary**: `bin/test-db-connection` (build with `go build -o bin/test-db-connection ./cmd/test-db-connection/`)
- **Source**: `cmd/test-db-connection/main.go`
- **Convenience script**: `scripts/test-db-connection.sh`

## Features

### 1. Connection Testing with Retry Logic
- Tests database connectivity with configurable retry attempts
- Logs connection time and ping time for performance monitoring
- Validates database version and current database name
- Supports both flags and environment variables for configuration

### 2. Connection Pool Configuration
- Validates `SetMaxOpenConns`, `SetMaxIdleConns`, and `SetConnMaxLifetime` settings
- Reports connection pool statistics after testing
- Ensures proper resource management for production workloads

### 3. Connection Timeout Validation
- Tests timeout behavior with short and normal timeout values
- Ensures the connection respects configured timeout limits
- Helps identify network latency issues

### 4. Query Execution Testing
- Validates basic query execution capabilities
- Tests timestamp queries and system table access
- Checks for expected commitgraph tables if database exists

## Usage

### Command Line Flags

```bash
test-db-connection -db-host <host> -db-user <user> -db-password <password> [flags]
```

**Required Flags:**
- `-db-host`: PostgreSQL host address
- `-db-user`: PostgreSQL username  
- `-db-password`: PostgreSQL password

**Optional Flags:**
- `-db-port`: PostgreSQL port (default: "5432")
- `-db-name`: Database name (default: "commitgraph")
- `-sslmode`: SSL mode (default: "require")

**Connection Pool Settings:**
- `-max-open-conns`: Maximum open connections (default: 10)
- `-max-idle-conns`: Maximum idle connections (default: 5)
- `-conn-max-lifetime`: Connection maximum lifetime (default: 5m)

**Test Settings:**
- `-connection-timeout`: Connection timeout (default: 30s)
- `-retry-attempts`: Number of retry attempts (default: 3)
- `-retry-delay`: Delay between retries (default: 2s)

### Environment Variables

All flags can be set via environment variables:
- `DB_HOST` → `-db-host`
- `DB_PORT` → `-db-port`
- `DB_NAME` → `-db-name`
- `DB_USER` → `-db-user`
- `DB_PASSWORD` → `-db-password`
- `DB_SSLMODE` → `-sslmode`

### Convenience Script

```bash
./scripts/test-db-connection.sh [db-host] [db-user] [db-password] [db-name]
```

## Examples

### Local PostgreSQL Testing

```bash
# Using flags
bin/test-db-connection \
  -db-host localhost \
  -db-user postgres \
  -db-password mypassword \
  -db-name commitgraph

# Using environment variables
export DB_HOST=localhost
export DB_USER=postgres
export DB_PASSWORD=mypassword
bin/test-db-connection

# Using convenience script
./scripts/test-db-connection.sh localhost postgres mypassword commitgraph
```

### Remote PostgreSQL Testing

```bash
bin/test-db-connection \
  -db-host db.example.com \
  -db-port 5433 \
  -db-user app_user \
  -db-password secure_password \
  -db-name commitgraph_production \
  -sslmode require
```

### Kubernetes Deployment Testing

```bash
# Get database URL from Kubernetes secret
POSTGRES_URL=$(kubectl get secret commitgraph-app -n commitgraph -o jsonpath='{.data.postgres-url}' | base64 -d)

# Parse connection details and test
# (Note: you'll need to parse the URL or use the components directly)
export DB_HOST=$(kubectl get secret commitgraph-app -n commitgraph -o jsonpath='{.data.pg-host}' | base64 -d)
export DB_USER=$(kubectl get secret commitgraph-app -n commitgraph -o jsonpath='{.data.pg-user}' | base64 -d)
export DB_PASSWORD=$(kubectl get secret commitgraph-app -n commitgraph -o jsonpath='{.data.pg-password}' | base64 -d)
./scripts/test-db-connection.sh
```

## Expected Output

### Successful Connection

```
=== Database Connection Test ===
Target: localhost:5432/commitgraph
User: postgres
SSL Mode: require

Test 1: Basic connection with retry logic
  Connection attempt 1/3...
  ✓ Connection successful
    Connect time: 45ms
    Ping time: 12ms
    Database: commitgraph
    Version: PostgreSQL 15.4 on x86_64-pc-linux-gnu, compiled by gcc (GCC) 11.3.0, 64-bit

Test 2: Connection pool configuration
  Configured connection pool:
    Max Open Connections: 10
    Max Idle Connections: 5
    Conn Max Lifetime: 5m0s
  ✓ Connection pool configured successfully
    Open Connections: 1
    In Use: 0
    Idle: 1
    Wait Count: 0
    Wait Duration: 0s
    Max Idle Closed: 0
    Max Lifetime Closed: 0

Test 3: Connection timeout
  Testing timeout with 1ms timeout...
  ✓ Timeout respected (deadline exceeded after 2ms)
  Testing with normal 30s timeout...
  ✓ Normal timeout test passed (15ms)

Test 4: Query execution
  Test: Current timestamp query
  ✓ Server time: 2024-08-06T12:34:56Z
  Test: Query pg_tables
  ✓ Public tables count: 8
  Test: Checking for commitgraph tables
  ✓ Found 8/8 expected tables: [email_resolution repos commits commit_authors commit_parents user_aliases audit_log email_revalidation]

=== Test Summary ===

✓ Connection: SUCCESS (45ms connect, 12ms ping)
  Database: commitgraph
✓ Connection Pool: SUCCESS
  Stats: Open=1 Idle=1 InUse=0
✓ Timeout: SUCCESS
✓ Query Execution: SUCCESS

✓ All tests passed - database connection verified successfully
```

### Failed Connection

```
=== Database Connection Test ===
Target: unavailable-host:5432/commitgraph
User: postgres
SSL Mode: require

Test 1: Basic connection with retry logic
  Connection attempt 1/3...
  ✗ sql.Open failed: dial tcp 192.168.1.100:5432: connect: connection refused
  Retrying in 2s...
  Connection attempt 2/3...
  ✗ sql.Open failed: dial tcp 192.168.1.100:5432: connect: connection refused
  Retrying in 2s...
  Connection attempt 3/3...
  ✗ sql.Open failed: dial tcp 192.168.1.100:5432: connect: connection refused
  ✗ All 3 connection attempts failed, last error: dial tcp 192.168.1.100:5432: connect: connection refused

=== Test Summary ===

✗ Connection: FAILED - dial tcp 192.168.1.100:5432: connect: connection refused
✗ Connection Pool: FAILED - dial tcp 192.168.1.100:5432: connect: connection refused
✗ Timeout: FAILED - dial tcp 192.168.1.100:5432: connect: connection refused
✗ Query Execution: FAILED - dial tcp 192.168.1.100:5432: connect: connection refused

✗ Tests failed - database connection not working
```

## Integration with Seed Scripts

The database connection test should be run before any seed script to ensure database connectivity:

```bash
# Test connection first
if ! bin/test-db-connection -db-host "$DB_HOST" -db-user "$DB_USER" -db-password "$DB_PASSWORD"; then
    echo "Database connection test failed, aborting seed operation"
    exit 1
fi

# Run seed script if connection test passed
./seed-email-resolution -db-host "$DB_HOST" -db-user "$DB_USER" -db-password "$DB_PASSWORD" -seed-db "$SEED_DB_PATH"
```

## Exit Codes

- `0`: All tests passed - database connection verified successfully
- `1`: One or more tests failed - database connection not working
- `2`: Usage error - invalid flags or missing required parameters

## Acceptance Criteria Verification

The tool satisfies all acceptance criteria for database connection testing:

1. ✅ **Script connects to database without errors** - Basic connection test validates successful connectivity
2. ✅ **Connection timeout and retry logic work correctly** - Dedicated timeout test and retry logic validation
3. ✅ **Database credentials are properly loaded from environment** - Supports both flags and environment variables
4. ✅ **Connection pool configuration is verified** - Tests connection pool settings and reports statistics
5. ✅ **Log output shows successful connection** - Comprehensive logging with timing and connection details

## Troubleshooting

### Connection Refused
- Verify PostgreSQL is running: `pg_isready -h hostname -p port`
- Check firewall rules and network connectivity
- Ensure correct host and port values

### Authentication Failed  
- Verify username and password are correct
- Check user permissions for the database
- Ensure `pg_hba.conf` allows connections from your client IP

### SSL Errors
- Try `-sslmode disable` for testing (not recommended for production)
- Verify SSL certificate configuration
- Check if PostgreSQL requires SSL connections

### Timeout Issues
- Increase `-connection-timeout` for slower networks
- Check network latency between client and server
- Verify PostgreSQL server load and responsiveness

## Building the Tool

```bash
# Build from source
go build -o bin/test-db-connection ./cmd/test-db-connection/

# Verify build
./bin/test-db-connection -help
```

## Continuous Integration

The tool can be integrated into CI/CD pipelines:

```yaml
# Example GitHub Actions / Argo Workflows step
- name: Test Database Connection
  env:
    DB_HOST: ${{ secrets.DB_HOST }}
    DB_USER: ${{ secrets.DB_USER }}
    DB_PASSWORD: ${{ secrets.DB_PASSWORD }}
  run: |
    if ! ./bin/test-db-connection; then
      echo "Database connection test failed"
      exit 1
    fi
```

## Related Documentation

- [Plan.md](../../docs/plan/plan.md) - Overall project architecture
- [Seed Email Resolution](../../cmd/seed-email-resolution/main.go) - Email resolution seeding
- [Database Schema](../../docs/database-schema.md) - Table structures and constraints
- [Kubernetes Deployment](../../k8s/) - Production deployment configuration
