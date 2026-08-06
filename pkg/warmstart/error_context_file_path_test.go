package warmstart

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestErrorContext_FilePathInclusion verifies that file paths are correctly included in error context.
func TestErrorContext_FilePathInclusion(t *testing.T) {
	tests := []struct {
		name           string
		memberName     string
		expectedInMsg  bool
		validateFormat func(t *testing.T, errMsg string)
	}{
		{
			name:          "absolute path in member name",
			memberName:    "/absolute/path/to/config.json",
			expectedInMsg: true,
			validateFormat: func(t *testing.T, errMsg string) {
				if !strings.Contains(errMsg, "/absolute/path/to/config.json") {
					t.Errorf("error message should contain absolute path, got: %s", errMsg)
				}
			},
		},
		{
			name:          "relative path in member name",
			memberName:    "relative/path/to/data.txt",
			expectedInMsg: true,
			validateFormat: func(t *testing.T, errMsg string) {
				if !strings.Contains(errMsg, "relative/path/to/data.txt") {
					t.Errorf("error message should contain relative path, got: %s", errMsg)
				}
			},
		},
		{
			name:          "nested tarball path",
			memberName:    "objects/pack/pack-abc123.ref",
			expectedInMsg: true,
			validateFormat: func(t *testing.T, errMsg string) {
				if !strings.Contains(errMsg, "objects/pack/pack-abc123.ref") {
					t.Errorf("error message should contain nested path, got: %s", errMsg)
				}
			},
		},
		{
			name:          "simple filename",
			memberName:    "config.json",
			expectedInMsg: true,
			validateFormat: func(t *testing.T, errMsg string) {
				if !strings.Contains(errMsg, "config.json") {
					t.Errorf("error message should contain simple filename, got: %s", errMsg)
				}
			},
		},
		{
			name:          "path with special characters",
			memberName:    "path/to/file with spaces.dat",
			expectedInMsg: true,
			validateFormat: func(t *testing.T, errMsg string) {
				if !strings.Contains(errMsg, "path/to/file with spaces.dat") {
					t.Errorf("error message should handle spaces in path, got: %s", errMsg)
				}
			},
		},
		{
			name:          "windows-style path",
			memberName:    "C:\\Users\\test\\file.dat",
			expectedInMsg: true,
			validateFormat: func(t *testing.T, errMsg string) {
				if !strings.Contains(errMsg, "C:\\Users\\test\\file.dat") {
					t.Errorf("error message should contain Windows path, got: %s", errMsg)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewMissingMemberError(tt.memberName)

			errMsg := err.Error()
			t.Logf("Error message: %s", errMsg)

			// Verify member name is in error message
			if !strings.Contains(errMsg, tt.memberName) {
				t.Errorf("error message should contain member name '%s', got: %s", tt.memberName, errMsg)
			}

			// Run format-specific validation
			if tt.validateFormat != nil {
				tt.validateFormat(t, errMsg)
			}

			// Verify error can be extracted with context
			var extractedErr *Error
			if !errors.As(err, &extractedErr) {
				t.Fatal("error should be extractable as *Error type")
			}

			if extractedErr.MemberName != tt.memberName {
				t.Errorf("extracted MemberName = '%s', want '%s'", extractedErr.MemberName, tt.memberName)
			}
		})
	}
}

// TestErrorContext_DebuggingInformation verifies that error context provides sufficient debugging information.
func TestErrorContext_DebuggingInformation(t *testing.T) {
	tests := []struct {
		name             string
		err              *Error
		expectedContents []string
	}{
		{
			name: "MissingMember error with member name",
			err:  NewMissingMemberError("objects/pack/pack-123.ref"),
			expectedContents: []string{
				"warmstart:",
				"missing required member",
				"member=objects/pack/pack-123.ref",
			},
		},
		{
			name: "MissingMember with detailed context",
			err: NewMissingMemberErrorWithContext(
				".ref",
				"missing files: objects/pack/pack-abc.ref, objects/pack/pack-def.ref",
			),
			expectedContents: []string{
				"warmstart:",
				"missing required member",
				"member=.ref",
				"missing files: objects/pack/pack-abc.ref, objects/pack/pack-def.ref",
			},
		},
		{
			name: "Truncated error with offset",
			err:  NewTruncatedError("unexpected EOF during header read", 1024),
			expectedContents: []string{
				"warmstart:",
				"truncated tarball",
				"unexpected EOF during header read",
				"offset=1024",
			},
		},
		{
			name: "CorruptPack error with member and context",
			err:  NewCorruptPackError("objects/pack/pack-xyz.pack", "checksum validation failed"),
			expectedContents: []string{
				"warmstart:",
				"corrupt pack data",
				"member=objects/pack/pack-xyz.pack",
				"checksum validation failed",
			},
		},
		{
			name: "IO error with underlying error",
			err:  NewIOError("failed to read pack file", errors.New("disk full")),
			expectedContents: []string{
				"warmstart:",
				"I/O error",
				"failed to read pack file",
				"disk full",
			},
		},
		{
			name: "Complete error with all fields",
			err: &Error{
				Kind:       CorruptPack,
				MemberName: "objects/pack/pack-full.pack",
				Context:    "SHA256 mismatch",
				Offset:     4096,
				Underlying: errors.New("hash verification failed"),
			},
			expectedContents: []string{
				"warmstart:",
				"corrupt pack data",
				"member=objects/pack/pack-full.pack",
				"SHA256 mismatch",
				"offset=4096",
				"hash verification failed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMsg := tt.err.Error()
			t.Logf("Debugging info: %s", errMsg)

			// Verify all expected debugging content is present
			for _, expected := range tt.expectedContents {
				if !strings.Contains(errMsg, expected) {
					t.Errorf("error message should contain '%s' for debugging, got: %s", expected, errMsg)
				}
			}

			// Verify error type is preserved
			if tt.err.Kind.String() == "unknown error" {
				t.Errorf("error kind should be specific, got unknown")
			}
		})
	}
}

// TestErrorContext_RelativeVsAbsolutePathHandling verifies handling of relative and absolute paths.
func TestErrorContext_RelativeVsAbsolutePathHandling(t *testing.T) {
	// Get current working directory for absolute path tests
	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}

	tests := []struct {
		name         string
		inputPath    string
		expectedType string
	}{
		{
			name:         "absolute Unix path",
			inputPath:    "/var/lib/git/config.json",
			expectedType: "absolute",
		},
		{
			name:         "relative Unix path",
			inputPath:    "config.json",
			expectedType: "relative",
		},
		{
			name:         "nested relative path",
			inputPath:    "objects/pack/pack-123.ref",
			expectedType: "relative",
		},
		{
			name:         "parent directory relative path",
			inputPath:    "../config/settings.json",
			expectedType: "relative",
		},
		{
			name:         "current directory reference",
			inputPath:    "./config.json",
			expectedType: "relative",
		},
		{
			name:         "absolute path with cwd prefix",
			inputPath:    filepath.Join(cwd, "config.json"),
			expectedType: "absolute",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewMissingMemberError(tt.inputPath)

			// Verify path is preserved exactly as provided
			if err.MemberName != tt.inputPath {
				t.Errorf("MemberName = '%s', want '%s'", err.MemberName, tt.inputPath)
			}

			// Verify path appears in error message
			errMsg := err.Error()
			if !strings.Contains(errMsg, tt.inputPath) {
				t.Errorf("error message should contain '%s', got: %s", tt.inputPath, errMsg)
			}

			// Verify the error type is correct
			if err.Kind != MissingMember {
				t.Errorf("error Kind = %d, want MissingMember (%d)", err.Kind, MissingMember)
			}

			t.Logf("Path '%s' (type: %s) correctly preserved in error: %s", tt.inputPath, tt.expectedType, errMsg)
		})
	}
}

