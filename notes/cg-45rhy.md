# Parsing Error Catalog (cg-45rhy)

## Summary

Completed comprehensive catalog of all parsing error locations in the commitgraph codebase.

## Findings

- **Total parsing error sites**: 27
- **With commit SHA context**: 0 (0%)
- **With position context**: 1 (4%)
- **With any domain context**: 2 (8%)
- **Without context**: 25 (92%)

## Categories Documented

1. **Timestamp/Date Parsing Errors** (16 sites)
   - Command-line date parsing in audit log tools
   - SQLite timestamp parsing
   - Missing line numbers and record identifiers

2. **SQLite Dump Parsing Errors** (5 sites)
   - Email resolution dump ingestion
   - Missing row numbers and position context
   - No traceability for failed records

3. **YAML/Config Parsing Errors** (2 sites)
   - Admin alias ConfigMap parsing
   - Missing YAML line numbers and file positions

4. **JSON Parsing Errors** (1 site)
   - Warmstart snapshot config parsing
   - Missing byte offset and context

5. **Integer Parsing Errors** (3 sites)
   - Query parameter validation
   - Missing parameter context and source

## Output

Created comprehensive catalog: `docs/parsing-error-catalog.md`

The catalog includes:
- Exact file paths and line numbers for all 27 error sites
- Code context showing the error generation
- Categorization by context availability
- Prioritized recommendations for remediation
- Analysis of infrastructure already available (pkg/errors)

## Recommendations

Priority 1: Add context to high-volume parsing errors (SQLite dump, YAML configs)
Priority 2: Standardize error context across all parsing operations
Priority 3: Use structured error types from pkg/errors instead of fmt.Errorf

## Impact

Parsing errors without context prevent:
- Identification of which record/row failed
- Automated retry logic for specific records
- Effective debugging and log correlation
- Tracking error patterns over time
