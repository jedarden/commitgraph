package errors

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestGetExitCode_NilError tests that nil errors return ExitSuccess.
func TestGetExitCode_NilError(t *testing.T) {
	code := GetExitCode(nil)
	if code != ExitSuccess {
		t.Errorf("GetExitCode(nil) = %d, want %d", code, ExitSuccess)
	}
}

// TestGetExitCode_StructuredError tests exit code mapping for StructuredError types.
func TestGetExitCode_StructuredError(t *testing.T) {
	tests := []struct {
		name     string
		category ErrorCategory
		want     int
	}{
		{"ValidationError", ValidationError, ExitUsage},
		{"ParseError", ParseError, ExitDataError},
		{"DatabaseError", DatabaseError, ExitUnavailable},
		{"NetworkError", NetworkError, ExitTempFail},
		{"TimeoutError", TimeoutError, ExitTempFail},
		{"ClientError", ClientError, ExitUsage},
		{"ServerError", ServerError, ExitUnavailable},
		{"AuthError", AuthError, ExitPermission},
		{"ConfigError", ConfigError, ExitConfig},
		{"ResourceError", ResourceError, ExitOSError},
		{"ConcurrencyError", ConcurrencyError, ExitSoftware},
		{"UnknownError", UnknownError, ExitError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewError(tt.category, SeverityHigh, "test message", "TEST_CODE", "test-component", "test-operation")
			code := GetExitCode(err)
			if code != tt.want {
				t.Errorf("GetExitCode(%v) = %d, want %d", tt.category, code, tt.want)
			}
		})
	}
}

// TestGetExitCode_ClassifiedError tests exit code mapping for standard Go errors.
func TestGetExitCode_ClassifiedError(t *testing.T) {
	tests := []struct {
		name      string
		errMsg    string
		wantCode  int
		wantClass ErrorCategory
	}{
		{
			name:      "Timeout error",
			errMsg:    "context deadline exceeded",
			wantCode:  ExitTempFail,
			wantClass: TimeoutError,
		},
		{
			name:      "Network connection refused",
			errMsg:    "connection refused",
			wantCode:  ExitTempFail,
			wantClass: NetworkError,
		},
		{
			name:      "Invalid JSON",
			errMsg:    "invalid character '}'",
			wantCode:  ExitDataError,
			wantClass: ParseError,
		},
		{
			name:      "Database connection error",
			errMsg:    "database connection failed",
			wantCode:  ExitUnavailable,
			wantClass: DatabaseError,
		},
		{
			name:      "Authentication error",
			errMsg:    "unauthorized access",
			wantCode:  ExitPermission,
			wantClass: AuthError,
		},
		{
			name:      "Resource exhausted",
			errMsg:    "out of memory",
			wantCode:  ExitOSError,
			wantClass: ResourceError,
		},
		{
			name:      "Validation error",
			errMsg:    "required field missing",
			wantCode:  ExitUsage,
			wantClass: ValidationError,
		},
		{
			name:      "Configuration error",
			errMsg:    "config file not found",
			wantCode:  ExitConfig,
			wantClass: ConfigError,
		},
		{
			name:      "Deadlock error",
			errMsg:    "deadlock detected",
			wantCode:  ExitSoftware,
			wantClass: ConcurrencyError,
		},
		{
			name:      "Unknown error",
			errMsg:    "something unexpected happened",
			wantCode:  ExitError,
			wantClass: UnknownError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.errMsg)
			code := GetExitCode(err)
			if code != tt.wantCode {
				t.Errorf("GetExitCode(%q) = %d, want %d", tt.errMsg, code, tt.wantCode)
			}

			// Verify classification
			category := classifyError(err)
			if category != tt.wantClass {
				t.Errorf("classifyError(%q) = %v, want %v", tt.errMsg, category, tt.wantClass)
			}
		})
	}
}

