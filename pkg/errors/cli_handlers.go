// Package errors provides CLI error handling utilities.
package errors

import (
	"fmt"
	"os"
)

// CLIHandler handles CLI error presentation and exit codes.
type CLIHandler struct {
	programName string
	verbose      bool
}

// NewCLIHandler creates a new CLI handler.
func NewCLIHandler(programName string) *CLIHandler {
	return &CLIHandler{
		programName: programName,
		verbose:     false,
	}
}

// SetVerbose sets whether to show verbose error details.
func (h *CLIHandler) SetVerbose(verbose bool) {
	h.verbose = verbose
}

// HandleError handles an error and exits with the appropriate exit code.
// This function never returns - it always calls os.Exit().
func (h *CLIHandler) HandleError(err error) {
	h.handleError(err, true)
}

// HandleErrorNoExit handles an error and returns the exit code without exiting.
// This is useful for testing or when you want to handle the exit yourself.
func (h *CLIHandler) HandleErrorNoExit(err error) int {
	return h.handleError(err, false)
}

// handleError is the internal error handling implementation.
func (h *CLIHandler) handleError(err error, shouldExit bool) int {
	if err == nil {
		return ExitCodeSuccess
	}

	// Check if it's a structured error
	structuredErr, isStructured := err.(*StructuredError)
	exitCode := ExitCodeError

	if isStructured {
		exitCode = ExitCodeForStructuredError(structuredErr)
		h.printStructuredError(structuredErr)
	} else {
		exitCode = h.printGenericError(err)
	}

	if shouldExit {
		os.Exit(exitCode)
	}
	return exitCode
}

// printStructuredError prints a formatted structured error to stderr.
func (h *CLIHandler) printStructuredError(err *StructuredError) {
	// Print the main error message
	fmt.Fprintf(os.Stderr, "%s: error: %s\n", h.programName, err.Message)

	// Print error code and type
	fmt.Fprintf(os.Stderr, "%s:   error code: %s\n", h.programName, err.Code)
	fmt.Fprintf(os.Stderr, "%s:   error type: %s\n", h.programName, err.Type)

	// Print component and operation if available
	if err.Component != "" {
		fmt.Fprintf(os.Stderr, "%s:   component: %s\n", h.programName, err.Component)
	}
	if err.Operation != "" {
		fmt.Fprintf(os.Stderr, "%s:   operation: %s\n", h.programName, err.Operation)
	}

	// Print domain-specific context
	if err.CommitSHA != "" {
		fmt.Fprintf(os.Stderr, "%s:   commit: %s\n", h.programName, err.CommitSHA)
	}
	if err.Email != "" {
		fmt.Fprintf(os.Stderr, "%s:   email: %s\n", h.programName, err.Email)
	}
	if err.RecordKey != "" {
		fmt.Fprintf(os.Stderr, "%s:   record: %s\n", h.programName, err.RecordKey)
	}

	// Print recovery suggestion if available
	if err.Recovery.Action != "" {
		fmt.Fprintf(os.Stderr, "%s:   recovery: %s\n", h.programName, err.Recovery.Action)
		if h.verbose && len(err.Recovery.Steps) > 0 {
			for _, step := range err.Recovery.Steps {
				fmt.Fprintf(os.Stderr, "%s:     - %s\n", h.programName, step)
			}
		}
	}

	// Print verbose details
	if h.verbose {
		if err.Cause != nil {
			fmt.Fprintf(os.Stderr, "%s:   caused by: %v\n", h.programName, err.Cause)
		}
		if err.StackTrace != "" {
			fmt.Fprintf(os.Stderr, "%s:   stack trace:\n%s\n", h.programName, err.StackTrace)
		}
	}
}

