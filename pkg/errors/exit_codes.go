// Package errors provides CLI exit code constants and error handling utilities.
package errors

// Exit codes for CLI commands
const (
	// ExitCodeSuccess indicates successful execution
	ExitCodeSuccess = 0

	// ExitCodeError indicates a general error (default)
	ExitCodeError = 1

	// ExitCodeInvalidInput indicates invalid command-line arguments or input
	ExitCodeInvalidInput = 2

	// ExitCodeDatabaseError indicates database connection or query errors
	ExitCodeDatabaseError = 3

	// ExitCodeNetworkError indicates network connectivity errors
	ExitCodeNetworkError = 4

	// ExitCodeTimeoutError indicates operation timeout
	ExitCodeTimeoutError = 5

	// ExitCodeAuthenticationError indicates authentication/authorization failures
	ExitCodeAuthenticationError = 6

	// ExitCodeConfigError indicates configuration problems
	ExitCodeConfigError = 7

	// ExitCodeValidationError indicates data validation failures
	ExitCodeValidationError = 8

	// ExitCodeParseError indicates data parsing failures
	ExitCodeParseError = 9

	// ExitCodeResourceError indicates resource exhaustion/unavailability
	ExitCodeResourceError = 10

	// ExitCodeConcurrencyError indicates concurrency/locking issues
	ExitCodeConcurrencyError = 11

	// ExitCodeServerError indicates server-side errors (5xx)
	ExitCodeServerError = 12
)

// ExitCodeForErrorType maps error categories to appropriate exit codes.
func ExitCodeForErrorType(typ ErrorCategory) int {
	switch typ {
	case ValidationError:
		return ExitCodeValidationError
	case ParseError:
		return ExitCodeParseError
	case DatabaseError:
		return ExitCodeDatabaseError
	case NetworkError:
		return ExitCodeNetworkError
	case TimeoutError:
		return ExitCodeTimeoutError
	case ClientError:
		return ExitCodeInvalidInput
	case ServerError:
		return ExitCodeServerError
	case AuthError:
		return ExitCodeAuthenticationError
	case ConfigError:
		return ExitCodeConfigError
	case ResourceError:
		return ExitCodeResourceError
	case ConcurrencyError:
		return ExitCodeConcurrencyError
	default:
		return ExitCodeError
	}
}

// ExitCodeForStructuredError returns the appropriate exit code for a structured error.
func ExitCodeForStructuredError(err *StructuredError) int {
	if err == nil {
		return ExitCodeSuccess
	}
	return ExitCodeForErrorType(err.Type)
}

// SeverityForExitCode returns the severity level associated with an exit code.
func SeverityForExitCode(exitCode int) SeverityLevel {
	switch exitCode {
	case ExitCodeSuccess:
		return SeverityInfo
	case ExitCodeInvalidInput, ExitCodeValidationError, ExitCodeParseError:
		return SeverityLow
	case ExitCodeTimeoutError, ExitCodeConcurrencyError:
		return SeverityMedium
	case ExitCodeDatabaseError, ExitCodeNetworkError, ExitCodeServerError:
		return SeverityHigh
	case ExitCodeAuthenticationError, ExitCodeConfigError, ExitCodeResourceError:
		return SeverityCritical
	default:
		return SeverityLow
	}
}
