# Environment Variables and System Requirements

This document describes the required environment variables and system resources needed to run commitgraph applications and tools.

**Last Updated:** 2026-08-08

## Disk Space Requirements

### Current System Status
- **Main Filesystem (`/`):** 436G total, 389G used, **25G available** (95% usage)
- **Temporary Filesystem (`/tmp`):** 32G total, 3.6G used, **28G available** (12% usage)
- **Log Directory (`/var/log`):** Mounted on main filesystem, **25G available**

### Minimum Requirements
- **Main filesystem:** At least 10GB free space recommended for logs, databases, and application data
- **Temporary files:** At least 5GB free space for build artifacts, temporary processing files
- **Log files:** Sufficient space for application logs (growth rate depends on workload)

### Monitoring Recommendations
⚠️ **Warning:** The main filesystem is currently at 95% capacity. Monitor disk usage closely and consider cleanup or expansion before running production workloads.

## Required Environment Variables

### Database Connection Variables

These variables are required for PostgreSQL database connectivity used by most commitgraph applications.

#### Option 1: Single Connection URL
```bash
export DATABASE_URL="postgres://user:password@host:port/database"
```

#### Option 2: Individual Components
```bash
export DB_HOST="localhost"           # PostgreSQL host address
export DB_PORT="5432"               # PostgreSQL port (default: 5432)
export DB_NAME="commitgraph"        # Database name (default: commitgraph)
export DB_USER="postgres"           # PostgreSQL username
export DB_PASSWORD="password"       # PostgreSQL password
export DB_SSLMODE="require"         # SSL mode (default: require)
```

#### Option 3: Native PostgreSQL Variables
```bash
export PGHOST="localhost"
export PGPORT="5432"
export PGDATABASE="commitgraph"
export PGUSER="postgres"
export PGPASSWORD="password"
```

### Application Variables

#### GitHub API Access
```bash
export GITHUB_TOKEN="ghp_xxxxxxxxxxxx"  # GitHub personal access token for repository operations
```

#### Queue API Integration
```bash
export QUEUE_API_URL="http://queue-api:8080"           # Queue API endpoint
export QUEUE_API_INTERNAL_TOKEN="internal-token"       # Internal authentication token
```

#### Alternative Database URL Format
```bash
export POSTGRES_URL="postgres://user:password@host:port/database"  # Production format used by workers
```

### Test Database Variables (Optional)

Used only for testing and development:
```bash
export TEST_DB_URL="postgres://test_user:test_password@test_host:5432/test_db"
export TEST_DB_HOST="localhost"
export TEST_DB_USER="test_user"
export TEST_DB_PASSWORD="test_password"
export TEST_DB_NAME="commitgraph_test"
```

## Usage Examples

### Testing Database Connection
```bash
# Using environment variables
export DB_HOST="localhost"
export DB_USER="postgres"
export DB_PASSWORD="mypassword"
./bin/test-db-connection

# Using command-line flags
./bin/test-db-connection -db-host localhost -db-user postgres -db-password mypassword
```

### Running Seed Scripts
```bash
# Set database variables
export DB_HOST="localhost"
export DB_USER="postgres"
export DB_PASSWORD="mypassword"
export DB_NAME="commitgraph"

# Run seed script
./cmd/seed-email-resolution/main
```

### Running Workers
```bash
# Set required variables
export POSTGRES_URL="postgres://user:password@host:5432/commitgraph"
export GITHUB_TOKEN="ghp_xxxxxxxxxxxx"

# Run worker
./containers/login-revalidation-worker/login-revalidation-worker
```

## Production Deployment

In Kubernetes production deployments, these variables are typically sourced from secrets:

- `GITHUB_TOKEN` → Secret: `commitgraph-app/github-token`
- `POSTGRES_URL` → Secret: `commitgraph-app/postgres-url`

See `k8s/login-revalidation-worker-deployment.yaml` for production configuration examples.

## Verification

To verify your environment is properly configured:

```bash
# Check environment variables are set
echo "DATABASE_URL: $DATABASE_URL"
echo "DB_HOST: $DB_HOST"
echo "GITHUB_TOKEN: ${GITHUB_TOKEN:0:10}..."  # Shows first 10 chars only

# Check disk space
df -h /

# Test database connection
./bin/test-db-connection
```

## Troubleshooting

### Environment Variables Not Set
- **Symptom:** Applications fail with "database connection failed" or "missing environment variable" errors
- **Solution:** Export required variables before running applications, or create a `.env` file for local development

### Insufficient Disk Space
- **Symptom:** Application writes fail, database operations fail, system becomes unstable
- **Solution:** Clean up old log files, remove unused data, or expand disk capacity

### Database Connection Failures
- **Symptom:** "connection refused" or "authentication failed" errors
- **Solution:** Verify PostgreSQL is running, check credentials, ensure network connectivity

## Related Documentation

- [Database Connection Testing](database-connection-testing.md) - Detailed testing procedures
- [Kubernetes Deployment](../k8s/) - Production deployment configurations
- [README.md](../README.md) - Project overview and setup
