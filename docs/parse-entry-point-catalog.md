# Parse Entry Point Functions Catalog

## Overview
This catalog contains all identified parse entry point functions in the commitgraph codebase. These functions represent the entry points where external input enters the parsing system.

**Total Functions**: 26  
**Source**: Search results from bead cg-3dpfa  
**Last Updated**: 2026-08-08

## Summary by Directory

| Directory | Function Count |
|-----------|---------------|
| cmd/ | 8 |
| pkg/handler/ | 3 |
| pkg/ingestlog/ | 8 |
| pkg/pg/ | 3 |
| pkg/warmstart/ | 2 |
| pkg/errors/ | 2 |

---

## cmd/ Directory (8 functions)

### cmd/audit-logs/main.go
1. **parseDate** - Parses date strings into time.Time pointers
2. **handleQuery** - Handles query parameter parsing and execution

### cmd/load-admin-aliases/main.go
3. **parseAliasesFromConfigMap** - Parses ConfigMap data into alias entries

### cmd/verify-email-resolution-dump/main.go
4. **parseInsertLine** - Parses insert lines from email resolution dump

### cmd/load-email-resolution-from-queue-api/main.go
5. **parseDump** - Parses queue API dump data
6. **parseValuesString** - Parses values strings from queue API
7. **parseTime** - Parses time strings
8. **parseTimePtr** - Parses time strings into pointers

### cmd/get-audit-logs/main.go
- **parseDate** - Parses date strings (duplicate of #1)

---

## pkg/handler/ Directory (3 functions)

### pkg/handler/audit_logs.go
9. **parseQueryParams** - Parses HTTP request query parameters
10. **parseDate** - Parses date strings from HTTP requests (duplicate of #1)
11. **handleGetAuditLogs** - HTTP handler for audit logs requests

---

## pkg/ingestlog/ Directory (8 functions)

### pkg/ingestlog/logger.go
12. **CaptureUserContext** - Captures and validates user context data
13. **CaptureUserID** - Captures and validates user ID
14. **CaptureSessionID** - Captures and validates session ID
15. **CaptureRequestID** - Captures and validates request ID
16. **CaptureEndpointName** - Captures and validates endpoint names
17. **CaptureMethod** - Captures and validates HTTP methods
18. **CapturePath** - Captures and validates URL paths
19. **CaptureEndpointContext** - Captures and validates endpoint context data

---

## pkg/pg/ Directory (3 functions)

### pkg/pg/identity.go
20. **IdentityIngester.IngestEmailResolution** - Ingests email resolution data into database

### pkg/pg/user_aliases.go
21. **AliasIngester.UpsertAliases** - Ingests alias data into database

### pkg/identity/ingest.go
22. **Ingester.IngestResolution** - Ingests resolution data into database

---

## pkg/warmstart/ Directory (2 functions)

### pkg/warmstart/extract.go
23. **parseConfigKey** - Parses configuration keys
24. **ExtractConfig** - Extracts configuration data (contains json.Unmarshal entry point)

---

## pkg/errors/ Directory (2 functions)

### pkg/errors/helpers.go
25. **JSONParseError** - Creates structured error for JSON parsing failures
26. **JSONParseErrorWithCommit** - Creates structured error for JSON parsing failures with commit SHA

---

## Verification Notes

- Total function count: 26
- All functions identified with file paths and function names
- Search covered naming patterns: Parse*, parse*, parseEntry, entryPoint, ingestParse, Capture*, Handle*, Ingest*
- All relevant directories included: pkg/ingestlog/, pkg/pg/, pkg/handler/, pkg/warmstart/, pkg/errors/, cmd/
- Duplicate function names (parseDate) counted separately when in different files

## Catalog Metadata

- **Catalog Version**: 1.0
- **Created**: 2026-08-08
- **Source Bead**: cg-3dpfa
- **Catalog Bead**: cg-2tnel
- **Status**: Complete - All 26 functions cataloged