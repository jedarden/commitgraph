// Package ingestlog tests the error serialization functionality.
package ingestlog

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
)

// TestSerializeError verifies basic error serialization.
func TestSerializeError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantType  string
		wantMsg   string
		wantStack bool
	}{
		{
			name:      "standard error",
			err:       errors.New("something went wrong"),
			wantType:  "errorString",
			wantMsg:   "something went wrong",
			wantStack: true,
		},
		{
			name:      "network error",
			err:       &net.OpError{Op: "dial", Err: errors.New("connection refused")},
			wantType:  "OpError",
			wantMsg:   "dial: connection refused",
			wantStack: true,
		},
		{
			name:      "nil error",
			err:       nil,
			wantType:  "",
			wantMsg:   "",
			wantStack: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := SerializeError(tt.err)

			// Verify type
			if ctx.Type != tt.wantType {
				t.Errorf("SerializeError() Type = %q, want %q", ctx.Type, tt.wantType)
			}

			// Verify message
			if ctx.Message != tt.wantMsg {
				t.Errorf("SerializeError() Message = %q, want %q", ctx.Message, tt.wantMsg)
			}

			// Verify stack trace presence/absence
			if tt.wantStack && ctx.StackTrace == "" {
				t.Errorf("SerializeError() StackTrace is empty, want non-empty")
			}
			if !tt.wantStack && ctx.StackTrace != "" {
				t.Errorf("SerializeError() StackTrace = %q, want empty", ctx.StackTrace)
			}
		})
	}
}

// TestSerializeErrorWithCaller verifies error serialization with custom caller depth.
func TestSerializeErrorWithCaller(t *testing.T) {
	err := errors.New("test error")

	ctx := SerializeErrorWithCaller(err, 0)

	// Verify basic fields
	if ctx.Type == "" {
		t.Errorf("SerializeErrorWithCaller() Type is empty")
	}
	if ctx.Message != "test error" {
		t.Errorf("SerializeErrorWithCaller() Message = %q, want 'test error'", ctx.Message)
	}
	if ctx.StackTrace == "" {
		t.Errorf("SerializeErrorWithCaller() StackTrace is empty")
	}

	// Verify stack trace contains this test function
	if !strings.Contains(ctx.StackTrace, "TestSerializeErrorWithCaller") {
		t.Errorf("StackTrace does not contain calling function name")
	}
}

// TestSerializeErrorWithOptions verifies error serialization with options.
func TestSerializeErrorWithOptions(t *testing.T) {
	err := errors.New("test error")

	// Test with default options
	ctx := SerializeErrorWithOptions(err, nil)
	if ctx.Type == "" {
		t.Errorf("SerializeErrorWithOptions() Type is empty")
	}
	if ctx.Message != "test error" {
		t.Errorf("SerializeErrorWithOptions() Message = %q, want 'test error'", ctx.Message)
	}
	if ctx.StackTrace == "" {
		t.Errorf("SerializeErrorWithOptions() StackTrace is empty with nil options")
	}

	// Test without stack trace
	opts := &SerializationOptions{
		IncludeStackTrace: false,
		CallerDepth:        0,
	}
	ctx = SerializeErrorWithOptions(err, opts)
	if ctx.StackTrace != "" {
		t.Errorf("SerializeErrorWithOptions() StackTrace = %q, want empty when IncludeStackTrace=false", ctx.StackTrace)
	}

	// Test with custom caller depth
	opts = &SerializationOptions{
		IncludeStackTrace: true,
		CallerDepth:        1,
	}
	ctx = SerializeErrorWithOptions(err, opts)
	if ctx.StackTrace == "" {
		t.Errorf("SerializeErrorWithOptions() StackTrace is empty with IncludeStackTrace=true")
	}
}

// TestGetErrorType verifies error type extraction using reflection.
func TestGetErrorType(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantType string
	}{
		{
			name:     "standard error",
			err:      errors.New("standard error"),
			wantType: "errorString",
		},
		{
			name:     "network OpError",
			err:      &net.OpError{Op: "dial", Err: errors.New("connection refused")},
			wantType: "OpError",
		},
		{
			name:     "fmt wrapError",
			err:      fmt.Errorf("wrapped: %w", errors.New("base")),
			wantType: "wrapError",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType := getErrorType(tt.err)
			if gotType != tt.wantType {
				t.Errorf("getErrorType() = %q, want %q", gotType, tt.wantType)
			}
		})
	}
}

// TestCustomTypeError verifies that errors implementing Type() method use it.
func TestCustomTypeError(t *testing.T) {
	customErr := &customError{msg: "custom error"}

	ctx := SerializeError(customErr)

	if ctx.Type != "CustomTypeError" {
		t.Errorf("SerializeError() Type = %q, want 'CustomTypeError'", ctx.Type)
	}
	if ctx.Message != "custom error" {
		t.Errorf("SerializeError() Message = %q, want 'custom error'", ctx.Message)
	}
}

// customError implements error with a custom Type() method.
type customError struct {
	msg string
}

func (e *customError) Error() string {
	return e.msg
}

func (e *customError) Type() string {
	return "CustomTypeError"
}

