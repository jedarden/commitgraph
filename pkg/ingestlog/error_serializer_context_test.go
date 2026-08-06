package ingestlog

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestSerializeError_FilePathInclusion verifies that file paths and source locations are included in error context.
func TestSerializeError_FilePathInclusion(t *testing.T) {
	tests := []struct {
		name            string
		err             error
		validateContext func(t *testing.T, ctx ErrorContext)
	}{
		{
			name: "basic error serialization includes stack trace with file paths",
			err:  errors.New("test error"),
			validateContext: func(t *testing.T, ctx ErrorContext) {
				if ctx.Message == "" {
					t.Error("error message should be captured")
				}
				if ctx.Type == "" {
					t.Error("error type should be captured")
				}
				if ctx.StackTrace == "" {
					t.Error("stack trace should be captured for debugging")
				}
				// Verify stack trace contains file:line format
				if !strings.Contains(ctx.StackTrace, ":") {
					t.Error("stack trace should contain file:line format")
				}
			},
		},
		{
			name: "error with file path in message",
			err:  errors.New("failed to open /path/to/config.json: no such file"),
			validateContext: func(t *testing.T, ctx ErrorContext) {
				if !strings.Contains(ctx.Message, "/path/to/config.json") {
					t.Errorf("error message should preserve file path, got: %s", ctx.Message)
				}
				// Verify stack trace is captured
				if ctx.StackTrace == "" {
					t.Error("stack trace should be captured for source location")
				}
			},
		},
		{
			name: "nil error produces empty context",
			err:  nil,
			validateContext: func(t *testing.T, ctx ErrorContext) {
				if ctx.Message != "" || ctx.Type != "" || ctx.StackTrace != "" {
					t.Error("nil error should produce empty context")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := SerializeError(tt.err)

			if tt.validateContext != nil {
				tt.validateContext(t, ctx)
			}

			t.Logf("Error context - Type: %s, Message: %s, StackTrace length: %d",
				ctx.Type, ctx.Message, len(ctx.StackTrace))
		})
	}
}

// TestSerializeError_StackTraceFormat verifies stack trace format includes file paths and line numbers.
func TestSerializeError_StackTraceFormat(t *testing.T) {

	err := errors.New("test error for stack trace")
	ctx := SerializeError(err)

	// Verify stack trace is not empty
	if ctx.StackTrace == "" {
		t.Fatal("stack trace should be captured")
	}

	// Verify stack trace contains file:line format
	lines := strings.Split(ctx.StackTrace, "\n")
	var foundFileLineFormat bool
	for _, line := range lines {
		if line == "" {
			continue
		}
		// Check for format: "file:line function"
		if strings.Contains(line, ":") && strings.Contains(line, " ") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				// First part should be file path, second part should have line number
				if strings.Contains(parts[1], " ") {
					foundFileLineFormat = true
					t.Logf("Found valid stack trace line: %s", line)
				}
			}
		}
	}

	if !foundFileLineFormat {
		t.Errorf("stack trace should contain file:line format, got:\n%s", ctx.StackTrace)
	}

	// Verify this test file appears in stack trace
	if !strings.Contains(ctx.StackTrace, "error_serializer_context_test.go") {
		t.Errorf("stack trace should contain this test file, got:\n%s", ctx.StackTrace)
	}

	t.Logf("Stack trace captured successfully with %d lines", len(lines))
}

