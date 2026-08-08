// Package cli provides standard error handling infrastructure for CLI entry points.
//
// This package implements a consistent error handling pattern across all command-line
// tools, including:
//   - Panic recovery with defer/recover
//   - Structured error wrapping with context
//   - Consistent exit codes
//   - Error logging for debugging
//
// Usage pattern:
//
//	func main() {
//	    cli.Main(run)
//	}
//
//	func run() error {
//	    // Actual main logic here
//	    return nil
//	}
package cli

import (
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"
)

// RunFunc is the signature for the main logic function.
// All CLI entry points should implement their main logic in a function
// of this type, then pass it to Main().
type RunFunc func() error

// Exit codes
const (
	ExitSuccess       = 0
	ExitCodeError     = 1
	ExitCodeUsage     = 2
	ExitCodePanic     = 3
	ExitCodeInterrupt = 4
)

// Main wraps a RunFunc with standard error handling and panic recovery.
// It catches panics, logs errors appropriately, and exits with the correct code.
//
// Usage:
//
//	func main() {
//	    cli.Main(run)
//	}
//
//	func run() error {
//	    // Your main logic here
//	    return nil
//	}
func Main(runFn RunFunc) {
	// Set up panic recovery
	defer func() {
		if r := recover(); r != nil {
			handlePanic(r)
		}
	}()

	// Run the main function
	if err := runFn(); err != nil {
		handleError(err)
	}

	// Success - exit with code 0 (implicitly)
}

// handlePanic processes a recovered panic and exits.
func handlePanic(r any) {
	// Build stack trace
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	stack := string(buf[:n])

	// Log the panic
	slog.Error("CLI panic recovered",
		"panic_value", fmt.Sprintf("%v", r),
		"stack_trace", stack,
	)

	// Print user-friendly message
	fmt.Fprintf(os.Stderr, "\nPANIC: %v\n\n", r)
	fmt.Fprintf(os.Stderr, "This is a bug. Please report this panic with the full error message above.\n")

	os.Exit(ExitCodePanic)
}

// handleError processes an error and exits with the appropriate code.
func handleError(err error) {
	// Check for ExitError types first to get proper exit codes
	if exitErr, ok := err.(*ExitError); ok {
		// Log at error level
		slog.Error("CLI error", "message", exitErr.Error(), "exit_code", exitErr.Code)

		// Print to stderr for user visibility
		fmt.Fprintf(os.Stderr, "Error: %s\n", exitErr.Message)

		os.Exit(exitErr.Code)
	}

	// Check for UsageError
	if usageErr, ok := err.(*UsageError); ok {
		slog.Error("CLI usage error", "message", usageErr.Error())
		fmt.Fprintf(os.Stderr, "Usage error: %s\n", usageErr.Message)
		os.Exit(ExitCodeUsage)
	}

	// Generic error - log and exit
	slog.Error("CLI error", "error", err.Error())
	fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())

	os.Exit(ExitCodeError)
}

// ExitError is a standard error that carries an exit code.
type ExitError struct {
	// Message is the error message to display
	Message string

	// Code is the exit code to use
	Code int

	// Underlying is the original error (if any)
	Underlying error
}

// Error implements the error interface.
func (e *ExitError) Error() string {
	if e.Underlying != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Underlying)
	}
	return e.Message
}

// Unwrap returns the underlying error for errors.Is/As compatibility.
func (e *ExitError) Unwrap() error {
	return e.Underlying
}

// NewExitError creates an ExitError with a message and exit code.
func NewExitError(message string, code int) *ExitError {
	return &ExitError{
		Message: message,
		Code:    code,
	}
}

// WrapExitError wraps an existing error with an exit code.
func WrapExitError(err error, code int) *ExitError {
	return &ExitError{
		Message:   err.Error(),
		Code:      code,
		Underlying: err,
	}
}

// UsageError is an error indicating incorrect command-line usage.
// It should exit with code 2 (Unix convention for usage errors).
type UsageError struct {
	Message string
}

// Error implements the error interface.
func (e *UsageError) Error() string {
	return fmt.Sprintf("usage error: %s", e.Message)
}

// NewUsageError creates a UsageError.
func NewUsageError(message string) *UsageError {
	return &UsageError{Message: message}
}

// IsUsageError checks if an error is a UsageError.
func IsUsageError(err error) bool {
	_, ok := err.(*UsageError)
	return ok
}

// WrapContext wraps an error with additional context information.
// This is useful for adding stack-like context to errors as they propagate.
func WrapContext(err error, context string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", context, err)
}

// SanitizeError removes sensitive information from error messages.
// This is a basic implementation that looks for common patterns.
// Extend this as needed for your specific sensitive data patterns.
func SanitizeError(err error) error {
	if err == nil {
		return nil
	}

	msg := err.Error()

	// Sanitize common sensitive patterns
	sanitizers := []struct {
		pattern     string
		replacement string
	}{
		{"password=[^\\s]+", "password=REDACTED"},
		{"token=[^\\s]+", "token=REDACTED"},
		{"api_key=[^\\s]+", "api_key=REDACTED"},
		{"secret=[^\\s]+", "secret=REDACTED"},
		{"authorization:\\s*Bearer\\s+[^\\s]+", "authorization: Bearer REDACTED"},
	}

	sanitized := msg
	for _, s := range sanitizers {
		// Simple case-insensitive replacement
		lower := strings.ToLower(sanitized)
		if strings.Contains(lower, s.pattern) {
			// This is a very simple sanitization - for production use,
			// consider using proper regex with case-insensitive matching
			parts := strings.Split(sanitized, " ")
			for i, part := range parts {
				if strings.Contains(strings.ToLower(part), strings.Split(s.pattern, "=")[0]) {
					parts[i] = s.replacement
				}
			}
			sanitized = strings.Join(parts, " ")
		}
	}

	if sanitized != msg {
		return fmt.Errorf("%s (sanitized)", sanitized)
	}

	return err
}
