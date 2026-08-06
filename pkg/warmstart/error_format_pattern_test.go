package warmstart

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestErrorFormatPattern_MissingMemberStructure verifies the MissingMember error structure matches expected pattern.
func TestErrorFormatPattern_MissingMemberStructure(t *testing.T) {
	err := NewMissingMemberError("objects/pack/pack-123.ref")

	// Verify it's a proper Error type
	var typedErr *Error
	if !errors.As(err, &typedErr) {
		t.Fatalf("expected error to be *Error type, got %T", err)
	}

	// Verify Kind field is MissingMember
	if typedErr.Kind != MissingMember {
		t.Errorf("expected Kind=MissingMember (%d), got %d", MissingMember, typedErr.Kind)
	}

	// Verify MemberName field is set
	if typedErr.MemberName != "objects/pack/pack-123.ref" {
		t.Errorf("expected MemberName='objects/pack/pack-123.ref', got '%s'", typedErr.MemberName)
	}

	// Verify Context field is empty for basic MissingMember error
	if typedErr.Context != "" {
		t.Errorf("expected empty Context for basic MissingMember error, got '%s'", typedErr.Context)
	}

	t.Logf("MissingMember error structure validated: %+v", typedErr)
}

// TestErrorFormatPattern_MissingMemberFieldTypes verifies error field types are correct.
func TestErrorFormatPattern_MissingMemberFieldTypes(t *testing.T) {
	err := NewMissingMemberErrorWithContext("test-member", "test context")

	// Verify Kind is ErrorKind type
	if kind := err.Kind; kind < 0 || kind > Other {
		t.Errorf("expected Kind to be valid ErrorKind (0-%d), got %d", Other, kind)
	}

	// Verify MemberName is string type and not nil
	var memberName string
	if _, ok := interface{}(err.MemberName).(string); !ok {
		t.Error("MemberName field should be string type")
	}
	memberName = err.MemberName
	if memberName != "test-member" {
		t.Errorf("MemberName = '%s', want 'test-member'", memberName)
	}

	// Verify Context is string type
	var context string
	if _, ok := interface{}(err.Context).(string); !ok {
		t.Error("Context field should be string type")
	}
	context = err.Context
	if context != "test context" {
		t.Errorf("Context = '%s', want 'test context'", context)
	}

	// Verify Offset is int64 type
	var offset int64
	offset = err.Offset
	if offset != 0 {
		t.Errorf("Offset = %d, want 0 for MissingMember error", offset)
	}

	// Verify Underlying is error type (can be nil)
	if err.Underlying != nil {
		t.Errorf("Underlying should be nil for MissingMember error, got %v", err.Underlying)
	}

	t.Logf("Field types validated: Kind=%d, MemberName='%s', Context='%s', Offset=%d",
		err.Kind, err.MemberName, err.Context, err.Offset)
}

// TestErrorFormatPattern_ParsableByConsumers verifies error can be properly parsed/handled by consumers.
func TestErrorFormatPattern_ParsableByConsumers(t *testing.T) {
	// Test with errors.As pattern
	err := NewMissingMemberError("config.json")

	// Consumer pattern 1: Extract Error type
	var parsedErr *Error
	if !errors.As(err, &parsedErr) {
		t.Error("consumer cannot extract Error type using errors.As")
	}

	// Verify extracted error has correct fields
	if parsedErr.Kind != MissingMember {
		t.Errorf("consumer extracted wrong Kind: got %d, want %d", parsedErr.Kind, MissingMember)
	}

	// Consumer pattern 2: Check error kind via field access
	if parsedErr.Kind != MissingMember {
		t.Error("consumer cannot check error Kind via field access")
	}

	// Consumer pattern 3: Extract member name
	if parsedErr.MemberName != "config.json" {
		t.Errorf("consumer extracted wrong MemberName: got '%s', want 'config.json'", parsedErr.MemberName)
	}

	// Consumer pattern 4: Use error in error interface
	var errInterface error = err
	if errInterface == nil {
		t.Error("error should not be nil when assigned to error interface")
	}

	// Consumer pattern 5: Get error message
	errMsg := errInterface.Error()
	if !strings.Contains(errMsg, "missing required member") {
		t.Errorf("error message should mention 'missing required member', got: %s", errMsg)
	}

	t.Logf("Consumer parsing test passed. Error message: %s", errMsg)
}

