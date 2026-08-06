# Error Serialization Implementation (Bead cg-2iff2)

## Status
✓ **COMPLETE** - All acceptance criteria verified

## Implementation Location
`pkg/ingestlog/error_serializer.go` - Full implementation with comprehensive test coverage

## ErrorContext Schema
```go
type ErrorContext struct {
    Type        string `json:"type"`                  // Error type extracted via reflection
    Message     string `json:"message"`               // Error message from err.Error()
    StackTrace  string `json:"stack_trace,omitempty"` // Stack trace captured at call site
}
```

## Implemented Functions

### Primary Function
- **`SerializeError(err error) ErrorContext`** - Main serialization function accepting error and returning ErrorContext

### Advanced Variants
- **`SerializeErrorWithCaller(err error, callerDepth int) ErrorContext`** - Serialize with custom caller depth for stack trace
- **`SerializeErrorWithOptions(err error, opts *SerializationOptions) ErrorContext`** - Serialize with configurable options
- **`GetErrorChain(err error) []string`** - Extract full wrapped error chain

### Helper Functions
- **`getErrorType(err error) string`** - Extract error type using reflection with custom Type() support
- **`captureStackTrace() string`** - Capture stack trace from call site
- **`captureStackTraceWithDepth(depth int) string`** - Capture with custom skip depth
- **`simplifyTypeName(typeName string) string`** - Clean up package prefixes

## Acceptance Criteria Met

1. ✓ **Function accepts an error parameter and returns ErrorContext**
   - `SerializeError(err error) ErrorContext` - returns populated ErrorContext

2. ✓ **Error type is correctly extracted and populated**
   - Uses reflection via `reflect.TypeOf()` to get type name
   - Handles pointer types by dereferencing to element type
   - Supports custom Type() method on error types
   - Simplifies package prefixes for readability

3. ✓ **Error message is populated**
   - Extracted from `err.Error()` method

4. ✓ **Stack trace is captured and formatted as a string**
   - Uses `runtime.Callers()` to capture stack frames
   - Formats as "file:line function" for each frame
   - Captures up to 32 frames

5. ✓ **Handles nil errors gracefully**
   - Returns empty `ErrorContext{}` for nil errors
   - No nil pointer dereferences

6. ✓ **ErrorContext matches the schema**
   - Struct has `Type`, `Message`, `StackTrace` fields
   - JSON tags properly configured for serialization
   - Matches ErrorContext in `pkg/ingestlog/logger.go`

## Test Coverage

Comprehensive test suite in `pkg/ingestlog/error_serializer_test.go`:
- ✓ Basic error serialization (standard, network, nil errors)
- ✓ Custom caller depth handling
- ✓ Options-based serialization with/without stack trace
- ✓ Custom error types with Type() method
- ✓ Stack trace format verification
- ✓ Error chain extraction for wrapped errors
- ✓ Type name simplification
- ✓ Benchmarks for performance validation

All tests pass:
```bash
go test ./pkg/ingestlog/... -v
```

## Example Usage

```go
import "github.com/jedarden/commitgraph/pkg/ingestlog"

// Basic usage
err := errors.New("something went wrong")
ctx := ingestlog.SerializeError(err)
// ctx.Type = "errorString"
// ctx.Message = "something went wrong"
// ctx.StackTrace = "file:line function\n..."

// With custom caller depth
ctx = ingestlog.SerializeErrorWithCaller(err, 1)

// Without stack trace
opts := &ingestlog.SerializationOptions{IncludeStackTrace: false}
ctx = ingestlog.SerializeErrorWithOptions(err, opts)

// Get error chain
chain := ingestlog.GetErrorChain(wrappedErr)
```

## Additional Features

- **Custom Type Support**: Errors implementing `Type() string` method can provide custom type names
- **Error Chain Analysis**: `GetErrorChain()` unwraps errors to show the full wrapping hierarchy
- **Performance**: Benchmarks included for both with and without stack trace
- **Clean Type Names**: Removes common package prefixes (`*errors.`, `net.`, `github.com/...`)
