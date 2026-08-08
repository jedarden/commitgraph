# Entry Point Package/Module Context

## Overview

This document provides the package/module context for each parse and stream entry point in the commitgraph system. It establishes the organizational context, package hierarchy, and module responsibilities for all entry point functions.

## Package Hierarchy

```
commitgraph/
├── cmd/                    # Command-line tools (executables)
│   ├── audit-logs/
│   ├── load-admin-aliases/
│   ├── load-email-resolution-from-queue-api/
│   ├── get-audit-logs/
│   └── verify-email-resolution-dump/
└── pkg/                    # Library packages (reusable components)
    ├── handler/            # HTTP request handlers
    ├── identity/           # Identity resolution services
    ├── ingestlog/          # Structured logging for ingest operations
    ├── pg/                 # PostgreSQL implementations
    └── warmstart/          # Git repository warm-start functionality
```

## Parse Entry Points

### cmd/audit-logs (parseDate)

**Package:** `cmd/audit-logs`  
**File:** `cmd/audit-logs/main.go:211`  
**Function:** `parseDate(dateStr string) (*time.Time, error)`

**Package Role:**
CLI tool for querying audit logs from the commitgraph database. Provides a modern interface to audit log data using the service layer with support for filtering by repository, date range, actor, and event type with pagination.

**Module Context:**
- **Hierarchy:** Command-line tool (executable package)
- **Responsibility:** User-facing interface for audit log queries
- **Dependencies:** `pkg/service` for database access
- **Output Format:** Supports both table and JSON output formats
- **Entry Point Type:** Date parsing for command-line argument validation

---

### cmd/load-admin-aliases (parseAliasesFromConfigMap)

**Package:** `cmd/load-admin-aliases`  
**File:** `cmd/load-admin-aliases/main.go:227`  
**Function:** `parseAliasesFromConfigMap(configMap *ConfigMap) ([]AliasEntry, error)`

**Package Role:**
Admin alias management tool that loads user-defined source_login → target_login mappings from declarative-config ConfigMap into the user_aliases database table.

**Module Context:**
- **Hierarchy:** Migration/configuration tool (executable package)
- **Responsibility:** Idempotent upsert of admin-defined user aliases
- **Data Flow:** ConfigMap YAML → database upsert with reason='admin'
- **Conflict Resolution:** Uses ON CONFLICT (source_login) DO UPDATE
- **Entry Point Type:** ConfigMap structure parsing and validation

---

### cmd/verify-email-resolution-dump (parseInsertLine)

**Package:** `cmd/verify-email-resolution-dump`  
**File:** `cmd/verify-email-resolution-dump/main.go:65`  
**Function:** `parseInsertLine(line string) (status, attemptedAt, updatedAt string)`

**Package Role:**
Verification tool for email resolution dump format validation. Parses SQL INSERT statements to extract status, attemptedAt, and updatedAt fields for validation.

**Module Context:**
- **Hierarchy:** Data validation tool (executable package)
- **Responsibility:** Format verification of migration dump files
- **Data Source:** SQLite dump files from queue-api
- **Entry Point Type:** SQL INSERT statement parsing for format validation

---

### cmd/load-email-resolution-from-queue-api (parseDump, parseValuesString, parseTime, parseTimePtr)

**Package:** `cmd/load-email-resolution-from-queue-api`  
**File:** `cmd/load-email-resolution-from-queue-api/main.go`  
**Functions:**
- `parseDump(dump string) ([]QueueAPIRow, error)` :163
- `parseValuesString(valuesStr string) (QueueAPIRow, error)` :197
- `parseTime(s string) (time.Time, error)` :290
- `parseTimePtr(s string) *time.Time` :312

**Package Role:**
One-off migration script that loads SQLite queue-api dump into PostgreSQL using the identity ingest endpoint with source='live' for resolved entries only.

**Module Context:**
- **Hierarchy:** Data migration tool (executable package)
- **Responsibility:** Legacy data migration from SQLite to PostgreSQL
- **Data Flow:** SQLite dump → identity ingest endpoint → PostgreSQL
- **Filtering:** Only loads resolved entries (status='resolved' with non-NULL github_login)
- **Entry Point Type:** Multi-level SQL dump parsing (dump → values → timestamps)

---

### cmd/get-audit-logs (parseDate)

**Package:** `cmd/get-audit-logs`  
**File:** `cmd/get-audit-logs/main.go:216`  
**Function:** `parseDate(dateStr string) (time.Time, error)`

**Package Role:**
Audit log retrieval CLI tool (separate from audit-logs command). Fetches audit logs from the database with similar filtering capabilities.