// TestErrorFormatPattern_NegativeTests_MalformedErrors tests negative cases with malformed errors.
func TestErrorFormatPattern_NegativeTests_MalformedErrors(t *testing.T) {
	t.Run("nil error handled gracefully", func(t *testing.T) {
		var err *Error
		if err != nil {
			t.Error("nil *Error should be nil")
		}
		if err != nil && err.Error() != "" {
			t.Error("should not call Error() on nil error")
		}
	})

	t.Run("empty MemberName produces valid error", func(t *testing.T) {
		err := NewMissingMemberError("")
		if err == nil {
			t.Fatal("constructor should not return nil for empty MemberName")
		}
		if err.MemberName != "" {
			t.Errorf("empty MemberName should remain empty, got '%s'", err.MemberName)
		}
		errMsg := err.Error()
		t.Logf("Empty MemberName error message: %s", errMsg)
	})

	t.Run("special characters in MemberName", func(t *testing.T) {
		specialNames := []string{
			"file with spaces.pack",
			"file'with'quotes.idx",
			"file\"with\"dquotes.ref",
			"file\nwith\nnewlines.txt",
			"file\twith\ttabs.dat",
		}
		for _, name := range specialNames {
			err := NewMissingMemberError(name)
			if err == nil {
				t.Errorf("constructor should handle special characters in '%s'", name)
			}
			errMsg := err.Error()
			t.Logf("Special character error for '%s': %s", name, errMsg)
		}
	})

	t.Run("very long MemberName", func(t *testing.T) {
		longName := strings.Repeat("a", 10000) + ".pack"
		err := NewMissingMemberError(longName)
		if err == nil {
			t.Error("constructor should handle very long MemberName")
		}
		if err.MemberName != longName {
			t.Error("very long MemberName should be preserved")
		}
		errMsg := err.Error()
		if len(errMsg) < len(longName) {
			t.Logf("Very long name truncated: original=%d, message=%d", len(longName), len(errMsg))
		}
	})

	t.Run("unicode characters in fields", func(t *testing.T) {
		err := NewMissingMemberErrorWithContext("文件.pack", "中文上下文")
		if err == nil {
			t.Fatal("constructor should handle unicode characters")
		}
		errMsg := err.Error()
		t.Logf("Unicode error message: %s", errMsg)
	})
}

// TestErrorFormatPattern_JSONSerialization verifies error can be serialized to JSON.
func TestErrorFormatPattern_JSONSerialization(t *testing.T) {
	err := NewMissingMemberErrorWithContext("test.ref", "missing files: a.ref, b.ref")

	// Test JSON marshaling
	data, jsonErr := json.Marshal(err)
	if jsonErr != nil {
		t.Fatalf("failed to marshal error to JSON: %v", jsonErr)
	}

	// Verify JSON contains expected fields
	jsonStr := string(data)
	expectedFields := []string{
		"Kind",
		"MemberName",
		"Context",
	}
	for _, field := range expectedFields {
		if !strings.Contains(jsonStr, field) {
			t.Errorf("JSON should contain field '%s', got: %s", field, jsonStr)
		}
	}

	// Verify values are present
	if !strings.Contains(jsonStr, "test.ref") {
		t.Errorf("JSON should contain MemberName value 'test.ref', got: %s", jsonStr)
	}

	t.Logf("JSON serialized error: %s", jsonStr)

	// Test JSON unmarshaling
	var unmarshaled Error
	unmarshalErr := json.Unmarshal(data, &unmarshaled)
	if unmarshalErr != nil {
		t.Fatalf("failed to unmarshal error from JSON: %v", unmarshalErr)
	}

	// Verify unmarshaled fields
	if unmarshaled.Kind != err.Kind {
		t.Errorf("unmarshaled Kind = %d, want %d", unmarshaled.Kind, err.Kind)
	}
	if unmarshaled.MemberName != err.MemberName {
		t.Errorf("unmarshaled MemberName = '%s', want '%s'", unmarshaled.MemberName, err.MemberName)
	}
	if unmarshaled.Context != err.Context {
		t.Errorf("unmarshaled Context = '%s', want '%s'", unmarshaled.Context, err.Context)
	}
}

