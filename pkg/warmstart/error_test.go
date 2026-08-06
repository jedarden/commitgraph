package warmstart

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestErrorKind_String(t *testing.T) {
	cases := []struct {
		kind     ErrorKind
		expected string
	}{
		{Truncated, "truncated tarball"},
		{MissingMember, "missing required member"},
		{CorruptPack, "corrupt pack data"},
		{IO, "I/O error"},
		{Other, "other error"},
		{ErrorKind(99), "unknown error"},
	}

	for _, tc := range cases {
		t.Run(tc.expected, func(t *testing.T) {
			got := tc.kind.String()
			if got != tc.expected {
				t.Errorf("ErrorKind.String() = %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestError_Error(t *testing.T) {
	cases := []struct {
		name           string
		err            *Error
		containsSubstr []string
	}{
		{
			name: "basic error with context",
			err: &Error{
				Kind:    Truncated,
				Context: "unexpected EOF",
			},
			containsSubstr: []string{"truncated tarball", "unexpected EOF"},
		},
		{
			name: "error with member name",
			err: &Error{
				Kind:       MissingMember,
				MemberName: "config.json",
			},
			containsSubstr: []string{"missing required member", "member=config.json"},
		},
		{
			name: "error with offset",
			err: &Error{
				Kind:    Truncated,
				Context: "header incomplete",
				Offset:  1024,
			},
			containsSubstr: []string{"truncated tarball", "offset=1024", "header incomplete"},
		},
		{
			name: "error with underlying error",
			err: &Error{
				Kind:       IO,
				Context:    "failed to read",
				Underlying: errors.New("disk full"),
			},
			containsSubstr: []string{"I/O error", "failed to read", "disk full"},
		},
		{
			name: "complete error with all fields",
			err: &Error{
				Kind:       CorruptPack,
				Context:    "checksum mismatch",
				MemberName: "objects/pack/pack-123.pack",
				Offset:     4096,
				Underlying: errors.New("invalid hash"),
			},
			containsSubstr: []string{
				"corrupt pack data",
				"member=objects/pack/pack-123.pack",
				"offset=4096",
				"checksum mismatch",
				"invalid hash",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errMsg := tc.err.Error()
			for _, substr := range tc.containsSubstr {
				if !strings.Contains(errMsg, substr) {
					t.Errorf("Error message %q does not contain expected substring %q", errMsg, substr)
				}
			}
			t.Logf("Error message: %s", errMsg)
		})
	}
}

func TestError_Unwrap(t *testing.T) {
	underlying := errors.New("base error")
	err := &Error{
		Kind:       IO,
		Underlying: underlying,
	}

	if unwrapped := err.Unwrap(); unwrapped != underlying {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, underlying)
	}
}

func TestError_AsAndIs(t *testing.T) {
	underlying := errors.New("base error")
	err := &Error{
		Kind:       IO,
		Context:    "test",
		Underlying: underlying,
	}

	// Test errors.Is with underlying error
	if !errors.Is(err, underlying) {
		t.Error("errors.Is failed to find underlying error")
	}

	// Test errors.As
	var target *Error
	if !errors.As(err, &target) {
		t.Error("errors.As failed to extract Error type")
	}
	if target.Kind != IO {
		t.Errorf("errors.As extracted error with wrong Kind: got %v, want %v", target.Kind, IO)
	}
}

func TestCorruptionError(t *testing.T) {
	err := &CorruptionError{
		Context: "invalid ref format",
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "corrupt tarball") {
		t.Errorf("CorruptionError.Error() = %q, must contain 'corrupt tarball'", errMsg)
	}
	if !strings.Contains(errMsg, "invalid ref format") {
		t.Errorf("CorruptionError.Error() = %q, must contain context", errMsg)
	}
}

func TestNotAGitRepoError(t *testing.T) {
	cases := []struct {
		name    string
		err     *NotAGitRepoError
		contain []string
	}{
		{
			name: "with reason",
			err: &NotAGitRepoError{
				Path:   "/path/to/repo",
				Reason: "HEAD not found",
			},
			contain: []string{"not a git repository", "/path/to/repo", "HEAD not found"},
		},
		{
			name: "without reason",
			err: &NotAGitRepoError{
				Path: "/path/to/repo",
			},
			contain: []string{"not a git repository", "/path/to/repo"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errMsg := tc.err.Error()
			for _, substr := range tc.contain {
				if !strings.Contains(errMsg, substr) {
					t.Errorf("NotAGitRepoError.Error() = %q, must contain %q", errMsg, substr)
				}
			}
		})
	}
}

func TestNewIOError(t *testing.T) {
	baseErr := errors.New("disk read error")
	err := NewIOError("reading pack file", baseErr)

	if err.Kind != IO {
		t.Errorf("NewIOError() Kind = %v, want %v", err.Kind, IO)
	}
	if err.Context != "reading pack file" {
		t.Errorf("NewIOError() Context = %q, want %q", err.Context, "reading pack file")
	}
	if err.Underlying != baseErr {
		t.Errorf("NewIOError() Underlying = %v, want %v", err.Underlying, baseErr)
	}
	if !errors.Is(err, baseErr) {
		t.Error("NewIOError() should wrap base error for errors.Is")
	}
}

func TestNewTruncatedError(t *testing.T) {
	err := NewTruncatedError("unexpected EOF", 2048)

	if err.Kind != Truncated {
		t.Errorf("NewTruncatedError() Kind = %v, want %v", err.Kind, Truncated)
	}
	if err.Context != "unexpected EOF" {
		t.Errorf("NewTruncatedError() Context = %q, want %q", err.Context, "unexpected EOF")
	}
	if err.Offset != 2048 {
		t.Errorf("NewTruncatedError() Offset = %d, want %d", err.Offset, 2048)
	}
}

func TestNewMissingMemberError(t *testing.T) {
	err := NewMissingMemberError("config.json")

	if err.Kind != MissingMember {
		t.Errorf("NewMissingMemberError() Kind = %v, want %v", err.Kind, MissingMember)
	}
	if err.MemberName != "config.json" {
		t.Errorf("NewMissingMemberError() MemberName = %q, want %q", err.MemberName, "config.json")
	}
}

func TestNewCorruptPackError(t *testing.T) {
	err := NewCorruptPackError("objects/pack/pack-123.pack", "checksum failed")

	if err.Kind != CorruptPack {
		t.Errorf("NewCorruptPackError() Kind = %v, want %v", err.Kind, CorruptPack)
	}
	if err.MemberName != "objects/pack/pack-123.pack" {
		t.Errorf("NewCorruptPackError() MemberName = %q, want %q", err.MemberName, "objects/pack/pack-123.pack")
	}
	if err.Context != "checksum failed" {
		t.Errorf("NewCorruptPackError() Context = %q, want %q", err.Context, "checksum failed")
	}
}

func TestIsOsPermissionError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "os permission error",
			err:  os.ErrPermission,
			want: true,
		},
		{
			name: "wrapped permission error",
			err:  fmt.Errorf("write failed: %w", os.ErrPermission),
			want: true,
		},
		{
			name: "other error",
			err:  errors.New("not permission"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "IO error with permission denied",
			err:  NewIOError("write failed", errors.New("permission denied")),
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsOsPermissionError(tc.err)
			if got != tc.want {
				t.Errorf("IsOsPermissionError() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAsOSPermission(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "direct os.ErrPermission",
			err:  os.ErrPermission,
			want: true,
		},
		{
			name: "wrapped permission error",
			err:  NewIOError("write", os.ErrPermission),
			want: true,
		},
		{
			name: "deeply wrapped permission error",
			err:  fmt.Errorf("outer: %w", NewIOError("inner", os.ErrPermission)),
			want: true,
		},
		{
			name: "non-permission error",
			err:  errors.New("file not found"),
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := AsOSPermission(tc.err)
			if got != tc.want {
				t.Errorf("AsOSPermission() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestErrorFormattingExamples(t *testing.T) {
	examples := []struct {
		name     string
		err      error
		validate func(t *testing.T, msg string)
	}{
		{
			name: "truncated tarball error",
			err: NewTruncatedError("unexpected EOF during header read", 512),
			validate: func(t *testing.T, msg string) {
				if !strings.Contains(msg, "truncated tarball") {
					t.Error("must mention truncated tarball")
				}
				if !strings.Contains(msg, "unexpected EOF") {
					t.Error("must mention context")
				}
				if !strings.Contains(msg, "offset=512") {
					t.Error("must mention offset")
				}
			},
		},
		{
			name: "missing member error",
			err: NewMissingMemberError("config.json"),
			validate: func(t *testing.T, msg string) {
				if !strings.Contains(msg, "missing required member") {
					t.Error("must mention missing member")
				}
				if !strings.Contains(msg, "member=config.json") {
					t.Error("must mention member name")
				}
			},
		},
		{
			name: "corrupt pack error",
			err: NewCorruptPackError("objects/pack/pack-123.pack", "SHA256 checksum mismatch"),
			validate: func(t *testing.T, msg string) {
				if !strings.Contains(msg, "corrupt pack data") {
					t.Error("must mention corrupt pack")
				}
				if !strings.Contains(msg, "pack-123.pack") {
					t.Error("must mention pack file name")
				}
				if !strings.Contains(msg, "SHA256 checksum mismatch") {
					t.Error("must mention corruption context")
				}
			},
		},
		{
			name: "IO error with underlying",
			err: NewIOError("failed to write pack file", errors.New("disk full")),
			validate: func(t *testing.T, msg string) {
				if !strings.Contains(msg, "I/O error") {
					t.Error("must mention I/O error")
				}
				if !strings.Contains(msg, "failed to write") {
					t.Error("must mention operation context")
				}
				if !strings.Contains(msg, "disk full") {
					t.Error("must mention underlying error")
				}
			},
		},
	}

	for _, ex := range examples {
		t.Run(ex.name, func(t *testing.T) {
			msg := ex.err.Error()
			t.Logf("Example error message: %s", msg)
			ex.validate(t, msg)
		})
	}
}