// printGenericError prints a generic error to stderr and returns an exit code.
func (h *CLIHandler) printGenericError(err error) int {
	// Try to classify the error
	typ := classifyError(err)
	exitCode := ExitCodeForErrorType(typ)

	fmt.Fprintf(os.Stderr, "%s: error: %v\n", h.programName, err)

	// Add helpful hints based on error type
	switch typ {
	case NetworkError:
		fmt.Fprintf(os.Stderr, "%s: hint: Check your network connection and endpoint availability\n", h.programName)
	case DatabaseError:
		fmt.Fprintf(os.Stderr, "%s: hint: Verify database connection and credentials\n", h.programName)
	case TimeoutError:
		fmt.Fprintf(os.Stderr, "%s: hint: Operation timed out - retry with increased timeout if needed\n", h.programName)
	case ValidationError:
		fmt.Fprintf(os.Stderr, "%s: hint: Check your input parameters and command-line flags\n", h.programName)
	case ParseError:
		fmt.Fprintf(os.Stderr, "%s: hint: Verify data format and schema compatibility\n", h.programName)
	}

	return exitCode
}

// Fatal is a convenience function that handles an error and exits.
// This is similar to log.Fatal but uses structured error handling.
func (h *CLIHandler) Fatal(err error) {
	h.HandleError(err)
}

// FatalIfError is a convenience function that handles an error if it's non-nil.
func (h *CLIHandler) FatalIfError(err error) {
	if err != nil {
		h.HandleError(err)
	}
}

// Success exits with success code and an optional message.
func (h *CLIHandler) Success(message string) {
	if message != "" {
		fmt.Println(message)
	}
	os.Exit(ExitCodeSuccess)
}

// ValidationErrorMessage formats a validation error message.
func ValidationErrorMessage(field, reason string) string {
	return fmt.Sprintf("validation failed for %s: %s", field, reason)
}

// RequiredFlagError creates an error for a missing required flag.
func RequiredFlagError(flagName string) error {
	return NewError(
		ValidationError,
		SeverityLow,
		fmt.Sprintf("required flag -%s is not set", flagName),
		"MISSING_REQUIRED_FLAG",
		"cli",
		"parse_flags",
	)
}

// InvalidFlagValueError creates an error for an invalid flag value.
func InvalidFlagValueError(flagName, value, reason string) error {
	return NewError(
		ValidationError,
		SeverityLow,
		fmt.Sprintf("invalid value for -%s: %s (%s)", flagName, value, reason),
		"INVALID_FLAG_VALUE",
		"cli",
		"parse_flags",
	)
}

// WrapDatabaseConnectionError wraps an existing error as a database connection error.
func WrapDatabaseConnectionError(cause error) error {
	return WrapError(cause, *NewError(
		DatabaseError,
		SeverityHigh,
		"failed to connect to database",
		"DATABASE_CONNECTION_FAILED",
		"database",
		"connect",
	)).WithRecovery(RecoverySuggestion{
		Action:   "Check database connection and credentials",
		Steps:    []string{"Verify database is running", "Check connection parameters", "Validate credentials"},
		Severity: SeverityHigh,
	})
}

// QueryExecutionError creates an error for query execution failures.
func QueryExecutionError(query string, cause error) *StructuredError {
	return WrapError(cause, *NewError(
		DatabaseError,
		SeverityHigh,
		"failed to execute database query",
		"QUERY_EXECUTION_FAILED",
		"database",
		"query",
	)).WithRecovery(RecoverySuggestion{
		Action:   "Verify query syntax and database state",
		Steps:    []string{"Check query syntax", "Verify table/column names", "Review database logs"},
		Severity: SeverityHigh,
	})
}

// NetworkRequestError creates an error for HTTP/network request failures.
func NetworkRequestError(url string, cause error) *StructuredError {
	return WrapError(cause, *NewError(
		NetworkError,
		SeverityHigh,
		fmt.Sprintf("failed to make request to %s", url),
		"NETWORK_REQUEST_FAILED",
		"http",
		"request",
	)).WithRecovery(RecoverySuggestion{
		Action:   "Check network connectivity and endpoint availability",
		Steps:    []string{"Verify network connection", "Check DNS resolution", "Confirm endpoint is reachable"},
		Severity: SeverityHigh,
	})
}