// TestErrorFormatPattern_ErrorMessageFormat verifies error message format follows expected pattern.
func TestErrorFormatPattern_ErrorMessageFormat(t *testing.T) {
	tests := []struct {
		name          string
		err           *Error
		expectedParts []string
	}{
		{
			name: "basic MissingMember error",
			err:  NewMissingMemberError("config.json"),
			expectedParts: []string{
				"warmstart:",
				"missing required member",
				"member=config.json",
			},
		},
		{
			name: "MissingMember with context",
			err:  NewMissingMemberErrorWithContext(".ref", "missing files: a.ref, b.ref"),
			expectedParts: []string{
				"warmstart:",
				"missing required member",
				"member=.ref",
				"missing files: a.ref, b.ref",
			},
		},
		{
			name: "Truncated error with offset",
			err:  NewTruncatedError("unexpected EOF", 1024),
			expectedParts: []string{
				"warmstart:",
				"truncated tarball",
				"unexpected EOF",
				"offset=1024",
			},
		},
		{
			name: "CorruptPack error with member",
			err:  NewCorruptPackError("objects/pack/pack-123.pack", "checksum failed"),
			expectedParts: []string{
				"warmstart:",
				"corrupt pack data",
				"member=objects/pack/pack-123.pack",
				"checksum failed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMsg := tt.err.Error()

			for _, part := range tt.expectedParts {
				if !strings.Contains(errMsg, part) {
					t.Errorf("error message should contain '%s', got: %s", part, errMsg)
				}
			}

			// Verify format starts with "warmstart:"
			if !strings.HasPrefix(errMsg, "warmstart:") {
				t.Errorf("error message should start with 'warmstart:', got: %s", errMsg)
			}

			t.Logf("Error message format verified: %s", errMsg)
		})
	}
}

// TestErrorFormatPattern_ErrorKindValidation verifies all ErrorKind values produce valid strings.
func TestErrorFormatPattern_ErrorKindValidation(t *testing.T) {
	kinds := []ErrorKind{
		Truncated,
		MissingMember,
		CorruptPack,
		IO,
		Other,
	}

	for _, kind := range kinds {
		t.Run(kind.String(), func(t *testing.T) {
			// Create error with this kind
			err := &Error{
				Kind:       kind,
				MemberName: "test.file",
				Context:    "test context",
			}

			// Verify it produces a valid error message
			errMsg := err.Error()
			if errMsg == "" {
				t.Error("error message should not be empty")
			}

			// Verify message contains the kind string
			kindStr := kind.String()
			if !strings.Contains(errMsg, kindStr) {
				t.Errorf("error message should contain kind string '%s', got: %s", kindStr, errMsg)
			}

			t.Logf("ErrorKind %d (%s) produces valid message: %s", kind, kindStr, errMsg)
		})
	}
}

