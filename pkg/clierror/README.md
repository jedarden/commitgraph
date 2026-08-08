# CLI Error Handling Package

`pkg/clierror` provides standard error handling infrastructure for CLI entry points in the commitgraph project.

## Purpose

This package addresses a common issue across CLI tools: ensuring that:

1. Panics are caught and logged with stack traces before exit
2. Errors are logged consistently and exit codes are appropriate
3. Main functions never return normally (always call `os.Exit()`)
4. Errors are wrapped with structured context for better debugging
5. Exit codes follow standard conventions based on error categories

## Usage

Basic pattern for all CLI entry points:

```go
package main

import (
    "github.com/jedarden/commitgraph/pkg/clierror"
)

func main() {
    clierror.Run(run)
}

func run() error {
    // Your CLI logic here
    // Return nil on success
    // Return error on failure
    return nil
}
```

## Features

### Panic Recovery

If your `run()` function panics, `clierror.Run` will:

1. Recover from the panic
2. Log the panic value
3. Log the full stack trace
4. Exit with code 1

### Error Handling

If your `run()` function returns an error, `clierror.Run` will:

1. Log the error message
2. Exit with the appropriate code (determined by error category)

### Error Categories and Exit Codes

The package provides standard error categories with default exit codes following common CLI conventions:

| Category | Exit Code | Usage |
|----------|-----------|-------|
| `CategoryUsage` | 2 | Command-line argument/flag errors |
| `CategoryInput` | 3 | File validation, data parsing errors |
| `CategoryNetwork` | 4 | Connection failures, timeouts |
| `CategoryPermission` | 5 | Authorization/access control errors |
| `CategoryInternal` | 70 | Bugs, unexpected errors (should not occur) |
| `CategoryTransient` | 75 | Temporary failures (might succeed on retry) |

## Error Wrapping Patterns

### Basic Wrapping with Context

```go
func run() error {
    data, err := os.ReadFile("config.json")
    if err != nil {
        return clierror.NewInput(
            "failed to read configuration file",
            err,
        )
    }
    // ... process data
    return nil
}
```

### Wrapping with Additional Context

```go
func run() error {
    data, err := os.ReadFile(filepath)
    if err != nil {
        return clierror.NewInputWithContext(
            "failed to read file",
            filepath,
            err,
        )
    }
    // ... process data
    return nil
}
```

### Convenience Wrappers

The package provides convenience functions that only wrap if the error is non-nil:

```go
func run() error {
    data, err := os.ReadFile("input.json")
    if err != nil {
        return clierror.WrapInput("failed to read input file", err)
    }

    var config Config
    if err := json.Unmarshal(data, &config); err != nil {
        return clierror.WrapInput("failed to parse input JSON", err)
    }

    // ... process config
    return nil
}
```

### Network Error Handling

```go
func run() error {
    resp, err := http.Get(url)
    if err != nil {
        return clierror.WrapNetwork("failed to fetch data", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        return clierror.NewNetwork(
            fmt.Sprintf("server returned error status: %d", resp.StatusCode),
            fmt.Errorf("HTTP %d", resp.StatusCode),
        )
    }
    // ... process response
    return nil
}
```

### Usage Error Handling

```go
func run() error {
    if *inputPath == "" {
        return clierror.NewUsage(
            "missing required --input flag",
            fmt.Errorf("no input path specified"),
        )
    }
    // ... process
    return nil
}
```

### All Conditional Wrappers

The conditional wrappers are useful when you want to add context but only if an error actually occurred:

```go
func run() error {
    // Usage errors
    if err := validateFlags(); err != nil {
        return clierror.WrapUsage("flag validation failed", err)
    }

    // Input errors
    if err := parseInput(data); err != nil {
        return clierror.WrapInput("input parsing failed", err)
    }

    // Network errors
    if err := fetchRemoteData(); err != nil {
        return clierror.WrapNetwork("remote fetch failed", err)
    }

    // Permission errors
    if err := checkAccess(); err != nil {
        return clierror.WrapPermission("access check failed", err)
    }

    // Internal errors (bugs)
    if err := internalInvariantCheck(); err != nil {
        return clierror.WrapInternal("invariant violation", err)
    }

    // Transient errors (retryable)
    if err := attemptOperation(); err != nil {
        return clierror.WrapTransient("temporary failure", err)
    }

    return nil
}
```

### Custom Exit Codes

You can override the default exit code for any category:

```go
func run() error {
    if !isValid() {
        return clierror.NewWrapWithExitCode(
            clierror.CategoryInput,
            "validation failed",
            10, // Custom exit code
            fmt.Errorf("invalid data format"),
        )
    }
    return nil
}
```

## Error Types

### WrappedError

The primary error type that provides structured context:

```go
type WrappedError struct {
    Category         ErrorCategory  // Error classification
    Message          string         // Human-readable description
    Context          string         // Additional context
    Err              error          // Underlying error
    ExitCodeOverride int            // Custom exit code
}
```

### ExitCodeError (Legacy)

Legacy error type for simple exit code wrapping:

```go
type ExitCodeError struct {
    ExitCode int
    Err      error
}
```

## Helper Functions

### Category-Specific Constructors

- `NewUsage(message, err)` - Usage/argument errors (exit code 2)
- `NewInput(message, err)` - Input/data errors (exit code 3)
- `NewInputWithContext(message, context, err)` - Input errors with context
- `NewNetwork(message, err)` - Network errors (exit code 4)
- `NewPermission(message, err)` - Permission errors (exit code 5)
- `NewInternal(message, err)` - Internal errors (exit code 70)
- `NewTransient(message, err)` - Transient errors (exit code 75)

### Conditional Wrappers

