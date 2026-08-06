# Database Connection Verification (cg-2sp5c)

## Summary

Verified database connection parameters and identified the verification tooling for the commitgraph PostgreSQL database.

## Database Connection Parameters Identified

### PostgreSQL Connection Details
- **Database Type**: PostgreSQL 
- **Default Port**: 5432
- **Default Database Name**: `commitgraph`
- **SSL Mode**: `require` (default)
- **Driver**: `github.com/lib/pq` (lib/pq)

### Required Parameters (from cmd/verify-db and cmd/seed-author-login-cache)
- `db-host` (required): PostgreSQL host address
- `db-port` (default: "5432"): PostgreSQL port
- `db-name` (default: "commitgraph"): PostgreSQL database name
- `db-user` (required): PostgreSQL user
- `db-password` (required): PostgreSQL password
- `sslmode` (default: "require"): PostgreSQL SSL mode

### Connection String Format
```
host=%s port=%s dbname=%s user=%s password=%s sslmode=%s
```

## Database Schema Identified

### Tables Expected (from verify-db command)
1. `repos` - Repository metadata
2. `users` - User information
3. `email_resolution` - Email to username resolution mappings
4. `user_aliases` - User alias mappings
5. `repo_user_daily_tool` - Daily tool usage rollups per repo/user
6. `corpus_stats` - Aggregate corpus statistics

### Schema Files
- `migrations/00001_initial_schema.sql` - Initial schema creation
- `migrations/00002_create_tombstones.sql` - Tombstone tables
- `migrations/00003_create_email_revalidation.sql` - Email revalidation
- `migrations/00004_add_repo_queue_kind.sql` - Queue kind field
- `migrations/00005_add_repo_exclusion_fields.sql` - Exclusion fields

## Database Access Methods

### 1. Kubernetes Environment
**Location**: `k8s/login-revalidation-worker-deployment.yaml`

The Kubernetes deployments access the database via environment variables:
```yaml
env:
- name: POSTGRES_URL
  valueFrom:
    secretKeyRef:
      name: commitgraph-app
      key: postgres-url
```

This indicates:
- Credentials stored in Kubernetes secret `commitgraph-app`
- Key `postgres-url` contains the full connection URL
- Used by containers in the `commitgraph` namespace

### 2. Direct Connection Commands
**Verification Tool**: `cmd/verify-db/main.go` (compiled binary: `verify-db`)

This command performs comprehensive checks:
1. ✓ Database connection parameters identification
2. ✓ Database connection test via ping
3. ✓ Credentials validation (queries PostgreSQL version)
4. ✓ Database schema reachability (checks table existence)
5. ✓ Table accessibility verification (queries row counts)

**Usage**:
```bash
./verify-db -db-host <host> -db-user <user> -db-password <pass> [flags]
```

### 3. Seed Scripts
**Example**: `cmd/seed-author-login-cache/main.go`

Seed scripts use the same connection pattern with additional SQLite database for source data:
- PostgreSQL target database (connection via flags)
- SQLite source database (claude-leaderboard cache)

## Verification Tool Capabilities

The `verify-db` command provides comprehensive verification:

### Check 1: Connection Parameters
- Displays all connection parameters
- Validates required parameters are provided

### Check 2: Connection Test
- Opens database connection using lib/pq driver
- Performs ping test to verify connectivity

### Check 3: Credential Validation  
- Queries PostgreSQL version to confirm credentials work
- Displays PostgreSQL version info

### Check 4: Schema Reachability
- Verifies all expected tables exist in `public` schema
- Checks table accessibility via information schema queries

### Additional Check: Data Accessibility
- Queries row counts for all major tables
- Confirms read access to data

## Database Credential Storage

### Kubernetes Secrets
- **Secret Name**: `commitgraph-app`
- **Namespace**: `commitgraph`
- **Keys**: 
  - `postgres-url`: Full PostgreSQL connection URL
  - `github-token`: GitHub API token for workers

### Local Development
- Connection parameters provided via command-line flags
- No hardcoded credentials in source code
- Secure credential passing required

## Connection Testing Status

### ✓ Identified
- [x] Database connection parameters are clearly documented
- [x] Connection string format is standardized across all tools
- [x] Schema structure is well-defined
- [x] Verification tooling exists and is comprehensive

### Testing Requires
- Database host address
- Database user credentials  
- Database password
- Network access to PostgreSQL server

### Current Status
- Connection parameters: **IDENTIFIED ✓**
- Verification tool: **AVAILABLE ✓** (`verify-db` command)
- Credential access: **Requires Kubernetes secret access or manual provision**
- Network connectivity: **Not tested without actual credentials**

## Recommendations

1. **For Development**: Use the `verify-db` command with local PostgreSQL instance
2. **For Kubernetes Testing**: Access the `commitgraph-app` secret via kubectl
3. **For CI/CD**: Configure verify-db with appropriate environment variables
4. **Security**: Never hardcode credentials; always use secret management

## Conclusion

The database connection infrastructure is well-designed with:
- Clear parameter requirements
- Standardized connection approach
- Comprehensive verification tooling
- Secure credential management via Kubernetes secrets

The `verify-db` command provides all verification capabilities needed for acceptance criteria completion. To run actual connection tests, database credentials from the Kubernetes secret or local development database are required.
