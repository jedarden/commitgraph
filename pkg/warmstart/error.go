package warmstart

import (
	"fmt"
	"os"
	"strings"
)

// ErrorKind represents the category of error that occurred during tarball operations.
type ErrorKind int

const (
	// Truncated indicates the tarball was cut off or incomplete.
	Truncated ErrorKind = iota

	// MissingMember indicates a required tarball member was not found.
	MissingMember

	// CorruptPack indicates pack file data corruption was detected.
	CorruptPack

	// IO indicates an underlying input/output error occurred.
	IO

	// Other indicates an uncategorized error.
	Other
)

func (k ErrorKind) String() string {
	switch k {
	case Truncated:
		return "truncated tarball"
	case MissingMember:
		return "missing required member"
	case CorruptPack:
		return "corrupt pack data"
	case IO:
		return "I/O error"
	case Other:
		return "other error"
	default:
		return "unknown error"
	}
}

// Error is the structured error type for all tarball operation failures.
type Error struct {
	// Kind is the category of error.
	Kind ErrorKind

	// Context provides human-readable details about what went wrong.
	Context string

	// MemberName is the tarball member name (if applicable).
	MemberName string

	// Offset is the byte offset in the tarball (if applicable).
	Offset int64

	// Underlying is the original error (if applicable).
	Underlying error
}

// Error implements the error interface.
func (e *Error) Error() string {
	var details string

	if e.MemberName != "" {
		details = fmt.Sprintf(" (member=%s)", e.MemberName)
	}

	if e.Offset > 0 {
		if details == "" {
			details = fmt.Sprintf(" (offset=%d)", e.Offset)
		} else {
			details += fmt.Sprintf(", offset=%d", e.Offset)
		}
	}

	if e.Underlying != nil {
		if details == "" {
			details = fmt.Sprintf(": %v", e.Underlying)
		} else {
			details += fmt.Sprintf(": %v", e.Underlying)
		}
	}

	if e.Context != "" {
		if details == "" {
			details = fmt.Sprintf(": %s", e.Context)
		} else {
			details += fmt.Sprintf(" - %s", e.Context)
		}
	}

	return fmt.Sprintf("warmstart: %s%s", e.Kind, details)
}

// Unwrap returns the underlying error for errors.Is/As compatibility.
func (e *Error) Unwrap() error {
	return e.Underlying
}

// CorruptionError represents data corruption in the tarball.
// Deprecated: Use Error with Kind=CorruptPack or Kind=Truncated instead.
type CorruptionError struct {
	// Context describes what was corrupted.
	Context string
}

// Error implements the error interface.
func (e *CorruptionError) Error() string {
	return fmt.Sprintf("warmstart: corrupt tarball - %s", e.Context)
}

// NotAGitRepoError indicates the target directory is not a git repository.
// Deprecated: Use Error with Kind=Other instead.
type NotAGitRepoError struct {
	// Path is the directory that was checked.
	Path string

	// Reason explains why it's not a git repository.
	Reason string
}

// Error implements the error interface.
func (e *NotAGitRepoError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("warmstart: not a git repository at %s: %s", e.Path, e.Reason)
	}
	return fmt.Sprintf("warmstart: not a git repository at %s", e.Path)
}

// Is allows errors.Is to match NotAGitRepoError against ErrNotAGitRepo.
func (e *NotAGitRepoError) Is(target error) bool {
	return target == ErrNotAGitRepo
}

// NewIOError creates an Error with Kind=IO from an io error.
func NewIOError(context string, err error) *Error {
	return &Error{
		Kind:       IO,
		Context:    context,
		Underlying: err,
	}
}

// NewTruncatedError creates an Error with Kind=Truncated.
func NewTruncatedError(context string, offset int64) *Error {
	return &Error{
		Kind:    Truncated,
		Context: context,
		Offset:  offset,
	}
}

// NewTruncatedMemberError creates an Error with Kind=Truncated for a specific tarball member.
func NewTruncatedMemberError(memberName string, context string, offset int64) *Error {
	return &Error{
		Kind:       Truncated,
		MemberName: memberName,
		Context:    context,
		Offset:     offset,
	}
}

// NewMissingMemberError creates an Error with Kind=MissingMember.
func NewMissingMemberError(memberName string) *Error {
	return &Error{
		Kind:       MissingMember,
		MemberName: memberName,
	}
}

// NewMissingMemberErrorWithContext creates an Error with Kind=MissingMember and additional context.
// The context should provide human-readable details about what went wrong, such as a list of missing files.
func NewMissingMemberErrorWithContext(memberName string, context string) *Error {
	return &Error{
		Kind:       MissingMember,
		MemberName: memberName,
		Context:    context,
	}
}

// NewCorruptPackError creates an Error with Kind=CorruptPack.
func NewCorruptPackError(memberName string, context string) *Error {
	return &Error{
		Kind:       CorruptPack,
		MemberName: memberName,
		Context:    context,
	}
}

// IsOsPermissionError checks if an error is an OS permission error.
func IsOsPermissionError(err error) bool {
	if os.IsPermission(err) {
		return true
	}
	// Also check if error message contains permission denied
	if err != nil && strings.Contains(err.Error(), "permission denied") {
		return true
	}
	// Check if we've wrapped an OS permission error
	if err != nil && AsOSPermission(err) {
		return true
	}
	return false
}

// AsOSPermission checks if the error chain contains a permission error.
func AsOSPermission(err error) bool {
	for err != nil {
		if os.IsPermission(err) {
			return true
		}
		err = UnwrapError(err)
	}
	return false
}

// UnwrapError recursively unwraps errors to find underlying causes.
func UnwrapError(err error) error {
	switch u := err.(type) {
	case interface{ Unwrap() error }:
		return u.Unwrap()
	case interface{ Unwrap() []error }:
		// For errors that unwrap to multiple errors, check each
		for _, e := range u.Unwrap() {
			if e != nil {
				return e
			}
		}
		return nil
	default:
		return nil
	}
}
