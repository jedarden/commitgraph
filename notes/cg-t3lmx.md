# Log Entry Schema Definition - cg-t3lmx

## Task Completed

Successfully defined and verified the data structures for log entries used by the ingest error logging system.

## What Was Done

### 1. Existing Structures Verified
The following structures were already well-defined in `pkg/ingestlog/logger.go`:

- **LogEntry** (lines 54-63): Main struct containing all log event data
- **ErrorContext** (lines 33-37): Error details with type, message, and stack trace
- **UserContext** (lines 40-43): User identification with email and GitHub username
- **EndpointContext** (lines 46-51): HTTP endpoint interaction details

### 2. Missing Implementation Added
Added the `classifyError` function (lines ~294-330) which was being called but not implemented:
- Classifies errors into: timeout, network, parse_error, client_error, server_error, unknown, or empty (for nil)
- Analyzes both error message content and HTTP status codes
- Essential for proper error categorization in log entries

## Acceptance Criteria Met

✅ **LogEntry struct** defined with all required fields  
✅ **ErrorContext struct** includes type, message, and stack_trace fields  
✅ **UserContext struct** includes email and githubUsername fields  
✅ **EndpointContext struct** includes url, attempt_number, status_code, and response_body fields  
✅ **All structs** have proper JSON tags for serialization  

## Schema Structure

```go
type LogEntry struct {
    Timestamp       time.Time       `json:"timestamp"`
    EventType       string          `json:"event_type"`      // "retry", "failure", or "success"
    User            UserContext     `json:"user"`
    Endpoint        EndpointContext `json:"endpoint"`
    Error           ErrorContext    `json:"error,omitempty"`
    MaxRetries      int             `json:"max_retries"`
    RetryDelayMs    int             `json:"retry_delay_ms,omitempty"`
    TotalDurationMs int64           `json:"total_duration_ms,omitempty"`
}

type ErrorContext struct {
    Type       string `json:"type"`                  // error classification
    Message    string `json:"message"`               // human-readable error
    StackTrace string `json:"stack_trace,omitempty"` // debug stack trace
}

type UserContext struct {
    Email          string `json:"email"`            // user's email
    GithubUsername string `json:"github_username"`  // target GitHub username
}

type EndpointContext struct {
    URL           string `json:"url"`                           // full endpoint URL
    AttemptNumber int    `json:"attempt_number"`                 // current retry attempt
    StatusCode    int    `json:"status_code,omitempty"`          // HTTP status code
    ResponseBody  string `json:"response_body,omitempty"`        // response content
}
```

## JSON Serialization Example

```json
{
  "timestamp": "2026-08-06T12:00:00Z",
  "event_type": "retry",
  "user": {
    "email": "test@example.com",
    "github_username": "testuser"
  },
  "endpoint": {
    "url": "http://test:8080/resolve",
    "attempt_number": 1,
    "status_code": 429,
    "response_body": "{\"error\": \"rate limit\"}"
  },
  "error": {
    "type": "client_error",
    "message": "rate limit exceeded",
    "stack_trace": "stacktrace here"
  },
  "max_retries": 4,
  "retry_delay_ms": 1000,
  "total_duration_ms": 1500
}
```

## Files Modified

- `pkg/ingestlog/logger.go`: Added `classifyError`, `contains`, and `containsSubstring` functions
- `pkg/ingestlog/logger_test.go`: Added missing `encoding/json` import and fixed function call signatures

## Verification

- Code compiles successfully
- JSON serialization tested and working correctly
- All struct fields properly tagged for JSON marshaling
- Error classification logic tested with all expected categories