// TestErrorFormatPattern_CompleteErrorVerification verifies all error fields in a complete error.
func TestErrorFormatPattern_CompleteErrorVerification(t *testing.T) {
	baseErr := errors.New("underlying error")
	err := &Error{
		Kind:       CorruptPack,
		MemberName: "objects/pack/pack-abc123.pack",
		Context:    "checksum validation failed",
		Offset:     4096,
		Underlying: baseErr,
	}

	// Verify all fields are set
	if err.Kind != CorruptPack {
		t.Errorf("Kind = %d, want %d", err.Kind, CorruptPack)
	}
	if err.MemberName != "objects/pack/pack-abc123.pack" {
		t.Errorf("MemberName = '%s', want 'objects/pack/pack-abc123.pack'", err.MemberName)
	}
	if err.Context != "checksum validation failed" {
		t.Errorf("Context = '%s', want 'checksum validation failed'", err.Context)
	}
	if err.Offset != 4096 {
		t.Errorf("Offset = %d, want 4096", err.Offset)
	}
	if err.Underlying != baseErr {
		t.Error("Underlying should be baseErr")
	}

	// Verify error message includes all components
	errMsg := err.Error()
	expectedComponents := []string{
		"warmstart:",
		"corrupt pack data",
		"member=objects/pack/pack-abc123.pack",
		"checksum validation failed",
		"offset=4096",
		"underlying error",
	}

	for _, component := range expectedComponents {
		if !strings.Contains(errMsg, component) {
			t.Errorf("error message should contain '%s', got: %s", component, errMsg)
		}
	}

	t.Logf("Complete error verification passed: %s", errMsg)
}

// TestErrorFormatPattern_ErrorWrappingChain verifies error wrapping and unwrapping.
func TestErrorFormatPattern_ErrorWrappingChain(t *testing.T) {
	// Create a chain: base error -> IO error -> generic wrapper
	baseErr := errors.New("disk is full")
	ioErr := NewIOError("writing pack file", baseErr)
	wrappedErr := fmt.Errorf("operation failed: %w", ioErr)

	// Verify we can extract our Error type from the chain
	var extractedErr *Error
	if !errors.As(wrappedErr, &extractedErr) {
		t.Error("should be able to extract Error type from wrapped chain")
	}

	// Verify the extracted error has correct properties
	if extractedErr.Kind != IO {
		t.Errorf("extracted error Kind = %d, want %d", extractedErr.Kind, IO)
	}
	if extractedErr.Context != "writing pack file" {
		t.Errorf("extracted error Context = '%s', want 'writing pack file'", extractedErr.Context)
	}

	// Verify we can check for the base error in the chain
	if !errors.Is(wrappedErr, baseErr) {
		t.Error("should be able to find base error in wrapped chain")
	}

	t.Logf("Error wrapping chain verified: %v", wrappedErr)
}

// TestErrorFormatPattern_EdgeCases tests edge cases in error format.
func TestErrorFormatPattern_EdgeCases(t *testing.T) {
	t.Run("zero offset in error", func(t *testing.T) {
		err := &Error{
			Kind:    Truncated,
			Context: "test",
			Offset:  0,
		}
		errMsg := err.Error()
		// Zero offset should not appear in message
		if strings.Contains(errMsg, "offset=0") {
			t.Error("zero offset should not appear in error message")
		}
		t.Logf("Zero offset handled correctly: %s", errMsg)
	})

	t.Run("negative offset in error", func(t *testing.T) {
		err := &Error{
			Kind:    Truncated,
			Context: "test",
			Offset:  -1,
		}
		errMsg := err.Error()
		// Negative offset should not appear (logic uses > 0 check)
		if strings.Contains(errMsg, "offset=") {
			t.Error("negative offset should not appear in error message")
		}
		t.Logf("Negative offset handled correctly: %s", errMsg)
	})

	t.Run("empty context with underlying error", func(t *testing.T) {
		baseErr := errors.New("base")
		err := &Error{
			Kind:       IO,
			Underlying: baseErr,
		}
		errMsg := err.Error()
		if !strings.Contains(errMsg, "base") {
			t.Error("underlying error should appear even with empty context")
		}
		t.Logf("Empty context with underlying: %s", errMsg)
	})

	t.Run("all fields empty except Kind", func(t *testing.T) {
		err := &Error{
			Kind: Other,
		}
		errMsg := err.Error()
		expected := "warmstart: other error"
		if errMsg != expected {
			t.Errorf("error with minimal fields = '%s', want '%s'", errMsg, expected)
		}
		t.Logf("Minimal error: %s", errMsg)
	})
}
