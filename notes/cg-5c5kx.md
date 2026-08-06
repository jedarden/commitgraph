# Analysis of Errors Lacking Commit SHA Parameters (cg-5c5kx)

## Task Completion Summary

This analysis identified all error constructors and call sites in the commitgraph codebase that lack commit SHA parameters, which is critical context for debugging and monitoring during git operations, commit parsing, and data processing.

## Key Findings

### Critical Discovery
**ALL error constructors lack commit SHA as a direct parameter**, despite the StructuredError type having a CommitSHA field and a WithCommitSHA() method.

### Statistics
- **Total error constructors analyzed**: 35+
- **Constructors with commit SHA parameter**: 0
- **High-priority errors needing SHA**: ~15 (parsing, git operations, database)
- **Medium-priority errors**: ~10 (network, HTTP operations)
- **Low-priority errors**: ~10 (infrastructure, configuration)

## Error Constructors Catalog

### pkg/errors/helpers.go (29 constructors - ALL lack SHA)
All helper functions create StructuredError instances but don't accept commit SHA parameter:
- ValidationErrorf, RequiredFieldError, InvalidFormatError
- ParseErrorf, JSONParseError
- DatabaseErrorf, DatabaseConnectionError, DatabaseQueryError
- NetworkErrorf, ConnectionRefusedError, DNSError
- TimeoutErrorf, HTTPTimeoutError, DatabaseTimeoutError
- HTTPError, AuthErrorf, UnauthorizedError, ForbiddenError
- ConfigErrorf, MissingConfigError, InvalidConfigError
- ResourceErrorf, MemoryExhaustedError, DiskSpaceExhaustedError
- ConnectionPoolExhaustedError, ConcurrencyErrorf
- DeadlockError, LockConflictError

### pkg/errors/types.go (3 constructors - lack direct SHA)
- NewError() - accepts WithCommitSHAOption but not direct parameter
- WrapError() - no commit SHA parameter
- ClassifyError() - no commit SHA parameter

### pkg/warmstart/error.go (6 constructors - ALL lack SHA)
- NewIOError, NewTruncatedError, NewTruncatedMemberError
- NewMissingMemberError, NewMissingMemberErrorWithContext
- NewCorruptPackError

## High-Value Targets for Improvement

### 1. Git Operations (warmstart package)
- **VerifyGitLog()** - Should include SHA being verified
- **ParseTarball()** - RefSHA extracted but not in error context
- **Git fsck operations** - Errors could include object SHA

### 2. Parsing Operations
- **ParseErrorf()** - Commit data parsing failures
- **JSONParseError()** - JSON structure parsing
- **ValidationErrorf()** - Commit data validation

### 3. Database Operations
- **DatabaseErrorf()** - Commit record storage failures
- **DatabaseQueryError()** - Commit data query failures

## Current State Analysis

1. **Error infrastructure exists** - StructuredError.CommitSHA field and WithCommitSHA() method available
2. **Helper functions defined** - 29+ helpers for creating structured errors
3. **Limited adoption** - Most code still uses basic `fmt.Errorf()`
4. **Missing integration** - Even helpers don't accept SHA as parameter
5. **No call site pattern** - No clear pattern for including SHA context

## Recommended Implementation Strategy

### Phase 1: High-Value Targets
Modify warmstart and parsing error constructors to accept commit SHA:
```go
// Current
err := ParseErrorf("component", "operation", "dataType", "message")
err.WithCommitSHA(sha)  // Separate call

// Proposed
err := ParseErrorf("component", "operation", "dataType", sha, "message")
```

### Phase 2: Database/Network Errors
Add optional commit SHA parameters to relevant constructors

### Phase 3: Backward Compatibility
Maintain existing constructors, add SHA-aware variants, document migration path

## Files Created
- `/tmp/missing-sha-analysis.txt` - Detailed analysis document

## Conclusion

The analysis revealed a systematic gap where commit SHA context is missing from error constructors despite the infrastructure supporting it. This limits debugging capabilities during git operations, commit parsing, and data processing workflows.