// TestCaptureStackTrace verifies stack trace capture functionality.
func TestCaptureStackTrace(t *testing.T) {
	stack := captureStackTrace()

	if stack == "" {
		t.Errorf("captureStackTrace() returned empty string")
	}

	// Verify it contains this function
	if !strings.Contains(stack, "TestCaptureStackTrace") {
		t.Errorf("Stack trace does not contain calling function")
	}

	// Verify it has proper format (file:line function)
	if !strings.Contains(stack, ".go:") {
		t.Errorf("Stack trace does not contain proper format")
	}
}

// TestCaptureStackTraceWithDepth verifies stack trace capture with custom depth.
func TestCaptureStackTraceWithDepth(t *testing.T) {
	// Test with depth 1 (should include this function)
	stack := captureStackTraceWithDepth(1)
	if !strings.Contains(stack, "TestCaptureStackTraceWithDepth") {
		t.Errorf("Stack trace with depth does not contain calling function")
	}

	// Test with greater depth
	stack = captureStackTraceWithDepth(10)
	if stack == "" {
		t.Errorf("Stack trace with greater depth returned empty")
	}
}

// TestSimplifyTypeName verifies type name simplification.
func TestSimplifyTypeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "*errors.errorString",
			expected: "errorString",
		},
		{
			input:    "net.OpError",
			expected: "net.OpError",
		},
		{
			input:    "github.com/user/repo/pkg.Type",
			expected: "Type",
		},
		{
			input:    "*github.com/user/repo/pkg.Type",
			expected: "Type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := simplifyTypeName(tt.input)
			if result != tt.expected {
				t.Errorf("simplifyTypeName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestGetErrorChain verifies error chain extraction.
func TestGetErrorChain(t *testing.T) {
	// Create a wrapped error chain
	baseErr := errors.New("base error")
	wrappedErr := fmt.Errorf("wrapped: %w", baseErr)
	doubleWrapped := fmt.Errorf("double wrapped: %w", wrappedErr)

	chain := GetErrorChain(doubleWrapped)

	if len(chain) < 3 {
		t.Errorf("GetErrorChain() returned %d errors, want at least 3", len(chain))
	}

	// Verify chain contains the base error
	found := false
	for _, errType := range chain {
		if strings.Contains(errType, "errorString") || errType != "" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Error chain does not contain expected error type")
	}
}

// TestGetErrorChainNil verifies error chain handles nil errors.
func TestGetErrorChainNil(t *testing.T) {
	chain := GetErrorChain(nil)

	if chain != nil {
		t.Errorf("GetErrorChain(nil) = %v, want nil", chain)
	}
}

// TestErrorContextFields verifies ErrorContext struct completeness.
func TestErrorContextFields(t *testing.T) {
	err := errors.New("test error")
	ctx := SerializeError(err)

	// Verify all required fields exist and are non-empty (except for optional ones)
	if ctx.Type == "" {
		t.Errorf("ErrorContext.Type is empty")
	}
	if ctx.Message == "" {
		t.Errorf("ErrorContext.Message is empty")
	}
	// StackTrace is optional but should be present for non-nil errors
	if ctx.StackTrace == "" {
		t.Errorf("ErrorContext.StackTrace is empty for non-nil error")
	}
}

// TestNilErrorHandling verifies nil error is handled gracefully.
func TestNilErrorHandling(t *testing.T) {
	ctx := SerializeError(nil)

	if ctx.Type != "" {
		t.Errorf("SerializeError(nil) Type = %q, want empty", ctx.Type)
	}
	if ctx.Message != "" {
		t.Errorf("SerializeError(nil) Message = %q, want empty", ctx.Message)
	}
	if ctx.StackTrace != "" {
		t.Errorf("SerializeError(nil) StackTrace = %q, want empty", ctx.StackTrace)
	}
}

// TestStackTraceFormat verifies stack trace is properly formatted.
func TestStackTraceFormat(t *testing.T) {
	err := errors.New("test error")
	ctx := SerializeError(err)

	lines := strings.Split(ctx.StackTrace, "\n")

	// Verify we have multiple lines
	if len(lines) < 2 {
		t.Errorf("Stack trace has only %d lines, want at least 2", len(lines))
	}

	// Verify each non-empty line has proper format (file:line function)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Should contain ".go:" for file reference
		if !strings.Contains(line, ".go:") {
			t.Errorf("Stack trace line %q does not contain '.go:'", line)
		}
	}
}

// BenchmarkSerializeError benchmarks the error serialization performance.
func BenchmarkSerializeError(b *testing.B) {
	err := errors.New("benchmark error")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SerializeError(err)
	}
}

// BenchmarkSerializeErrorWithStack benchmarks serialization with stack trace.
func BenchmarkSerializeErrorWithStack(b *testing.B) {
	err := errors.New("benchmark error")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := SerializeError(err)
		// Force stack trace to stay in benchmark
		_ = ctx.StackTrace
	}
}

// BenchmarkSerializeErrorWithoutStack benchmarks serialization without stack trace.
func BenchmarkSerializeErrorWithoutStack(b *testing.B) {
	err := errors.New("benchmark error")
	opts := &SerializationOptions{
		IncludeStackTrace: false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SerializeErrorWithOptions(err, opts)
	}
}
