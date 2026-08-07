// Package ingestlog provides structured logging for ingest endpoint operations.
package ingestlog

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
)

// SerializeError serializes an error into an ErrorContext structure.
// This function extracts error type, message, and stack trace from the provided error.
//
// Parameters:
//   - err: The error to serialize (can be nil)
//
// Returns:
//   - ErrorContext: Populated error context with type, message, and stack trace
//
// The function handles nil errors gracefully by returning an empty ErrorContext.
// Error type is extracted using reflection to get the underlying type name.
// Stack trace is captured at the point of this call for debugging purposes.
func SerializeError(err error) ErrorContext {
	if err == nil {
		return ErrorContext{}
	}

	// Extract error message
	message := err.Error()

	// Extract error type using reflection
	errorType := getErrorType(err)

	// Capture stack trace
	stackTrace := captureStackTrace()

	return ErrorContext{
		Type:       errorType,
		Message:    message,
		StackTrace: stackTrace,
	}
}

// SerializeErrorWithCaller serializes an error into an ErrorContext structure,
// capturing the stack trace from a specific caller depth.
//
// Parameters:
//   - err: The error to serialize (can be nil)
//   - callerDepth: The number of stack frames to skip (0 = this function, 1 = direct caller, etc.)
//
// Returns:
//   - ErrorContext: Populated error context with type, message, and stack trace
//
// Use this function when you want to capture the stack trace from a specific
// call site, such as the function that originated the error.
func SerializeErrorWithCaller(err error, callerDepth int) ErrorContext {
	if err == nil {
		return ErrorContext{}
	}

	// Extract error message
	message := err.Error()

	// Extract error type using reflection
	errorType := getErrorType(err)

	// Capture stack trace with specified caller depth
	stackTrace := captureStackTraceWithDepth(callerDepth + 2) // +2 for this function and SerializeErrorWithCaller

	return ErrorContext{
		Type:       errorType,
		Message:    message,
		StackTrace: stackTrace,
	}
}

// SerializeErrorWithOptions serializes an error into an ErrorContext structure
// with additional options for customization.
//
// Parameters:
//   - err: The error to serialize (can be nil)
//   - opts: Serialization options (can be nil for defaults)
//
// Returns:
//   - ErrorContext: Populated error context based on provided options
func SerializeErrorWithOptions(err error, opts *SerializationOptions) ErrorContext {
	if err == nil {
		return ErrorContext{}
	}

	// Apply defaults if options not provided
	if opts == nil {
		opts = &SerializationOptions{
			IncludeStackTrace: true,
			CallerDepth:        0,
		}
	}

	ctx := ErrorContext{
		Message: err.Error(),
		Type:    getErrorType(err),
	}

	// Optionally include stack trace
	if opts.IncludeStackTrace {
		ctx.StackTrace = captureStackTraceWithDepth(opts.CallerDepth + 2)
	}

	return ctx
}

// SerializationOptions configures error serialization behavior.
type SerializationOptions struct {
	IncludeStackTrace bool // Whether to capture stack trace
	CallerDepth        int  // Stack frame skip depth for stack trace capture
}

// getErrorType extracts the error type using reflection.
// It returns the underlying type name of the error, unwrapping if necessary.
func getErrorType(err error) string {
	if err == nil {
		return "unknown"
	}

	// Use reflection to get the type name
	errType := reflect.TypeOf(err)

	// Handle nil interface case
	if errType == nil {
		return "unknown"
	}

	// Get the type name
	typeName := errType.String()

	// For pointer types, get the element type name instead
	if errType.Kind() == reflect.Ptr {
		typeName = errType.Elem().String()
	}

	// Clean up common package prefixes for readability
	typeName = simplifyTypeName(typeName)

	// If error implements a Type() method, use that
	if typeErr, ok := err.(interface{ Type() string }); ok {
		if customType := typeErr.Type(); customType != "" {
			return customType
		}
	}

	return typeName
}

// simplifyTypeName simplifies type names by removing common package prefixes.
func simplifyTypeName(typeName string) string {
	// Remove common prefixes
	prefixes := []string{
		"*",
		"errors.",
		"fmt.",
		"net.",
		"net/http.",
		"database/sql.",
		"github.com/",
	}

	for _, prefix := range prefixes {
		typeName = strings.TrimPrefix(typeName, prefix)
	}

	// For import-path-qualified types (e.g. github.com/user/repo/pkg.Type),
	// drop the path down to the last path segment...
	if idx := strings.LastIndex(typeName, "/"); idx != -1 {
		typeName = typeName[idx+1:]
	}
	// ...then drop the package qualifier, leaving just the bare type name.
	if idx := strings.LastIndex(typeName, "."); idx != -1 {
		typeName = typeName[idx+1:]
	}

	return typeName
}

// captureStackTrace captures the current stack trace as a formatted string.
func captureStackTrace() string {
	return captureStackTraceWithDepth(2) // Start from caller of captureStackTrace
}

// captureStackTraceWithDepth captures the stack trace starting from a specific depth.
func captureStackTraceWithDepth(depth int) string {
	const maxDepth = 32
	pcs := make([]uintptr, maxDepth)

	// Skip the specified number of frames
	n := runtime.Callers(depth+1, pcs)
	if n == 0 {
		return ""
	}

	pcs = pcs[:n]

	frames := runtime.CallersFrames(pcs)
	var sb strings.Builder

	for {
		frame, more := frames.Next()

		// Format: file:line function
		sb.WriteString(fmt.Sprintf("%s:%d %s\n", frame.File, frame.Line, frame.Function))

		if !more {
			break
		}
	}

	return sb.String()
}

// GetErrorChain returns the chain of wrapped errors as a slice of error types.
// This is useful for understanding the full error wrapping hierarchy.
//
// Parameters:
//   - err: The error to analyze (can be nil)
//
// Returns:
//   - []string: Slice of error type names in the chain, from outermost to innermost
func GetErrorChain(err error) []string {
	if err == nil {
		return nil
	}

	var chain []string

	current := err
	for current != nil {
		chain = append(chain, getErrorType(current))

		// Attempt to unwrap. Note: a `break` inside a type switch only exits
		// the switch, not this for loop, so termination must be handled
		// explicitly via a sentinel "did we advance current?" check.
		next := current
		switch wrapped := current.(type) {
		case interface{ Unwrap() error }:
			next = wrapped.Unwrap()
		case interface{ Unwrap() []error }:
			// For multiple wrapped errors, just take the first one for the chain.
			unwrapped := wrapped.Unwrap()
			if len(unwrapped) > 0 {
				next = unwrapped[0]
			} else {
				next = nil
			}
		default:
			next = nil
		}

		if next == current {
			// No Unwrap method and no progress made - stop to avoid an infinite loop.
			break
		}
		current = next
	}

	return chain
}
