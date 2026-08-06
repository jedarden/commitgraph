# Error Instantiation Call Sites Catalog (Bead cg-5l90t)

## Task Completed
Cataloged all error instantiation call sites across the commitgraph codebase.

## Summary of Findings

### Total Error Instantiations Found
- **errors.New**: 6 call sites (all in warmstart package for sentinel errors)
- **fmt.Errorf**: 229+ call sites (dominant pattern throughout codebase)
- **warmstart.Error constructors**: 11 call sites in production code
- **pkg/errors helper functions**: 0 call sites in production (definitions only)
- **Direct struct instantiation**: 17 call sites (in helpers.go and error.go)

### Key Error Types

1. **Standard Library Errors** (errors.New, fmt.Errorf)
   - errors.New: 6 sentinel errors in pkg/warmstart/extract.go
   - fmt.Errorf: 229+ instances with %w wrapping for error chain preservation

2. **warmstart.Error Type**
   - Custom error type for tarball operations
   - 6 constructor functions: NewIOError, NewTruncatedError, NewTruncatedMemberError, NewMissingMemberError, NewMissingMemberErrorWithContext, NewCorruptPackError
   - Rich context: member names, byte offsets, corruption details

3. **pkg/errors.StructuredError**
   - Comprehensive error framework with 25 helper functions
   - Categories: ValidationError, ParseError, DatabaseError, NetworkError, TimeoutError, ClientError, ServerError, AuthError, ConfigError, ResourceError, ConcurrencyError
   - **Not currently used in production code** (only in tests)

### Error Construction Patterns

1. **Simple Error Creation** (errors.New)
   - Used for sentinel error values
   - Locations: pkg/warmstart/extract.go (6 instances)

2. **Wrapped Error Creation** (fmt.Errorf with %w)
   - Most common pattern (200+ instances)
   - Preserves error chains via Go's error wrapping
   - Locations: pkg/pg, pkg/service, pkg/identity, pkg/ingestlog

3. **Formatted Error Creation** (fmt.Errorf without %w)
   - Used for validation errors
   - Locations: pkg/handler, pkg/service validation functions

4. **Domain-Specific Constructors** (warmstart.Error)
   - Provides structured error information
   - Used exclusively in tarball extraction logic

### Key Observations

1. **Primary Pattern**: fmt.Errorf with %w wrapping is the dominant pattern
2. **Domain-Specific**: warmstart package has custom error type with rich context
3. **Unused Infrastructure**: pkg/errors package is comprehensive but not adopted
4. **Error Consistency**: Different layers use different patterns
5. **Error Context**: Most errors include contextual information

### Distribution by Package

- **pkg/warmstart/extract.go**: 17 error instantiations
- **pkg/handler/audit_logs.go**: 14 validation errors
- **pkg/service/exclusion.go**: 10+ validation errors
- **pkg/pg/*.go**: 30+ database errors
- **pkg/service/audit_query.go**: 8+ database query errors
- **pkg/identity/*.go**: 10+ errors
- **pkg/ingestlog/logger.go**: 4+ serialization errors

## Detailed Catalog
The complete catalog with all 270 lines of detailed call site information is saved in:
`/tmp/call-sites-found.txt`

## Next Steps / Opportunities

1. **Migration Opportunity**: Consider migrating from fmt.Errorf to pkg/errors helper functions
2. **Error Standardization**: Different packages could benefit from consistent error patterns
3. **Structured Error Adoption**: pkg/errors framework is ready for adoption but unused
4. **Error Context Enhancement**: warmstart.Error pattern could be applied to other domains

## Files Analyzed
- 23 production Go files in pkg/
- Multiple cmd/ directories
- Total codebase: 235+ error instantiation call sites documented