**Module Context:**
- **Hierarchy:** Command-line tool (executable package)
- **Responsibility:** Audit log data retrieval and presentation
- **Difference from audit-logs:** Separate implementation with similar functionality
- **Entry Point Type:** Date parsing for command-line argument validation

---

### pkg/warmstart (parseConfigKey)

**Package:** `pkg/warmstart`  
**File:** `pkg/warmstart/extract.go:441`  
**Function:** `parseConfigKey(key string) (string, string)`

**Package Role:**
Git repository warm-start snapshot materialization. Enables incremental fetch by restoring repository state from previous scans, avoiding full re-clones.

**Module Context:**
- **Hierarchy:** Library package (reusable component)
- **Responsibility:** Git repository warm-start artifact management
- **Artifact Contents:** Pack files, loose refs, and git config for partial clone support
- **Entry Point Type:** Git config key parsing (section + key extraction)
- **Validation:** Empirical validation in docs/research/incremental-fetch-warm-start.md

---

### pkg/handler (parseQueryParams, parseDate)

**Package:** `pkg/handler`  
**File:** `pkg/handler/audit_logs.go`  
**Functions:**
- `parseQueryParams(r *http.Request) (queryParams, error)` :107
- `parseDate(dateStr string) (*time.Time, error)` :174

**Package Role:**
HTTP request handlers for the audit log query API. Provides REST endpoints for audit log queries with parameter validation and parsing.

**Module Context:**
- **Hierarchy:** Library package (HTTP handler layer)
- **Responsibility:** HTTP request processing and response generation
- **Dependencies:** `pkg/service` for database queries
- **API Contract:** Defines queryParams struct for validated request parameters
- **Entry Point Type:** HTTP request parameter parsing and validation

---

## Stream Entry Points

### pkg/ingestlog (CaptureUserContext, CaptureUserID, CaptureSessionID, CaptureRequestID, CaptureEndpointName, CaptureMethod, CapturePath, CaptureEndpointContext)

**Package:** `pkg/ingestlog`  
**File:** `pkg/ingestlog/logger.go`  
**Functions:**
- `CaptureUserContext(email, githubUsername string) (UserContext, error)` :910
- `CaptureUserID(userID string) string` :934
- `CaptureSessionID(sessionID string) string` :950
- `CaptureRequestID(requestID string) string` :966
- `CaptureEndpointName(endpoint string) (string, error)` :983
- `CaptureMethod(method string) (string, error)` :1000
- `CapturePath(path string) (string, error)` :1017
- `CaptureEndpointContext(endpoint, method, path, url string, attemptNumber int, statusCode int, responseBody string) (EndpointContext, error)` :1041

**Package Role:**
Structured logging for ingest endpoint operations. Provides comprehensive logging and validation for all ingest operations with aggregate statistics tracking.

**Module Context:**
- **Hierarchy:** Library package (logging infrastructure)
- **Responsibility:** Ingest operation telemetry and validation
- **Data Flow:** Capture → Validate → Log → Aggregate Statistics
- **Features:** Thread-safe logging, error tracking, performance metrics
- **Entry Point Type:** Data capture and validation functions for stream processing

---

### pkg/identity (IngestResolution)

**Package:** `pkg/identity`  
**File:** `pkg/identity/ingest.go:131`  
**Function:** `(i *Ingester) IngestResolution(ctx context.Context, rows []ResolutionRow) error`

**Package Role:**
Bulk identity resolution ingest functionality. The single way all writers (live enrichment worker, claude-leaderboard seed, manual curation) write email→login resolutions to the email_resolution table.

**Module Context:**
- **Hierarchy:** Library package (business logic layer)
- **Responsibility:** Identity resolution conflict resolution and persistence
- **Conflict Resolution:** Enforces consistent conflict resolution across all sources
- **Data Flow:** ResolutionRow[] → validation → database upsert
- **Sources:** live (enrichment worker), seed (claude-leaderboard), manual (curation)
- **Entry Point Type:** Bulk data ingestion with conflict resolution

---

### pkg/pg (IngestEmailResolution, UpsertAliases)

**Package:** `pkg/pg`  
**File:** `pkg/pg/identity.go:94`, `pkg/pg/user_aliases.go:46`  
**Functions:**
- `IngestEmailResolution(ctx context.Context, rows []identity.ResolutionRow) (*identity.IngestResult, error)` :94
- `UpsertAliases(ctx context.Context, rows []AliasRow) error` :46

**Package Role:**
PostgreSQL implementations for commitgraph data access. Provides concrete database operations for identity resolution and user alias management.

**Module Context:**
- **Hierarchy:** Library package (data access layer)
- **Responsibility:** PostgreSQL-specific database operations
- **Operations:** Bulk upsert with conflict resolution
- **Interface:** Implements identity.DB interface for dependency injection
- **Entry Point Type:** Database bulk operations for stream processing

