# cg-1ie77: Endpoint Context Capture in LogIngestError

## Task
Wire up endpoint context capture (endpoint, method, path) in LogIngestError using the imported helper functions.

## Implementation Status: ✅ COMPLETE

The endpoint context capture is fully implemented in `pkg/ingestlog/logger.go` in the `LogIngestError` function (lines 1010-1029):

### Helper Functions Called
1. **Line 1011**: `CaptureEndpointName(endpoint)` - Captures endpoint identifier
2. **Line 1017**: `CaptureMethod(method)` - Captures HTTP method
3. **Line 1023**: `CapturePath(path)` - Captures request path
4. **Line 1029**: `CaptureEndpointContext(...)` - Combines all endpoint data into EndpointContext struct

### Data Storage
The captured endpoint context is stored in the LogEntry struct at line 1042:
```go
entry := &LogEntry{
    // ...
    Endpoint:  endpointCtx,  // Contains endpoint, method, path, URL, etc.
    // ...
}
```

## Acceptance Criteria Verification

✅ **Endpoint context capture functions are called in LogIngestError**
- All four helper functions are called with proper error handling

✅ **endpoint is captured and stored in LogEntry**
- Test JSON output shows: `"endpoint":"github-username-resolution"`

✅ **method is captured and stored in LogEntry**
- Test JSON output shows: `"method":"POST"`

✅ **path is captured and stored in LogEntry**
- Test JSON output shows: `"path":"/email-resolution/resolve"`

✅ **Context is preserved through the error flow**
- Both retry and failure events preserve full endpoint context
- User context (user_id, session_id, request_id) also preserved

## Test Coverage

Two comprehensive test suites verify the implementation:

1. **TestLogIngestError_EndpointContextCapture** - Tests endpoint, method, path capture
2. **TestLogIngestError_EndpointContextIntegration** - Tests full integration with error flow

All tests pass successfully.

## Example Log Output

```json
{
  "endpoint": {
    "endpoint": "github-username-resolution",
    "method": "POST",
    "path": "/email-resolution/resolve",
    "url": "http://queue-api:8080/email-resolution/resolve",
    "attempt_number": 1,
    "status_code": 503,
    "response_body": "{\"error\": \"service unavailable\"}"
  }
}
```

## Completion Date
2026-08-06

## Notes
This implementation ensures complete endpoint metadata is captured for all ingest operations, enabling better debugging, monitoring, and analysis of endpoint interactions in the commitgraph system.
