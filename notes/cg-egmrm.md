# Parsing Error Type Definitions Discovery

## Task: cg-egmrm - Find all parsing error type definitions

### Summary
Successfully discovered and documented 10 distinct parsing error type definitions across 3 packages in the commitgraph codebase.

### Key Findings

**Primary Error Types**:
1. `pkg/errors.StructuredError` - Main structured error type with comprehensive metadata
2. `pkg/errors.ErrorCategory` - String-based category system with `ParseError = "parse_error"`
3. `pkg/ingestlog.ErrorContext` - Ingest-specific error context for logging
4. `pkg/warmstart.Error` - Tarball operation error with kind classification

**Constructor Functions**:
- `ParseErrorf()` - General parse error constructor (pkg/errors/helpers.go:48)
- `JSONParseError()` - JSON-specific parse errors (pkg/errors/helpers.go:74)
- 9 additional constructor functions for warmstart operations

**Error Classification Logic**:
Both `pkg/errors` and `pkg/ingestlog` implement pattern matching for parse error detection:
- Keywords: "invalid JSON", "cannot unmarshal", "invalid character", "unmarshal", "parse error"
- Severity: Automatically set to `SeverityHigh`
- Retry policy: Parse errors are **non-retryable** by default

**Notable Discoveries**:
- Two different `ErrorContext` types exist (pkg/errors vs pkg/ingestlog) with different purposes
- Two deprecated error types in pkg/warmstart (CorruptionError, NotAGitRepoError)
- String-based enums (ErrorCategory) vs integer enums (ErrorKind)
- Comprehensive error recovery suggestions in pkg/ingestlog

### Files Analyzed
- pkg/errors/types.go (139 lines of error definitions)
- pkg/errors/helpers.go (468 lines of constructor functions)
- pkg/ingestlog/logger.go (1331 lines with error classification)
- pkg/warmstart/error.go (238 lines of warmstart-specific errors)

### Deliverables
- `/tmp/error-types-found.txt` - Comprehensive documentation with:
  - All 10 error type definitions with locations
  - Constructor signatures
  - Error classification logic
  - Usage patterns and examples
  - File location reference table

### Next Steps
This discovery phase provides the foundation for:
1. Creating comprehensive error handling tests
2. Standardizing error type usage across packages
3. Building error type conversion utilities
4. Documenting error patterns for developers