---

## Package Classification Matrix

| Package | Type | Layer | Data Flow | Entry Point Role |
|---------|------|-------|-----------|------------------|
| cmd/audit-logs | Executable | CLI | Database → CLI output | User interface |
| cmd/load-admin-aliases | Executable | Migration | ConfigMap → Database | Data migration |
| cmd/verify-email-resolution-dump | Executable | Validation | Dump → Validation | Data verification |
| cmd/load-email-resolution-from-queue-api | Executable | Migration | SQLite → PostgreSQL | Data migration |
| cmd/get-audit-logs | Executable | CLI | Database → CLI output | User interface |
| pkg/warmstart | Library | Infrastructure | Git artifact → State | Git operations |
| pkg/handler | Library | HTTP API | HTTP Request → Response | API gateway |
| pkg/ingestlog | Library | Telemetry | Operations → Logs | Observability |
| pkg/identity | Library | Business Logic | Resolution → Database | Data coordination |
| pkg/pg | Library | Data Access | Operations → Database | Persistence |

## Module Responsibility Categories

### 1. User Interface Layer (cmd/*)
- **Purpose:** Direct user interaction via CLI tools
- **Responsibilities:** Argument parsing, validation, output formatting
- **Entry Points:** parseDate, parseAliasesFromConfigMap, parseInsertLine, parseDump family

### 2. API Gateway Layer (pkg/handler)
- **Purpose:** HTTP request handling and routing
- **Responsibilities:** Parameter extraction, validation, response generation
- **Entry Points:** parseQueryParams, parseDate

### 3. Business Logic Layer (pkg/identity, pkg/warmstart)
- **Purpose:** Core application logic and coordination
- **Responsibilities:** Data validation, conflict resolution, workflow orchestration
- **Entry Points:** IngestResolution, parseConfigKey

### 4. Data Access Layer (pkg/pg)
- **Purpose:** Database operations and persistence
- **Responsibilities:** SQL execution, transaction management, bulk operations
- **Entry Points:** IngestEmailResolution, UpsertAliases

### 5. Observability Layer (pkg/ingestlog)
- **Purpose:** Telemetry and monitoring
- **Responsibilities:** Structured logging, metrics collection, validation
- **Entry Points:** All Capture* functions

## Dependency Relationships

```
CLI Tools (cmd/*) → Business Logic (pkg/identity, pkg/warmstart)
                   → Data Access (pkg/pg)
                   → Observability (pkg/ingestlog)

HTTP API (pkg/handler) → Business Logic (pkg/identity)
                      → Data Access (pkg/pg)
                      → Observability (pkg/ingestlog)
```

## Entry Point Naming Patterns

### Parse Functions
- **Pattern:** `parse[DataType](input) (output, error)`
- **Purpose:** Parse and validate input data
- **Locations:** CLI tools, HTTP handlers
- **Return Pattern:** Output + error for validation feedback

### Capture Functions (Stream)
- **Pattern:** `Capture[DataType](input) (output, error)`
- **Purpose:** Extract and validate streaming data
- **Locations:** pkg/ingestlog
- **Return Pattern:** Validated data + error for logging

### Ingest Functions (Stream)
- **Pattern:** `(receiver) Ingest[DataType](ctx, rows) (result, error)`
- **Purpose:** Bulk data ingestion with conflict resolution
- **Locations:** pkg/identity, pkg/pg
- **Return Pattern:** Ingest result statistics + error

## Validation Patterns

### Entry Point Validation
1. **Input Validation:** Type checking, format validation, range checking
2. **Business Rules:** Conflict resolution, source validation, timestamp ordering
3. **Data Integrity:** Foreign key constraints, uniqueness, referential integrity
4. **Error Propagation:** Structured error types, context preservation

### Stream vs. Parse Entry Points
- **Parse:** Single-item validation, immediate feedback, CLI/HTTP context
- **Stream:** Bulk validation, aggregate results, background processing context

## Architecture Context

This entry point structure reflects the system's design principles:

1. **Separation of Concerns:** Clear boundaries between CLI, API, business logic, and data access layers
2. **Single Responsibility:** Each package has a well-defined role in the data flow
3. **Dependency Injection:** Interface-based design allows testing and flexibility
4. **Stream Processing:** Bulk operations support high-throughput data ingestion
5. **Validation at Edges:** Input validation happens at entry points, protecting core logic

## Related Documentation

- `docs/plan/plan.md` - Complete architecture and rollout plan
- `docs/notes/` - Architecture decisions and rationale
- `docs/research/incremental-fetch-warm-start.md` - Warm-start empirical validation
- `parse_entry_point_signatures.json` - Detailed function signatures
- `stream_entry_points.json` - Stream entry point catalog