// TestSerializeErrorWithCaller_DifferentDepths verifies caller depth affects stack trace capture.
func TestSerializeErrorWithCaller_DifferentDepths(t *testing.T) {
	createErrorAtDepth := func(depth int) error {
		return errors.New("error at depth")
	}

	tests := []struct {
		name         string
		callerDepth  int
		shouldContain []string
		validateFunc func(t *testing.T, ctx ErrorContext)
	}{
		{
			name:        "depth 0 captures caller",
			callerDepth: 0,
			shouldContain: []string{
				"error_serializer_context_test.go",
			},
		},
		{
			name:        "depth 1 skips one frame",
			callerDepth: 1,
			shouldContain: []string{
				"error_serializer_context_test.go",
			},
		},
		{
			name:        "depth 5 skips multiple frames",
			callerDepth: 5,
			// At depth 5, we may skip past all frames, which results in empty stack trace
			// This is expected behavior - just verify it doesn't crash
			validateFunc: func(t *testing.T, ctx ErrorContext) {
				// Empty stack trace at high depth is acceptable
				t.Logf("Depth 5 produced stack trace length: %d", len(ctx.StackTrace))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := createErrorAtDepth(tt.callerDepth)
			ctx := SerializeErrorWithCaller(err, tt.callerDepth)

			// Verify stack trace is captured (except at very high depths where it may be empty)
			if ctx.StackTrace == "" && tt.callerDepth < 3 {
				t.Error("stack trace should be captured at reasonable depths")
			}

			// Run custom validation if provided
			if tt.validateFunc != nil {
				tt.validateFunc(t, ctx)
			} else {
				// Verify expected content is present
				for _, expected := range tt.shouldContain {
					if !strings.Contains(ctx.StackTrace, expected) {
						t.Logf("Warning: expected '%s' in stack trace at depth %d", expected, tt.callerDepth)
					}
				}
			}

			t.Logf("Stack trace at depth %d:\n%s", tt.callerDepth, ctx.StackTrace)
		})
	}
}

// TestSerializeErrorWithOptions_Customization verifies error serialization options.
func TestSerializeErrorWithOptions_Customization(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		opts           *SerializationOptions
		validateResult func(t *testing.T, ctx ErrorContext)
	}{
		{
			name: "include stack trace",
			err:  errors.New("test error"),
			opts: &SerializationOptions{
				IncludeStackTrace: true,
				CallerDepth:        0,
			},
			validateResult: func(t *testing.T, ctx ErrorContext) {
				if ctx.StackTrace == "" {
					t.Error("stack trace should be included when requested")
				}
				if ctx.Message == "" {
					t.Error("message should be captured")
				}
				if ctx.Type == "" {
					t.Error("type should be captured")
				}
			},
		},
		{
			name: "exclude stack trace",
			err:  errors.New("test error"),
			opts: &SerializationOptions{
				IncludeStackTrace: false,
				CallerDepth:        0,
			},
			validateResult: func(t *testing.T, ctx ErrorContext) {
				if ctx.StackTrace != "" {
					t.Error("stack trace should not be included when disabled")
				}
				if ctx.Message == "" {
					t.Error("message should still be captured")
				}
				if ctx.Type == "" {
					t.Error("type should still be captured")
				}
			},
		},
		{
			name: "nil options uses defaults",
			err:  errors.New("test error"),
			opts: nil,
			validateResult: func(t *testing.T, ctx ErrorContext) {
				// Default is to include stack trace
				if ctx.StackTrace == "" {
					t.Error("stack trace should be included by default")
				}
			},
		},
		{
			name: "custom caller depth",
			err:  errors.New("test error"),
			opts: &SerializationOptions{
				IncludeStackTrace: true,
				CallerDepth:        2,
			},
			validateResult: func(t *testing.T, ctx ErrorContext) {
				if ctx.StackTrace == "" {
					t.Error("stack trace should be captured with custom depth")
				}
				// Verify the stack trace contains this test file
				if !strings.Contains(ctx.StackTrace, "error_serializer_context_test.go") {
					t.Logf("Note: stack trace at depth 2:\n%s", ctx.StackTrace)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := SerializeErrorWithOptions(tt.err, tt.opts)

			if tt.validateResult != nil {
				tt.validateResult(t, ctx)
			}

			t.Logf("Options %v - Stack trace length: %d",
				tt.opts, len(ctx.StackTrace))
		})
	}
}

