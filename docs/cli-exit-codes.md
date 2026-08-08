# CLI Exit Codes

This document describes the exit code conventions used across commitgraph CLI tools.

## Overview

All commitgraph CLI commands use structured exit codes that follow BSD `sysexits.h` conventions where applicable. Exit codes are automatically mapped from error types using the `pkg/errors` package.

## Exit Code Reference

| Code | Constant | Description | Error Category | Retryable |
|------|----------|-------------|----------------|-----------|
| 0 | `ExitSuccess` | Success (no error) | N/A | N/A |
| 1 | `ExitError` | General error (uncategorized) | `UnknownError` | No |
| 2 | `ExitUsage` | Command line usage error | `ValidationError`, `ClientError` | No |
| 65 | `ExitDataError` | Data format error | `ParseError` | No |
| 66 | `ExitNoInput` | No input / insufficient data | N/A | No |
| 67 | `ExitNoUser` | User不存在 (for reference) | N/A | No |
| 68 | `ExitNoHost` | Host unknown (for reference) | N/A | No |
| 69 | `ExitUnavailable` | Service unavailable | `DatabaseError`, `ServerError` | Yes |
| 70 | `ExitSoftware` | Internal software error | `ConcurrencyError` | No |
| 71 | `ExitOSError` | System error (resource exhausted) | `ResourceError` | No |
| 74 | `ExitIOError` | I/O error | N/A | No |
| 75 | `ExitTempFail` | Temporary failure (retry may succeed) | `NetworkError`, `TimeoutError` | Yes |
| 76 | `ExitProtocol` | Protocol error | N/A | No |
| 77 | `ExitPermission` | Permission denied | `AuthError` | No |
| 78 | `ExitConfig` | Configuration error | `ConfigError` | No |

## Error Category to Exit Code Mapping

The `pkg/errors.GetExitCode()` function automatically maps error categories to appropriate exit codes:

### Exit Code 2 (Usage Error)
- **ValidationError**: Input validation failures, missing required fields
- **ClientError**: HTTP 4xx client errors (typically parameter issues)

### Exit Code 65 (Data Error)
- **ParseError**: Data parsing failures (JSON, XML, binary formats)

### Exit Code 69 (Unavailable)
- **DatabaseError**: Database operation failures
- **ServerError**: HTTP 5xx server errors

### Exit Code 70 (Software Error)
- **ConcurrencyError**: Deadlocks, race conditions, locking issues

### Exit Code 71 (OS Error)
- **ResourceError**: Out of memory, disk full, resource exhaustion

### Exit Code 75 (Temporary Failure)
- **NetworkError**: Connection refused, network unreachable
- **TimeoutError**: Deadlines exceeded, operation timeouts

### Exit Code 77 (Permission)
- **AuthError**: Authentication/authorization failures

### Exit Code 78 (Configuration)
- **ConfigError**: Configuration problems, missing environment variables

### Exit Code 1 (General Error)
- **UnknownError**: Uncategorized errors that don't fit other categories

## Using Exit Codes in CLI Commands

### Basic Pattern

```go
import (
    "github.com/jedarden/commitgraph/pkg/errors"
)

func main() {
    // ... command logic ...

    if err != nil {
        errors.ExitWithCode(err)  // Automatically determines correct exit code
    }
}
```

### Working with Structured Errors

```go
import (
    "github.com/jedarden/commitgraph/pkg/errors"
)

func processData() error {
    if validationFailed {
        return errors.NewError(
            errors.ValidationError,
            errors.SeverityHigh,
            "required field 'email' is missing",
            "MISSING_FIELD",
            "data-processor",
            "process-record",
        )
    }
    return nil
}

func main() {
    if err := processData(); err != nil {
        errors.ExitWithCode(err)  // Exits with code 2 (ValidationError)
    }
}
```

### Checking Exit Code Properties

```go
exitCode := errors.GetExitCode(err)

if errors.IsSuccess(exitCode) {
    // Command succeeded
}

if errors.IsTemporary(exitCode) {
    // Retry with backoff
}

if errors.IsUsageError(exitCode) {
    // Show usage message
}

if errors.IsPermissionError(exitCode) {
    // Check credentials/permissions
}
```

### Getting Exit Code Descriptions

```go
exitCode := errors.GetExitCode(err)
description := errors.GetExitCodeDescription(exitCode)
fmt.Printf("Exit: %d - %s\n", exitCode, description)
// Output: Exit: 75 - Temporary failure (retry may succeed)
```

## Exit Code Semantics

### Success (0)
Indicates successful execution. The command completed its work without errors.

### Usage Error (2)
Indicates the command was invoked incorrectly. Common causes:
- Missing or invalid command-line flags
- Invalid argument values
- Missing required parameters

**Action**: Show usage message and correct invocation examples.

### Data Error (65)
Indicates input data format problems. Common causes:
- Invalid JSON/XML/binary format
- Schema validation failures
- Malformed input data

**Action**: Validate input data format and schema compliance.

### Unavailable (69)
Indicates a service or resource is unavailable. Common causes:
- Database connection failures
- External service downtime
- HTTP 5xx server errors

**Action**: Check service status, retry with backoff.

### Temporary Failure (75)
Indicates a transient error that may succeed on retry. Common causes:
- Network timeouts
- Connection refused
- Operation deadlines exceeded

**Action**: Implement retry logic with exponential backoff.

### Permission (77)
Indicates authentication/authorization failures. Common causes:
- Invalid credentials
- Insufficient permissions
- Token expiration

**Action**: Verify credentials, refresh tokens, check permissions.

### Configuration (78)
Indicates configuration problems. Common causes:
- Missing environment variables
- Invalid configuration files
- Required settings not provided

**Action**: Review configuration, verify environment variables.

## Testing Exit Codes

The `pkg/errors` package includes comprehensive tests for exit code mappings. Run tests with:

```bash
go test ./pkg/errors -v -run "TestGetExitCode|TestExitCodeConstants"
```

## Implementation Details

### Exit Code Determination

The `GetExitCode()` function follows this logic:

1. If error is `nil` → return `ExitSuccess` (0)
2. If error is `*StructuredError` → map by `ErrorCategory`
3. For other errors → classify first, then map

### Error Classification

Standard Go errors are automatically classified using pattern matching:

```go
"context deadline exceeded" → TimeoutError → ExitTempFail (75)
"connection refused"         → NetworkError → ExitTempFail (75)
"invalid character"          → ParseError   → ExitDataError (65)
"unauthorized"               → AuthError    → ExitPermission (77)
```

### Consistency Guarantees

- Same error category always maps to same exit code
- Exit codes follow `sysexits.h` conventions where applicable
- All exit codes are positive integers (0-255)

## Related Documentation

- [Error Types Reference](pkg/errors/types.go) - Structured error types and categories
- [Error Classification](pkg/errors/types.go) - Automatic error classification logic
- [Exit Code Tests](pkg/errors/exitcodes_test.go) - Comprehensive test coverage
