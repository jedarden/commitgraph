// Package errors provides exit code constants and mapping functions for CLI applications.
package errors

import (
	"fmt"
	"os"
)

// Exit code constants following BSD sysexits.h conventions where applicable.
const (
	// ExitSuccess indicates successful execution (0 is the standard success exit code).
	ExitSuccess = 0

	// ExitError indicates a general error occurred (1 is the standard error exit code).
	ExitError = 1

	// ExitUsage indicates command line usage error (2 follows BSD sysexits.h EX_USAGE).
	ExitUsage = 2

	// ExitDataError indicates data format error (65 follows BSD sysexits.h EX_DATAERR).
	ExitDataError = 65

	// ExitNoInput indicates no input or insufficient data (66 follows BSD sysexits.h EX_NOINPUT).
	ExitNoInput = 66

	// ExitNoUser indicates user不存在 (67 follows BSD sysexits.h EX_NOUSER).
	ExitNoUser = 67

	// ExitNoHost indicates host unknown (68 follows BSD sysexits.h EX_NOHOST).
	ExitNoHost = 68

	// ExitUnavailable indicates service unavailable (69 follows BSD sysexits.h EX_UNAVAILABLE).
	ExitUnavailable = 69

	// ExitSoftware indicates internal software error (70 follows BSD sysexits.h EX_SOFTWARE).
	ExitSoftware = 70

	// ExitOSError indicates system error (71 follows BSD sysexits.h EX_OSERR).
	ExitOSError = 71

	// ExitIOError indicates I/O error (74 follows BSD sysexits.h EX_IOERR).
	ExitIOError = 74

	// ExitTempFail indicates temporary failure (75 follows BSD sysexits.h EX_TEMPFAIL).
	ExitTempFail = 75

	// ExitProtocol indicates protocol error (76 follows BSD sysexits.h EX_PROTOCOL).
	ExitProtocol = 76

	// ExitPermission indicates permission denied (77 follows BSD sysexits.h EX_NOPERM).
	ExitPermission = 77

	// ExitConfig indicates configuration error (custom, not in sysexits.h).
	ExitConfig = 78
)

// exitCodeMapping defines the mapping from error categories to exit codes.
var exitCodeMapping = map[ErrorCategory]int{
	ValidationError:  ExitUsage,      // Input validation failures are usage errors
	ParseError:       ExitDataError,  // Data parsing failures are data errors
	DatabaseError:    ExitUnavailable, // Database failures indicate service unavailable
	NetworkError:     ExitTempFail,   // Network errors are temporary failures
	TimeoutError:     ExitTempFail,   // Timeouts are temporary failures
	ClientError:      ExitUsage,      // HTTP 4xx errors are typically usage/parameter errors
	ServerError:      ExitUnavailable, // HTTP 5xx errors indicate service unavailable
	AuthError:        ExitPermission,  // Authentication/authorization failures
	ConfigError:      ExitConfig,     // Configuration problems
	ResourceError:    ExitOSError,    // Resource exhaustion is an OS error
	ConcurrencyError: ExitSoftware,   // Concurrency issues are internal software errors
	UnknownError:     ExitError,      // Uncategorized errors get general error code
}

// GetExitCode returns the appropriate exit code for a given error.
// If the error is nil, it returns ExitSuccess (0).
// If the error is a StructuredError, it maps based on the error category.
// For other error types, it attempts to classify them first.
func GetExitCode(err error) int {
	if err == nil {
		return ExitSuccess
	}

	// If it's a StructuredError, use its category
	if structuredErr, ok := err.(*StructuredError); ok {
		return getExitCodeForCategory(structuredErr.Type)
	}

	// For other errors, classify them and map
	category := classifyError(err)
	return getExitCodeForCategory(category)
}

// getExitCodeForCategory returns the exit code for a given error category.
func getExitCodeForCategory(category ErrorCategory) int {
	if code, ok := exitCodeMapping[category]; ok {
		return code
	}
	return ExitError
}

// ExitWithCode exits the process with the appropriate exit code for the given error.
// If the error is nil, it exits with ExitSuccess (0).
// Otherwise, it logs the error and exits with the mapped exit code.
func ExitWithCode(err error) {
	exitCode := GetExitCode(err)
	if err != nil {
		// Log the error before exiting
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}
	os.Exit(exitCode)
}

// MustExitWithCode exits the process with the given exit code.
// This is useful when you already know the exit code you want.
func MustExitWithCode(code int) {
	os.Exit(code)
}

// IsSuccess checks if an exit code indicates success.
func IsSuccess(code int) bool {
	return code == ExitSuccess
}

// IsError checks if an exit code indicates an error (non-zero).
func IsError(code int) bool {
	return code != ExitSuccess
}

// IsTemporary checks if an exit code indicates a temporary failure (retryable).
func IsTemporary(code int) bool {
	return code == ExitTempFail || code == ExitUnavailable
}

// IsUsageError checks if an exit code indicates a usage/parameter error.
func IsUsageError(code int) bool {
	return code == ExitUsage
}

// IsPermissionError checks if an exit code indicates a permission error.
func IsPermissionError(code int) bool {
	return code == ExitPermission
}

// GetExitCodeDescription returns a human-readable description for an exit code.
func GetExitCodeDescription(code int) string {
	descriptions := map[int]string{
		ExitSuccess:     "Success",
		ExitError:       "General error",
		ExitUsage:       "Command line usage error",
		ExitDataError:   "Data format error",
		ExitNoInput:     "No input or insufficient data",
		ExitNoUser:      "User不存在",
		ExitNoHost:      "Host unknown",
		ExitUnavailable: "Service unavailable",
		ExitSoftware:    "Internal software error",
		ExitOSError:     "System error (resource exhausted, etc.)",
		ExitIOError:     "I/O error",
		ExitTempFail:    "Temporary failure (retry may succeed)",
		ExitProtocol:    "Protocol error",
		ExitPermission:  "Permission denied",
		ExitConfig:      "Configuration error",
	}

	if desc, ok := descriptions[code]; ok {
		return desc
	}
	return fmt.Sprintf("Unknown exit code: %d", code)
}