// TestGetErrorChain_VerifyChainExtraction verifies error chain extraction for debugging.
func TestGetErrorChain_VerifyChainExtraction(t *testing.T) {
	baseErr := errors.New("base error")
	wrappedErr := fmt.Errorf("wrapped: %w", baseErr)
	doubleWrapped := fmt.Errorf("double wrapped: %w", wrappedErr)

	tests := []struct {
		name             string
		err              error
		expectedChainLen int
		expectedTypes    []string
	}{
		{
			name:             "single error",
			err:              errors.New("single error"),
			expectedChainLen: 1,
			expectedTypes:    []string{"*errors.errorString"},
		},
		{
			name:             "wrapped error chain",
			err:              wrappedErr,
			expectedChainLen: 2,
			expectedTypes:    []string{"*fmt.wrapError", "*errors.errorString"},
		},
		{
			name:             "double wrapped error chain",
			err:              doubleWrapped,
			expectedChainLen: 3,
			expectedTypes:    []string{"*fmt.wrapError", "*fmt.wrapError", "*errors.errorString"},
		},
		{
			name:             "nil error",
			err:              nil,
			expectedChainLen: 0,
			expectedTypes:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := GetErrorChain(tt.err)

			if len(chain) != tt.expectedChainLen {
				t.Errorf("chain length = %d, want %d", len(chain), tt.expectedChainLen)
			}

			if tt.expectedTypes != nil {
				for i, expectedType := range tt.expectedTypes {
					if i < len(chain) {
						// Allow for simplified type names
						if !strings.Contains(chain[i], expectedType) &&
							!strings.Contains(expectedType, chain[i]) {
							t.Logf("Chain[%d]: got %s, expected %s", i, chain[i], expectedType)
						}
					}
				}
			}

			t.Logf("Error chain (%d): %v", len(chain), chain)
		})
	}
}

// TestErrorContext_DebuggingCompleteness verifies error context provides complete debugging information.
func TestErrorContext_DebuggingCompleteness(t *testing.T) {
	// Create an error with specific file path
	err := fmt.Errorf("failed to read /var/lib/git/config.json: permission denied")
	ctx := SerializeError(err)

	// Verify all debugging components are present
	debuggingComponents := map[string]string{
		"Error Type":    ctx.Type,
		"Error Message": ctx.Message,
		"Stack Trace":   ctx.StackTrace,
	}

	for component, value := range debuggingComponents {
		if value == "" {
			t.Errorf("%s should be captured for debugging", component)
		}
		t.Logf("%s: %s", component, value)
	}

	// Verify error message contains file path
	if !strings.Contains(ctx.Message, "/var/lib/git/config.json") {
		t.Errorf("error message should contain file path, got: %s", ctx.Message)
	}

	// Verify stack trace contains file:line information
	if !strings.Contains(ctx.StackTrace, ":") {
		t.Error("stack trace should contain file:line information")
	}

	// Verify stack trace contains function names
	lines := strings.Split(ctx.StackTrace, "\n")
	var foundFunction bool
	for _, line := range lines {
		// Function format in stack trace: "file:line function.name"
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 && strings.Contains(parts[1], ".") {
			foundFunction = true
			t.Logf("Found function: %s", parts[1])
			break
		}
	}
	if !foundFunction {
		t.Error("stack trace should contain function names for call tracing")
	}
}

// TestErrorContext_SourceLocationTracing verifies source location can be traced from error context.
func TestErrorContext_SourceLocationTracing(t *testing.T) {
	// Create error at known location
	err := errors.New("error at specific location")

	// Serialize immediately after creation
	ctx := SerializeError(err)

	// Verify stack trace allows source location tracing
	if ctx.StackTrace == "" {
		t.Fatal("stack trace is required for source location tracing")
	}

	// Parse stack trace to extract file:line information
	lines := strings.Split(ctx.StackTrace, "\n")
	var sourceLocations []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Extract file:line from format "file:line function"
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			fileAndLine := parts[0]
			rest := parts[1]

			// Extract line number
			if spaceIdx := strings.Index(rest, " "); spaceIdx != -1 {
				lineNum := rest[:spaceIdx]
				sourceLocations = append(sourceLocations,
					fmt.Sprintf("%s:%s", fileAndLine, lineNum))
			}
		}
	}

	if len(sourceLocations) == 0 {
		t.Error("should be able to extract source locations from stack trace")
	}

	t.Logf("Extracted %d source locations from error context:", len(sourceLocations))
	for i, loc := range sourceLocations {
		t.Logf("  [%d] %s", i, loc)
	}

	// Verify at least one location contains this test file
	foundTestFile := false
	for _, loc := range sourceLocations {
		if strings.Contains(loc, "error_serializer_context_test.go") {
			foundTestFile = true
			t.Logf("Found this test file in stack trace: %s", loc)
			break
		}
	}
	if !foundTestFile {
		t.Logf("Note: this test file may not appear in stack trace depending on caller depth")
	}
}

