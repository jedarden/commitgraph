package warmstart

import (
	"fmt"
	"regexp"
	"strings"
)

// Ref represents a single git reference with its path and target SHA.
// Ref captures both direct references (pointing to a commit SHA) and
// symbolic references (pointing to another reference).
type Ref struct {
	// Path is the reference path (e.g., "refs/heads/main", "refs/tags/v1.0", "HEAD").
	Path string

	// SHA is the 40-character hexadecimal SHA that the reference points to.
	// For symbolic refs, this may start with "ref:" (e.g., "ref: refs/heads/main").
	SHA string
}

// Validate checks that a Ref has valid path and SHA fields.
// Returns error if the ref path or SHA is malformed.
func (r *Ref) Validate() error {
	if r.Path == "" {
		return fmt.Errorf("%w: ref path is empty", ErrInvalidRefMetadata)
	}

	// Check if SHA is valid (either 40 hex chars or a symbolic ref)
	if r.SHA == "" {
		return fmt.Errorf("%w: ref SHA is empty for %s", ErrInvalidRefMetadata, r.Path)
	}

	// If it's a symbolic ref, allow it
	if strings.HasPrefix(r.SHA, "ref:") {
		target := strings.TrimSpace(strings.TrimPrefix(r.SHA, "ref:"))
		if target == "" {
			return fmt.Errorf("%w: symbolic ref has empty target for %s", ErrInvalidRefMetadata, r.Path)
		}
		return nil
	}

	// For direct refs, validate SHA format (40 hex characters)
	if !isValidSHA(r.SHA) {
		return fmt.Errorf("%w: invalid SHA format for %s: %s (expected 40 hex characters)", ErrInvalidRefMetadata, r.Path, r.SHA)
	}

	return nil
}

// IsSymbolic returns true if this is a symbolic reference (points to another ref).
func (r *Ref) IsSymbolic() bool {
	return strings.HasPrefix(r.SHA, "ref:")
}

// SymbolicTarget returns the target reference for symbolic refs.
// Returns empty string for direct refs.
func (r *Ref) SymbolicTarget() string {
	if !r.IsSymbolic() {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(r.SHA, "ref:"))
}

// isValidSHA checks if a string is a valid 40-character hexadecimal SHA.
func isValidSHA(sha string) bool {
	if len(sha) != 40 {
		return false
	}
	// SHA should be only hexadecimal characters
	matched, err := regexp.MatchString("^[0-9a-fA-F]{40}$", sha)
	if err != nil {
		return false
	}
	return matched
}

// ErrInvalidRefMetadata indicates ref metadata is malformed.
var ErrInvalidRefMetadata = fmt.Errorf("warmstart: invalid ref metadata")

// ParseRefMetadata parses ref metadata from tarball members.
// It extracts ref name → SHA mappings from tarball entries in both legacy and new formats.
//
// The function handles two tarball formats:
//   - Legacy format: A single "ref" file containing "refpath SHA" (e.g., "refs/heads/main abc123")
//   - New format: Multiple ref files at their original paths (e.g., "refs/heads/main", "refs/tags/v1.0")
//
// Parameters:
//   - members: Slice of TarballMember representing files in the tarball
//
// Returns:
//   - []Ref: Slice of parsed references (may be empty if no refs found)
//   - error: Error if metadata is malformed (e.g., invalid SHA, empty ref path)
//
// Supported ref types:
//   - refs/heads/* (branches)
//   - refs/tags/* (tags)
//   - refs/remotes/* (remote tracking branches)
//   - HEAD (symbolic reference to default branch)
//
// Example:
//   members := []TarballMember{
//       {Name: "refs/heads/main", Data: []byte("abc123...")},
//       {Name: "refs/tags/v1.0", Data: []byte("def456...")},
//   }
//   refs, err := ParseRefMetadata(members)
//   // refs would contain [{Path: "refs/heads/main", SHA: "abc123..."}, {Path: "refs/tags/v1.0", SHA: "def456..."}]
func ParseRefMetadata(members []TarballMember) ([]Ref, error) {
	var refs []Ref
	var foundLegacyRef bool

	for _, member := range members {
		switch member.Name {
		case "ref":
			// Legacy format: single file with "refpath SHA" content
			foundLegacyRef = true
			refParts := strings.TrimSpace(string(member.Data))
			if refParts == "" {
				return nil, fmt.Errorf("%w: empty ref data in legacy ref file", ErrInvalidRefMetadata)
			}

			parts := strings.Fields(refParts)
			if len(parts) != 2 {
				return nil, fmt.Errorf("%w: invalid legacy ref format: expected 'refpath SHA', got '%s'", ErrInvalidRefMetadata, refParts)
			}

			ref := Ref{
				Path: parts[0],
				SHA:  parts[1],
			}
			if err := ref.Validate(); err != nil {
				return nil, err
			}
			refs = append(refs, ref)

		default:
			// New format: ref at its original path
			// Handle standard ref locations
			if strings.HasPrefix(member.Name, "refs/heads/") ||
			   strings.HasPrefix(member.Name, "refs/tags/") ||
			   strings.HasPrefix(member.Name, "refs/remotes/") ||
			   member.Name == "HEAD" {

				// Skip directory entries (files should have content)
				if len(member.Data) == 0 {
					continue
				}

				ref := Ref{
					Path: member.Name,
					SHA:  strings.TrimSpace(string(member.Data)),
				}
				if err := ref.Validate(); err != nil {
					return nil, err
				}
				refs = append(refs, ref)
			}
		}
	}

	// If we found legacy ref, return only that (for backward compatibility)
	// This prevents mixing legacy and new formats
	if foundLegacyRef && len(refs) > 1 {
		// This shouldn't happen with well-formed tarballs, but check anyway
		return nil, fmt.Errorf("%w: mixed legacy and new ref formats detected", ErrInvalidRefMetadata)
	}

	return refs, nil
}

// RefsByType filters refs by their type (heads, tags, remotes, or other).
// Returns a new slice containing only refs of the specified type.
//
// Parameters:
//   - refs: Slice of refs to filter
//   - refType: Type of ref to filter ("heads", "tags", "remotes", or "" for all)
//
// Returns:
//   - []Ref: Filtered slice of refs
//
// Example:
//   heads := RefsByType(refs, "heads")  // All refs/heads/* refs
//   tags := RefsByType(refs, "tags")    // All refs/tags/* refs
func RefsByType(refs []Ref, refType string) []Ref {
	if refType == "" {
		return refs
	}

	var filtered []Ref
	prefix := "refs/" + refType + "/"
	for _, ref := range refs {
		if strings.HasPrefix(ref.Path, prefix) {
			filtered = append(filtered, ref)
		}
	}
	return filtered
}

// FindRef finds a ref by its path.
// Returns the ref and true if found, nil and false otherwise.
//
// Parameters:
//   - refs: Slice of refs to search
//   - path: Exact ref path to find (e.g., "refs/heads/main")
//
// Returns:
//   - *Ref: Pointer to the found ref (or nil if not found)
//   - bool: True if found, false otherwise
func FindRef(refs []Ref, path string) (*Ref, bool) {
	for i := range refs {
		if refs[i].Path == path {
			return &refs[i], true
		}
	}
	return nil, false
}
