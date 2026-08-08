# CLI Error Handling Package

`pkg/clierror` provides standard error handling infrastructure for CLI entry points in the commitgraph project.

## Purpose

This package addresses a common issue across CLI tools: ensuring that:

1. Panics are caught and logged with stack traces before exit
2. Errors are logged consistently and exit codes are appropriate
3. Main functions never return normally (always call `os.Exit()`)

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
2. Exit with code 1 (or custom code via `ExitCodeError`)

### Custom Exit Codes

For CLI tools that need specific exit codes:

```go
import "github.com/jedarden/commitgraph/pkg/clierror"

func run() error {
    // Some validation error
    if !validate() {
        return clierror.NewExitCodeError(2, fmt.Errorf("validation failed"))
    }
    return nil
}
```

## Testing

The package is designed to work with `os.Exit()`, which makes direct testing challenging. For testing CLI tools:

1. Extract the main logic into a separate `run()` function
2. Test `run()` directly without calling `clierror.Run`
3. `clierror.Run` is only for the actual main function

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
        return err
    }
    return nil
}
```

## See Also

- `pkg/warmstart/error.go` - Similar error handling patterns for warmstart parsing
- Go's standard `errors` package for error wrapping