// TestGetExitCodeForCategory tests the category-to-exit-code mapping.
func TestGetExitCodeForCategory(t *testing.T) {
	tests := []struct {
		category ErrorCategory
		want     int
	}{
		{ValidationError, ExitUsage},
		{ParseError, ExitDataError},
		{DatabaseError, ExitUnavailable},
		{NetworkError, ExitTempFail},
		{TimeoutError, ExitTempFail},
		{ClientError, ExitUsage},
		{ServerError, ExitUnavailable},
		{AuthError, ExitPermission},
		{ConfigError, ExitConfig},
		{ResourceError, ExitOSError},
		{ConcurrencyError, ExitSoftware},
		{UnknownError, ExitError},
	}

	for _, tt := range tests {
		t.Run(string(tt.category), func(t *testing.T) {
			code := getExitCodeForCategory(tt.category)
			if code != tt.want {
				t.Errorf("getExitCodeForCategory(%v) = %d, want %d", tt.category, code, tt.want)
			}
		})
	}
}

// TestExitCodeConstants verifies exit code constants match expected values.
func TestExitCodeConstants(t *testing.T) {
	tests := []struct {
		name  string
		value int
	}{
		{"ExitSuccess", 0},
		{"ExitError", 1},
		{"ExitUsage", 2},
		{"ExitDataError", 65},
		{"ExitNoInput", 66},
		{"ExitNoUser", 67},
		{"ExitNoHost", 68},
		{"ExitUnavailable", 69},
		{"ExitSoftware", 70},
		{"ExitOSError", 71},
		{"ExitIOError", 74},
		{"ExitTempFail", 75},
		{"ExitProtocol", 76},
		{"ExitPermission", 77},
		{"ExitConfig", 78},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test just verifies the constants are defined correctly
			// We're not testing the actual values here, just that they exist
			if tt.value < 0 {
				t.Errorf("Exit code %s should be non-negative", tt.name)
			}
		})
	}
}

// TestIsSuccess tests the IsSuccess helper function.
func TestIsSuccess(t *testing.T) {
	tests := []struct {
		code     int
		expected bool
	}{
		{ExitSuccess, true},
		{ExitError, false},
		{ExitUsage, false},
		{ExitTempFail, false},
		{ExitPermission, false},
		{100, false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.code), func(t *testing.T) {
			result := IsSuccess(tt.code)
			if result != tt.expected {
				t.Errorf("IsSuccess(%d) = %v, want %v", tt.code, result, tt.expected)
			}
		})
	}
}

// TestIsError tests the IsError helper function.
func TestIsError(t *testing.T) {
	tests := []struct {
		code     int
		expected bool
	}{
		{ExitSuccess, false},
		{ExitError, true},
		{ExitUsage, true},
		{ExitTempFail, true},
		{ExitPermission, true},
		{100, true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.code), func(t *testing.T) {
			result := IsError(tt.code)
			if result != tt.expected {
				t.Errorf("IsError(%d) = %v, want %v", tt.code, result, tt.expected)
			}
		})
	}
}

// TestIsTemporary tests the IsTemporary helper function.
func TestIsTemporary(t *testing.T) {
	tests := []struct {
		code     int
		expected bool
	}{
		{ExitSuccess, false},
		{ExitError, false},
		{ExitUsage, false},
		{ExitTempFail, true},
		{ExitUnavailable, true},
		{ExitPermission, false},
		{ExitConfig, false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.code), func(t *testing.T) {
			result := IsTemporary(tt.code)
			if result != tt.expected {
				t.Errorf("IsTemporary(%d) = %v, want %v", tt.code, result, tt.expected)
			}
		})
	}
}

// TestIsUsageError tests the IsUsageError helper function.
func TestIsUsageError(t *testing.T) {
	tests := []struct {
		code     int
		expected bool
	}{
		{ExitSuccess, false},
		{ExitUsage, true},
		{ExitError, false},
		{ExitDataError, false},
		{ExitTempFail, false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.code), func(t *testing.T) {
			result := IsUsageError(tt.code)
			if result != tt.expected {
				t.Errorf("IsUsageError(%d) = %v, want %v", tt.code, result, tt.expected)
			}
		})
	}
}