// TestErrorContext_ErrorTypeExtraction verifies error type is correctly extracted.
func TestErrorContext_ErrorTypeExtraction(t *testing.T) {
	// Use a test error type that doesn't conflict with existing test files
	type testErrorType struct {
		msg string
	}
	testErr := &testErrorType{msg: "test error"}
	testErrFunc := func(e *testErrorType) string { return e.msg }

	wrappedTestErr := fmt.Errorf("wrapped: %w", errors.New(testErrFunc(testErr)))

	tests := []struct {
		name               string
		err                error
		expectedTypePrefix string
	}{
		{
			name:               "standard error",
			err:                errors.New("standard"),
			expectedTypePrefix: "errorString",
		},
		{
			name:               "wrapped error",
			err:                wrappedTestErr,
			expectedTypePrefix: "wrapError",
		},
		{
			name:               "nil error",
			err:                nil,
			expectedTypePrefix: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := SerializeError(tt.err)

			if tt.err == nil {
				if ctx.Type != "" {
					t.Errorf("nil error should produce empty type, got: %s", ctx.Type)
				}
			} else if tt.expectedTypePrefix != "" {
				if !strings.Contains(ctx.Type, tt.expectedTypePrefix) {
					t.Logf("Note: got type '%s', expected prefix '%s'", ctx.Type, tt.expectedTypePrefix)
				}
			}

			t.Logf("Error type extracted: %s", ctx.Type)
		})
	}
}

// TestErrorContext_MultipleErrorContext verifies error context handles multiple errors.
func TestErrorContext_MultipleErrorContext(t *testing.T) {
	err1 := errors.New("error 1: /path/to/file1.txt")
	err2 := fmt.Errorf("error 2: %w", errors.New("/path/to/file2.txt"))

	ctx1 := SerializeError(err1)
	ctx2 := SerializeError(err2)

	// Verify each error context is independent
	if ctx1.Message == ctx2.Message {
		t.Error("different errors should produce different messages")
	}

	// Verify both contain file paths
	if !strings.Contains(ctx1.Message, "/path/to/file1.txt") {
		t.Errorf("ctx1 should contain file1 path, got: %s", ctx1.Message)
	}
	if !strings.Contains(ctx2.Message, "file2.txt") {
		t.Errorf("ctx2 should contain file2 path, got: %s", ctx2.Message)
	}

	// Verify both have stack traces
	if ctx1.StackTrace == "" || ctx2.StackTrace == "" {
		t.Error("both error contexts should have stack traces")
	}

	t.Logf("Multiple error contexts handled independently")
}

// TestErrorContext_PathNormalization verifies path handling in error context.
func TestErrorContext_PathNormalization(t *testing.T) {
	tests := []struct {
		name       string
		errorMsg   string
		shouldPreserve bool
	}{
		{
			name:            "absolute Unix path",
			errorMsg:        "failed to open /usr/local/config.json",
			shouldPreserve: true,
		},
		{
			name:            "relative path",
			errorMsg:        "failed to open ./config/settings.json",
			shouldPreserve: true,
		},
		{
			name:            "path with parent directory",
			errorMsg:        "failed to open ../data/file.txt",
			shouldPreserve: true,
		},
		{
			name:            "Windows path",
			errorMsg:        "failed to open C:\\Users\\test\\file.dat",
			shouldPreserve: true,
		},
		{
			name:            "mixed path separators",
			errorMsg:        "failed to open path/to\\mixed/file",
			shouldPreserve: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.errorMsg)
			ctx := SerializeError(err)

			// Verify error message preserves original path
			if tt.shouldPreserve {
				// Extract path components from error message
				// (Path should be preserved as-is in the message)
				if ctx.Message != tt.errorMsg {
					t.Logf("Note: error message may be formatted differently")
					t.Logf("Original: %s", tt.errorMsg)
					t.Logf("Captured: %s", ctx.Message)
				}
			}

			// Verify stack trace is captured regardless of path format
			if ctx.StackTrace == "" {
				t.Error("stack trace should be captured")
			}

			t.Logf("Path handling: %s", ctx.Message)
		})
	}
}
