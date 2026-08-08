// Package clierror provides standard error handling infrastructure for CLI entry points.
//
// This package provides utilities for catching panics, wrapping errors, and ensuring
// clean error propagation from CLI main functions without panics reaching the runtime.
//
// Usage:
//
//   func main() {
//       clierror.Run(run)
//   }
//
//   func run() error {
//       // Your main logic here
//       return nil
//   }
package clierror

import (
	"log"
	"os"
	"runtime/debug"
)

// ExitCodeError is an error that carries a specific exit code.
type ExitCodeError struct {
	// ExitCode is the code to exit with (defaults to 1 if 0).
	ExitCode int

	// Err is the underlying error.
	Err error
}

// Error implements the error interface.
func (e *ExitCodeError) Error() string {
	if e.Err == nil {
		return "CLI error"
	}
	return e.Err.Error()
}

// Unwrap returns the underlying error for errors.Is/As compatibility.
func (e *ExitCodeError) Unwrap() error {
	return e.Err
}

// NewExitCodeError creates a new ExitCodeError with the given code and underlying error.
func NewExitCodeError(code int, err error) *ExitCodeError {
	return &ExitCodeError{
		ExitCode: code,
		Err:      err,
	}
}

// Run executes a function, catching panics and handling errors cleanly.
//
// If fn returns an error, Run prints it to stderr and exits with code 1
// (or the code from ExitCodeError if present).
//
// If fn panics, Run recovers, logs the panic with stack trace, and exits with code 1.
//
// Run never returns - it always calls os.Exit().
func Run(fn func() error) {
	// Catch panics
	defer func() {
		if r := recover(); r != nil {
			// Log panic with stack trace
			log.Printf("PANIC: %v\n", r)
			log.Println("Stack trace:")
			log.Println(string(debug.Stack()))
			os.Exit(1)
		}
	}()

	// Execute the function
	if err := fn(); err != nil {
		// Handle the error
		handleError(err)
		os.Exit(1) // Never reached (handleError calls os.Exit)
	}

	// Success - exit with code 0
	os.Exit(0)
}

// handleError prints an error to stderr and exits with the appropriate code.
// This function always calls os.Exit and never returns.
func handleError(err error) {
	if err == nil {
		return
	}

	// Extract exit code if this is an ExitCodeError
	exitCode := 1
	if exitCodeErr, ok := err.(*ExitCodeError); ok {
		exitCode = exitCodeErr.ExitCode
		// Use the underlying error for logging
		if exitCodeErr.Err != nil {
			err = exitCodeErr.Err
		}
	}

	// Log the error
	log.Printf("Error: %v\n", err)

	// Exit with the appropriate code
	os.Exit(exitCode)
}

// RunWithExitCode executes a function, catching panics and handling errors cleanly.
//
// If fn returns an error, RunWithExitCode prints it to stderr and exits with code 1
// (or the code from ExitCodeError if present).
//
// If fn panics, RunWithExitCode recovers, logs the panic with stack trace, and exits with code 1.
//
// If fn returns successfully, RunWithExitCode exits with the provided code.
//
// RunWithExitCode never returns - it always calls os.Exit().
func RunWithExitCode(fn func() error, successCode int) {
	// Catch panics
	defer func() {
		if r := recover(); r != nil {
			// Log panic with stack trace
			log.Printf("PANIC: %v\n", r)
			log.Println("Stack trace:")
			log.Println(string(debug.Stack()))
			os.Exit(1)
		}
	}()

	// Execute the function
	if err := fn(); err != nil {
		// Handle the error
		handleError(err)
		os.Exit(1) // Never reached (handleError calls os.Exit)
	}

	// Success - exit with provided code
	os.Exit(successCode)
}