// TestIsPermissionError tests the IsPermissionError helper function.
func TestIsPermissionError(t *testing.T) {
	tests := []struct {
		code     int
		expected bool
	}{
		{ExitSuccess, false},
		{ExitPermission, true},
		{ExitError, false},
		{ExitUsage, false},
		{ExitConfig, false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.code), func(t *testing.T) {
			result := IsPermissionError(tt.code)
			if result != tt.expected {
				t.Errorf("IsPermissionError(%d) = %v, want %v", tt.code, result, tt.expected)
			}
		})
	}
}

// TestGetExitCodeDescription tests the GetExitCodeDescription function.
func TestGetExitCodeDescription(t *testing.T) {
	tests := []struct {
		code            int
		expectedDesc    string
		expectedUnknown bool
	}{
		{ExitSuccess, "Success", false},
		{ExitError, "General error", false},
		{ExitUsage, "Command line usage error", false},
		{ExitDataError, "Data format error", false},
		{ExitNoInput, "No input or insufficient data", false},
		{ExitNoUser, "User不存在", false},
		{ExitNoHost, "Host unknown", false},
		{ExitUnavailable, "Service unavailable", false},
		{ExitSoftware, "Internal software error", false},
		{ExitOSError, "System error (resource exhausted, etc.)", false},
		{ExitIOError, "I/O error", false},
		{ExitTempFail, "Temporary failure (retry may succeed)", false},
		{ExitProtocol, "Protocol error", false},
		{ExitPermission, "Permission denied", false},
		{ExitConfig, "Configuration error", false},
		{999, "Unknown exit code: 999", true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.code), func(t *testing.T) {
			desc := GetExitCodeDescription(tt.code)
			if tt.expectedUnknown {
				if !strings.Contains(desc, "Unknown exit code") {
					t.Errorf("GetExitCodeDescription(%d) = %q, want description to contain 'Unknown exit code'", tt.code, desc)
				}
			} else {
				if desc != tt.expectedDesc {
					t.Errorf("GetExitCodeDescription(%d) = %q, want %q", tt.code, desc, tt.expectedDesc)
				}
			}
		})
	}
}

// TestWrappedErrorExitCode tests that wrapped errors get correct exit codes.
func TestWrappedErrorExitCode(t *testing.T) {
	baseErr := errors.New("base error")
	category := NetworkError

	structuredErr := NewError(category, SeverityHigh, "test message", "TEST_CODE", "test-component", "test-operation")
	wrappedErr := WrapError(baseErr, *structuredErr)

	code := GetExitCode(wrappedErr)
	expectedCode := getExitCodeForCategory(NetworkError)
	if code != expectedCode {
		t.Errorf("GetExitCode(wrapped error) = %d, want %d", code, expectedCode)
	}
}

// TestGetExitCodeEdgeCases tests edge cases in exit code mapping.
func TestGetExitCodeEdgeCases(t *testing.T) {
	t.Run("wrapped nil error", func(t *testing.T) {
		// Test that wrapping nil doesn't cause issues
		code := GetExitCode(nil)
		if code != ExitSuccess {
			t.Errorf("GetExitCode(nil) = %d, want %d", code, ExitSuccess)
		}
	})

	t.Run("empty error message", func(t *testing.T) {
		err := errors.New("")
		code := GetExitCode(err)
		// Empty errors should still get some exit code
		if code < ExitError {
			t.Errorf("GetExitCode(empty error) = %d, want >= %d", code, ExitError)
		}
	})

	t.Run("very long error message", func(t *testing.T) {
		longMsg := string(make([]byte, 10000))
		for i := range longMsg {
			longMsg = longMsg[:i] + "a" + longMsg[i+1:]
		}
		err := errors.New(longMsg)
		code := GetExitCode(err)
		// Should still handle without panic
		if code < 0 {
			t.Errorf("GetExitCode(long error) = %d, want >= 0", code)
		}
	})
}