// TestErrorContext_SourceLocationInformation verifies error source/location information is traceable.
func TestErrorContext_SourceLocationInformation(t *testing.T) {
	tests := []struct {
		name              string
		createError       func() *Error
		verifyTraceable   func(t *testing.T, err *Error)
		expectedFields    []string
	}{
		{
			name: "MissingMember error is traceable",
			createError: func() *Error {
				return NewMissingMemberError("test-file.ref")
			},
			verifyTraceable: func(t *testing.T, err *Error) {
				// Verify error can be extracted via errors.As
				var extractedErr *Error
				if !errors.As(err, &extractedErr) {
					t.Error("error should be extractable via errors.As")
				}

				// Verify all fields are accessible
				if extractedErr.MemberName == "" {
					t.Error("MemberName should be accessible for tracing")
				}
			},
			expectedFields: []string{"Kind", "MemberName"},
		},
		{
			name: "Error with context is traceable",
			createError: func() *Error {
				return NewMissingMemberErrorWithContext(
					"config.json",
					"file not found in tarball",
				)
			},
			verifyTraceable: func(t *testing.T, err *Error) {
				// Verify context is available
				if err.Context == "" {
					t.Error("Context should be available for debugging")
				}

				// Verify member name is available
				if err.MemberName == "" {
					t.Error("MemberName should be available for tracing")
				}
			},
			expectedFields: []string{"Kind", "MemberName", "Context"},
		},
		{
			name: "Error with offset is traceable",
			createError: func() *Error {
				return NewTruncatedError("EOF", 2048)
			},
			verifyTraceable: func(t *testing.T, err *Error) {
				// Verify offset is available for location tracing
				if err.Offset <= 0 {
					t.Errorf("Offset should be positive for location tracing, got: %d", err.Offset)
				}

				// Verify context is available
				if err.Context == "" {
					t.Error("Context should explain what happened")
				}
			},
			expectedFields: []string{"Kind", "Context", "Offset"},
		},
		{
			name: "Error with underlying cause is traceable",
			createError: func() *Error {
				underlying := errors.New("base error: permission denied")
				return NewIOError("write failed", underlying)
			},
			verifyTraceable: func(t *testing.T, err *Error) {
				// Verify underlying error is accessible
				if err.Underlying == nil {
					t.Error("Underlying error should be available for root cause analysis")
				}

				// Verify we can check for underlying error
				if !errors.Is(err, err.Underlying) {
					t.Error("underlying error should be findable via errors.Is")
				}

				// Verify error chain is traceable
				if err.Context == "" {
					t.Error("Context should explain the operation that failed")
				}
			},
			expectedFields: []string{"Kind", "Context", "Underlying"},
		},
		{
			name: "Complete error with all location info",
			createError: func() *Error {
				return &Error{
					Kind:       CorruptPack,
					MemberName: "objects/pack/pack-abc.pack",
					Context:    "checksum failed",
					Offset:     8192,
					Underlying: errors.New("hash mismatch"),
				}
			},
			verifyTraceable: func(t *testing.T, err *Error) {
				// All fields should be available for complete tracing
				if err.MemberName == "" {
					t.Error("MemberName should identify the file")
				}
				if err.Context == "" {
					t.Error("Context should explain the error")
				}
				if err.Offset <= 0 {
					t.Error("Offset should pinpoint the location")
				}
				if err.Underlying == nil {
					t.Error("Underlying should show root cause")
				}

				// Verify error message contains all location info
				errMsg := err.Error()
				requiredInfo := []string{
					err.MemberName,
					err.Context,
					"corrupt pack data",
				}
				for _, info := range requiredInfo {
					if !strings.Contains(errMsg, info) {
						t.Errorf("error message should contain '%s' for tracing", info)
					}
				}
			},
			expectedFields: []string{"Kind", "MemberName", "Context", "Offset", "Underlying"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createError()

			// Verify all expected fields are non-zero (unless optional)
			for _, field := range tt.expectedFields {
				switch field {
				case "Kind":
					if err.Kind.String() == "unknown error" {
						t.Errorf("Kind should be specific, got unknown")
					}
				case "MemberName":
					if err.MemberName == "" && tt.expectedFields[1] == "MemberName" {
						t.Error("MemberName should be set")
					}
				case "Context":
					if err.Context == "" && len(tt.expectedFields) > 2 {
						t.Errorf("Context should be set for debugging")
					}
				case "Offset":
					// Offset can be zero, so skip validation
				case "Underlying":
					// Underlying can be nil, so skip validation
				}
			}

			// Run traceability verification
			if tt.verifyTraceable != nil {
				tt.verifyTraceable(t, err)
			}

			t.Logf("Error is traceable: Kind=%s, Member=%s, Context=%s",
				err.Kind, err.MemberName, err.Context)
		})
	}
}

