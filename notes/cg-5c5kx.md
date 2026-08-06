# cg-5c5kx: Catalog errors lacking commit SHA parameters

## Task Completed

Analyzed all error constructors and call sites in the commitgraph codebase to identify which errors currently lack the commit SHA parameter.

## Key Findings

### All 29 Error Constructors Missing Commit SHA

Located in `/home/coding/commitgraph/pkg/errors/helpers.go`, ALL error helper functions lack commit SHA parameters:

1. ValidationErrorf
2. RequiredFieldError  
3. InvalidFormatError
4. ParseErrorf
5. JSONParseError
6. DatabaseErrorf
7. DatabaseConnectionError
8. DatabaseQueryError
9. NetworkErrorf
10. ConnectionRefusedError
11. DNSError
12. TimeoutErrorf
13. HTTPTimeoutError
14. DatabaseTimeoutError
15. HTTPError
16. AuthErrorf
17. UnauthorizedError
18. ForbiddenError
19. TokenExpiredError
20. ConfigErrorf
21. MissingConfigError
22. InvalidConfigError
23. ResourceErrorf
24. MemoryExhaustedError
25. DiskSpaceExhaustedError
26. ConnectionPoolExhaustedError
27. ConcurrencyErrorf
28. DeadlockError
29. LockConflictError

### Infrastructure Exists But Not Used

The `StructuredError` type has:
- `CommitSHA` field (line 125 in types.go)
- `WithCommitSHA()` method (lines 389-396)
- `WithCommitSHAOption()` function (lines 225-229)

However, NONE of the error constructors accept commit SHA as a parameter.

### Current Usage Patterns

- Production code primarily uses `fmt.Errorf()`, not structured error constructors
- Structured error constructors are only used internally within helpers.go
- Only test files currently use `WithCommitSHA` method
- 17 internal call sites need updating when constructors are enhanced

## Analysis Output

Detailed analysis saved to: `/tmp/missing-sha-analysis.txt`

## Next Steps

1. Add commit SHA parameter to all 29 error constructors
2. Update 17 internal call sites within helpers.go  
3. Consider adding convenience methods for chaining
4. Update documentation and examples

## Impact

This is a greenfield opportunity - structured errors aren't used in production yet, so implementing commit SHA support correctly from the start will enable better error tracking when the system is adopted.
