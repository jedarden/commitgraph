package errors

import (
	"fmt"
	"testing"
)

func TestErrorWithContextFields(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*StructuredError)
		expected string
	}{
		{
			name: "error with commit SHA",
			setup: func(e *StructuredError) {
				e.WithCommitSHA("abc123def456")
			},
			expected: "commit=abc123def456",
		},
		{
			name: "error with position",
			setup: func(e *StructuredError) {
				e.WithPosition(12345)
			},
			expected: "position=12345",
		},
		{
			name: "error with email",
			setup: func(e *StructuredError) {
				e.WithEmail("test@example.com")
			},
			expected: "email=test@example.com",
		},
		{
			name: "error with trace ID",
			setup: func(e *StructuredError) {
				e.WithTraceID("trace-123-456")
			},
			expected: "trace=trace-123-456",
		},
		{
			name: "error with record key",
			setup: func(e *StructuredError) {
				e.WithRecordKey("user:123")
			},
			expected: "record=user:123",
		},
		{
			name: "error with multiple context fields",
			setup: func(e *StructuredError) {
				e.WithCommitSHA("abc123").
					WithPosition(100).
					WithEmail("user@example.com")
			},
			expected: "commit=abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewError(
				ValidationError,
				SeverityHigh,
				"test error",
				"TEST_001",
				"test-component",
				"test-operation",
			)

			tt.setup(err)

			errStr := err.Error()
			if !contains(errStr, tt.expected) {
				t.Errorf("Error() = %q, want to contain %q", errStr, tt.expected)
			}

			// Verify nil-safe methods
			if nilErr := (*StructuredError)(nil); nilErr.WithCommitSHA("test") != nil {
				t.Error("WithCommitSHA on nil should return nil")
			}
		})
	}
}

func TestNewErrorWithContextOptions(t *testing.T) {
	err := NewError(
		ValidationError,
		SeverityHigh,
		"validation failed",
		"VAL_001",
		"validator",
		"check-email",
		WithCommitSHAOption("sha123"),
		WithEmailOption("user@example.com"),
		WithPositionOption(42),
		WithTraceIDOption("trace-abc"),
		WithRecordKeyOption("key:xyz"),
	)

	if err.CommitSHA != "sha123" {
		t.Errorf("CommitSHA = %q, want %q", err.CommitSHA, "sha123")
	}
	if err.Email != "user@example.com" {
		t.Errorf("Email = %q, want %q", err.Email, "user@example.com")
	}
	if err.Position != 42 {
		t.Errorf("Position = %d, want %d", err.Position, 42)
	}
	if err.TraceID != "trace-abc" {
		t.Errorf("TraceID = %q, want %q", err.TraceID, "trace-abc")
	}
	if err.RecordKey != "key:xyz" {
		t.Errorf("RecordKey = %q, want %q", err.RecordKey, "key:xyz")
	}

	errStr := err.Error()
	expectedParts := []string{"commit=sha123", "position=42", "email=user@example.com", "trace=trace-abc", "record=key:xyz"}
	for _, part := range expectedParts {
		if !contains(errStr, part) {
			t.Errorf("Error() = %q, want to contain %q", errStr, part)
		}
	}
}

func TestErrorContextChain(t *testing.T) {
	baseErr := fmt.Errorf("underlying error")

	err := WrapError(baseErr, *NewError(
		ParseError,
		SeverityMedium,
		"failed to parse commit data",
		"PARSE_001",
		"parser",
		"parse-commit",
	))

	// Test fluent chain with context fields
	err.WithCommitSHA("deadbeef").
		WithPosition(1024).
		WithEmail("author@example.com").
		WithTraceID("parent-trace-123").
		WithRecordKey("commits/deadbeef")

	if err.CommitSHA != "deadbeef" {
		t.Errorf("CommitSHA = %q, want %q", err.CommitSHA, "deadbeef")
	}
	if err.Position != 1024 {
		t.Errorf("Position = %d, want %d", err.Position, 1024)
	}
	if err.Email != "author@example.com" {
		t.Errorf("Email = %q, want %q", err.Email, "author@example.com")
	}
	if err.TraceID != "parent-trace-123" {
		t.Errorf("TraceID = %q, want %q", err.TraceID, "parent-trace-123")
	}
	if err.RecordKey != "commits/deadbeef" {
		t.Errorf("RecordKey = %q, want %q", err.RecordKey, "commits/deadbeef")
	}

	errStr := err.Error()
	if !contains(errStr, "commit=deadbeef") {
		t.Errorf("Error() = %q, want to contain %q", errStr, "commit=deadbeef")
	}
	if !contains(errStr, "position=1024") {
		t.Errorf("Error() = %q, want to contain %q", errStr, "position=1024")
	}
}

func TestErrorFormattingWithAndWithoutContext(t *testing.T) {
	// Error without context
	err1 := NewError(
		ValidationError,
		SeverityHigh,
		"test error",
		"TEST_001",
		"test",
		"op",
	)
	errStr1 := err1.Error()
	if contains(errStr1, "[commit=") || contains(errStr1, "[position=") {
		t.Errorf("Error without context should not contain context fields: %q", errStr1)
	}

	// Error with context
	err2 := NewError(
		ValidationError,
		SeverityHigh,
		"test error",
		"TEST_001",
		"test",
		"op",
	).WithCommitSHA("sha123").WithEmail("user@test.com")

	errStr2 := err2.Error()
	if !contains(errStr2, "[commit=sha123") {
		t.Errorf("Error with context should contain commit: %q", errStr2)
	}
	if !contains(errStr2, "email=user@test.com") {
		t.Errorf("Error with context should contain email: %q", errStr2)
	}
}