These functions only wrap if the error is non-nil (return nil if err == nil):

- `WrapInput(message, err)` - Conditional input error wrapping
- `WrapInputWithContext(message, context, err)` - Conditional with context
- `WrapNetwork(message, err)` - Conditional network error wrapping
- `WrapUsage(message, err)` - Conditional usage error wrapping
- `WrapPermission(message, err)` - Conditional permission error wrapping
- `WrapInternal(message, err)` - Conditional internal error wrapping
- `WrapTransient(message, err)` - Conditional transient error wrapping

### General Wrappers

- `NewWrap(category, message, err)` - Generic wrapped error
- `NewWrapWithContext(category, message, context, err)` - With context
- `NewWrapWithExitCode(category, message, exitCode, err)` - Custom exit code

### Utility Functions

- `GetExitCode(err)` - Extract exit code from any error type

## Testing

The package is designed to work with `os.Exit()`, which makes direct testing challenging. For testing CLI tools:

1. Extract the main logic into a separate `run()` function
2. Test `run()` directly without calling `clierror.Run`
3. `clierror.Run` is only for the actual main function

```go
// In your test file
func TestRun(t *testing.T) {
    // Test the run function directly
    err := run()
    if err != nil {
        t.Errorf("run() failed: %v", err)
    }
}
```

## Migration Guide

### Before (no error catching):

```go
func main() {
    if err := doSomething(); err != nil {
        log.Fatalf("Error: %v", err)
    }
}
```

### After (with error catching):

```go
func main() {
    clierror.Run(run)
}

func run() error {
    if err := doSomething(); err != nil {
        return clierror.WrapInput("operation failed", err)
    }
    return nil
}
```

### Before (basic error wrapping):

```go
func run() error {
    data, err := os.ReadFile("config.json")
    if err != nil {
        return fmt.Errorf("failed to read config: %w", err)
    }
    // ... process
    return nil
}
```

### After (structured error wrapping):

```go
func run() error {
    data, err := os.ReadFile("config.json")
    if err != nil {
        return clierror.NewInputWithContext(
            "failed to read configuration",
            "config.json",
            err,
        )
    }
    // ... process
    return nil
}
```

## Complete Example

```go
package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "os"

    "github.com/jedarden/commitgraph/pkg/clierror"
)

func main() {
    clierror.Run(run)
}

func run() error {
    inputPath := flag.String("input", "", "Path to input file")
    flag.Parse()

    // Usage error handling
    if *inputPath == "" {
        return clierror.NewUsage(
            "missing required flag",
            fmt.Errorf("--input is required"),
        )
    }

    // Input reading with context
    data, err := os.ReadFile(*inputPath)
    if err != nil {
        return clierror.NewInputWithContext(
            "failed to read input file",
            *inputPath,
            err,
        )
    }

    // Input parsing
    var config Config
    if err := json.Unmarshal(data, &config); err != nil {
        return clierror.WrapInput("failed to parse input JSON", err)
    }

    // Network operation (if needed)
    if err := processData(&config); err != nil {
        return clierror.WrapNetwork("failed to process data", err)
    }

    fmt.Println("Processing complete!")
    return nil
}
```

## Comprehensive Examples

The file `examples.go` contains 15 comprehensive examples covering common CLI error handling patterns:

1. **Basic usage error handling** - Flag and argument validation
2. **File reading with context** - Including file paths in error messages
3. **JSON parsing** - Structured data parsing errors
4. **Network operations** - HTTP requests and connection failures
5. **Database connections** - Multiple error types in one operation
6. **Directory walking** - Error accumulation during batch processing
7. **Conditional wrapping** - Only wrapping when error occurs
8. **Custom exit codes** - Overriding default exit codes
9. **Internal errors** - Bug/invariant violations
10. **Transient errors** - Retryable temporary failures
11. **Multi-step operations** - Layered error handling
12. **Complete run() function** - Full CLI structure
13. **Error accumulation** - Collecting multiple errors
14. **Permission handling** - Access control errors
15. **Panic-safe processing** - Working with panic recovery

Use these examples as reference patterns when implementing or refactoring CLI commands.

```bash
# View the examples
cat pkg/clierror/examples.go
```

## Quick Reference: Common Patterns

### When to use each error category

| Scenario | Category | Exit Code | Function |
|----------|----------|-----------|----------|
| Missing/invalid flags | `CategoryUsage` | 2 | `NewUsage`, `WrapUsage` |
| File read/write errors | `CategoryInput` | 3 | `NewInput`, `WrapInput`, `NewInputWithContext` |
| JSON parse errors | `CategoryInput` | 3 | `WrapInput` |
| Connection failures | `CategoryNetwork` | 4 | `NewNetwork`, `WrapNetwork` |
| Permission denied | `CategoryPermission` | 5 | `NewPermission`, `WrapPermission` |
| Bugs/invariant violations | `CategoryInternal` | 70 | `NewInternal`, `WrapInternal` |
| Temporary failures | `CategoryTransient` | 75 | `NewTransient`, `WrapTransient` |

### Pattern: Include context when possible

```go
// ❌ Bad: No context
return clierror.NewInput("file read failed", err)

// ✅ Good: Includes which file
return clierror.NewInputWithContext("file read failed", filepath, err)
```

### Pattern: Use conditional wrappers

```go
// These only wrap if err != nil (return nil if err is nil)
return clierror.WrapInput("failed to parse", err)
return clierror.WrapNetwork("connection failed", err)
```

## See Also

- `pkg/clierror/examples.go` - 15 comprehensive error wrapping examples
- `pkg/warmstart/error.go` - Similar error handling patterns for warmstart parsing
- Go's standard `errors` package for error wrapping
- Unix/SysExit conventions for exit code meanings
