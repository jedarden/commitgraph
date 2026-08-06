# cg-51x4u: LogIngestError Function Signature

## Status: COMPLETED

This work was completed in commit `3424f19` on 2026-08-06.

## Implementation Summary

The LogIngestErrorExtended function was implemented in `pkg/ingestlog/logger.go` with the following components:

### New Data Structures
- **RequestMetadata**: `map[string]interface{}` for optional metadata
- **ExtendedUserContext**: Struct with UserID, SessionID, RequestID, Email, Username
- **ExtendedEndpointContext**: Struct with Endpoint, Method, Path, URL, StatusCode, ResponseBody

### Function Signature
```go
func LogIngestErrorExtended(logger *Logger, err error, userCtx ExtendedUserContext, endpointCtx ExtendedEndpointContext, metadata RequestMetadata) error
```

### Acceptance Criteria Met
✅ Function signature defined with all required parameters  
✅ Doc comment explains purpose and all parameters  
✅ Basic scaffolding with TODO markers for child bead integrations  
✅ Function returns error for proper error handling propagation  

### Integration Points (TODOs)
- cg-2iff2: Error serialization and type classification
- cg-4zz54: User and endpoint context capture helpers
- Future: Integration with monitoring and alerting systems
- Future: Integration with distributed tracing systems

### Key Features
- Accepts error interface (nil-safe)
- Validates required user context fields (UserID, SessionID, RequestID)
- Validates required endpoint context fields (Endpoint, Method, Path)
- Integrates with existing SerializeError function from error_serializer.go
- Returns properly wrapped errors for chaining
- Handles nil logger by using default logger instance

The function maintains backward compatibility with the existing LogIngestError function while providing enhanced context capabilities for future extensibility.