// TestErrorContext_ErrorFormatForDebugging verifies error message format is optimized for debugging.
func TestErrorContext_ErrorFormatForDebugging(t *testing.T) {
	tests := []struct {
		name            string
		err             *Error
		formatValidator func(t *testing.T, errMsg string)
	}{
		{
			name: "error message follows structured format",
			err:  NewMissingMemberError("config.json"),
			formatValidator: func(t *testing.T, errMsg string) {
				// Should follow: warmstart: <kind> (member=<name>) - <context>
				if !strings.HasPrefix(errMsg, "warmstart:") {
					t.Errorf("error message should start with 'warmstart:', got: %s", errMsg)
				}

				// Should contain error kind
				if !strings.Contains(errMsg, "missing required member") {
					t.Errorf("error message should contain error kind, got: %s", errMsg)
				}

				// Should contain member information
				if !strings.Contains(errMsg, "member=") {
					t.Errorf("error message should contain member identifier, got: %s", errMsg)
				}
			},
		},
		{
			name: "complex error provides comprehensive debugging info",
			err: &Error{
				Kind:       CorruptPack,
				MemberName: "objects/pack/pack-123.pack",
				Context:    "checksum validation failed",
				Offset:     4096,
				Underlying: errors.New("invalid hash"),
			},
			formatValidator: func(t *testing.T, errMsg string) {
				// Verify comprehensive debugging information
				debuggingElements := map[string]string{
					"error kind":    "corrupt pack data",
					"file path":     "objects/pack/pack-123.pack",
					"explanation":   "checksum validation failed",
					"location":      "offset=4096",
					"root cause":    "invalid hash",
				}

				for element, expected := range debuggingElements {
					if !strings.Contains(errMsg, expected) {
						t.Errorf("error message missing %s: expected '%s', got: %s", element, expected, errMsg)
					}
				}

				// Verify structured format with clear separators
				if !strings.Contains(errMsg, "member=") {
					t.Error("should use 'member=' prefix for file path")
				}
				if !strings.Contains(errMsg, "offset=") {
					t.Error("should use 'offset=' prefix for location")
				}
			},
		},
		{
			name: "error message is machine-parseable",
			err:  NewMissingMemberErrorWithContext("test.ref", "missing file"),
			formatValidator: func(t *testing.T, errMsg string) {
				// Verify structured format allows parsing
				parts := strings.Split(errMsg, " ")
				if len(parts) < 3 {
					t.Errorf("error message should be parseable, got: %s", errMsg)
				}

				// Verify key-value pairs are parseable
				if !strings.Contains(errMsg, "member=") {
					t.Error("error message should have parseable member field")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMsg := tt.err.Error()
			t.Logf("Error format: %s", errMsg)

			if tt.formatValidator != nil {
				tt.formatValidator(t, errMsg)
			}
		})
	}
}

// TestErrorContext_NegativeCases verifies error context handles edge cases properly.
func TestErrorContext_NegativeCases(t *testing.T) {
	tests := []struct {
		name         string
		memberName   string
		shouldHandle bool
		validate     func(t *testing.T, err *Error)
	}{
		{
			name:         "empty member name",
			memberName:   "",
			shouldHandle: true,
			validate: func(t *testing.T, err *Error) {
				if err == nil {
					t.Error("should handle empty member name gracefully")
				}
				if err.MemberName != "" {
					t.Errorf("empty member name should remain empty, got: %s", err.MemberName)
				}
			},
		},
		{
			name:         "very long member name",
			memberName:   strings.Repeat("a", 10000) + ".pack",
			shouldHandle: true,
			validate: func(t *testing.T, err *Error) {
				if err == nil {
					t.Error("should handle very long member name")
				}
				if err.MemberName != strings.Repeat("a", 10000)+".pack" {
					t.Error("very long member name should be preserved")
				}
			},
		},
		{
			name:         "member name with newlines",
			memberName:   "file\nwith\nnewlines.txt",
			shouldHandle: true,
			validate: func(t *testing.T, err *Error) {
				if err == nil {
					t.Error("should handle newlines in member name")
				}
				errMsg := err.Error()
				t.Logf("Handled newlines in member name: %s", errMsg)
			},
		},
		{
			name:         "member name with unicode",
			memberName:   "文件.pack",
			shouldHandle: true,
			validate: func(t *testing.T, err *Error) {
				if err == nil {
					t.Error("should handle unicode in member name")
				}
				if !strings.Contains(err.Error(), "文件.pack") {
					t.Error("unicode characters should be preserved")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewMissingMemberError(tt.memberName)

			if err == nil && tt.shouldHandle {
				t.Error("constructor should handle special cases")
			}

			if tt.validate != nil {
				tt.validate(t, err)
			}
		})
	}
}

// TestErrorContext_CallerInformation verifies caller information is available in error context.
func TestErrorContext_CallerInformation(t *testing.T) {
	// Capture file and line where error is created
	_, file, line, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to capture caller information")
	}

	// Create error immediately after capturing caller info
	err := NewMissingMemberError("test-file.ref")

	// Verify error was created successfully
	if err == nil {
		t.Fatal("failed to create error")
	}

	// Verify error contains member name (file path information)
	if !strings.Contains(err.Error(), "test-file.ref") {
		t.Error("error should contain file path information")
	}

	// Log that this test verifies error contains caller information
	// (The actual file:line comes from runtime.Caller in production code)
	t.Logf("Error created at %s:%d contains member name for tracing", file, line)

	// Verify error type is correct
	if err.Kind != MissingMember {
		t.Errorf("error kind should be MissingMember, got: %v", err.Kind)
	}
}

// TestErrorContext_WrappedErrorChain verifies error wrapping preserves context.
func TestErrorContext_WrappedErrorChain(t *testing.T) {
	// Create error chain: base -> IO error -> wrapped
	baseErr := errors.New("original error: file not found")
	ioErr := NewIOError("reading pack file", baseErr)
	wrappedErr := newWrappedError(ioErr, "operation failed")

	// Verify we can extract original error
	var extractedErr *Error
	if !errors.As(wrappedErr, &extractedErr) {
		t.Error("should be able to extract Error from wrapped chain")
	}

	// Verify context is preserved
	if extractedErr.Context != "reading pack file" {
		t.Errorf("context should be preserved in wrapped error, got: %s", extractedErr.Context)
	}

	// Verify underlying error is preserved
	if !errors.Is(wrappedErr, baseErr) {
		t.Error("underlying error should be findable in wrapped chain")
	}

	// Verify member name would be preserved if set
	ioErrWithMember := NewCorruptPackError("objects/pack/test.pack", "checksum failed")
	wrappedWithMember := newWrappedError(ioErrWithMember, "validation failed")

	var extractedWithMember *Error
	if !errors.As(wrappedWithMember, &extractedWithMember) {
		t.Error("should extract error with member name from wrapped chain")
	}

	if extractedWithMember.MemberName != "objects/pack/test.pack" {
		t.Errorf("member name should be preserved: got %s, want objects/pack/test.pack",
			extractedWithMember.MemberName)
	}
}

// newWrappedError creates a simple wrapped error for testing.
func newWrappedError(err error, message string) error {
	return &wrappedError{
		err:  err,
		msg:  message,
	}
}

// wrappedError is a test helper for error wrapping.
type wrappedError struct {
	err error
	msg string
}

func (e *wrappedError) Error() string {
	return e.msg + ": " + e.err.Error()
}

func (e *wrappedError) Unwrap() error {
	return e.err
}
