package warmstart

import (
	"strings"
	"testing"
)

// TestIsValidSHA tests the isValidSHA helper function with various inputs.
func TestIsValidSHA(t *testing.T) {
	tests := []struct {
		name string
		sha  string
		want bool
	}{
		{
			name: "valid lowercase SHA",
			sha:  "abc123def456789abc123def456789abc123456767",
			want: true,
		},
		{
			name: "valid uppercase SHA",
			sha:  "ABC123DEF456789ABC123DEF456789ABC12345",
			want: true,
		},
		{
			name: "valid mixed case SHA",
			sha:  "AbC123DeF456789AbC123DeF456789AbC12345",
			want: true,
		},
		{
			name: "all zeros SHA",
			sha:  "0000000000000000000000000000000000000000",
			want: true,
		},
		{
			name: "all f's SHA",
			sha:  "ffffffffffffffffffffffffffffffffffffffff",
			want: true,
		},
		{
			name: "too short",
			sha:  "abc123",
			want: false,
		},
		{
			name: "too long",
			sha:  "abc123def456789abc123def456789abc1234567678",
			want: false,
		},
		{
			name: "contains invalid characters",
			sha:  "abc123def456789abc123def456789abc1234g",
			want: false,
		},
		{
			name: "contains special characters",
			sha:  "abc123def456789abc123def456789abc1234567!",
			want: false,
		},
		{
			name: "empty string",
			sha:  "",
			want: false,
		},
		{
			name: "with newline",
			sha:  "abc123def456789abc123def456789abc1234567\n",
			want: false,
		},
		{
			name: "with spaces",
			sha:  "abc123def456789abc123def456789abc1234567 ",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidSHA(tt.sha)
			if got != tt.want {
				t.Errorf("isValidSHA(%q) = %v, want %v", tt.sha, got, tt.want)
			}
		})
	}
}

// TestRefValidate tests the Ref.Validate method for various ref states.
func TestRefValidate(t *testing.T) {
	validSHA := "abc123def456789abc123def456789abc1234567"

	tests := []struct {
		name    string
		ref     Ref
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid direct ref",
			ref: Ref{
				Path: "refs/heads/main",
				SHA:  validSHA,
			},
			wantErr: false,
		},
		{
			name: "valid tag ref",
			ref: Ref{
				Path: "refs/tags/v1.0.0",
				SHA:  validSHA,
			},
			wantErr: false,
		},
		{
			name: "valid remote ref",
			ref: Ref{
				Path: "refs/remotes/origin/main",
				SHA:  validSHA,
			},
			wantErr: false,
		},
		{
			name: "valid HEAD ref",
			ref: Ref{
				Path: "HEAD",
				SHA:  validSHA,
			},
			wantErr: false,
		},
		{
			name: "valid symbolic ref",
			ref: Ref{
				Path: "HEAD",
				SHA:  "ref: refs/heads/main",
			},
			wantErr: false,
		},
		{
			name: "symbolic ref with spaces",
			ref: Ref{
				Path: "HEAD",
				SHA:  "ref:  refs/heads/main  ",
			},
			wantErr: false,
		},
		{
			name:    "empty path",
			ref: Ref{
				Path: "",
				SHA:  validSHA,
			},
			wantErr: true,
			errMsg:  "ref path is empty",
		},
		{
			name: "empty SHA",
			ref: Ref{
				Path: "refs/heads/main",
				SHA:  "",
			},
			wantErr: true,
			errMsg:  "ref SHA is empty",
		},
		{
			name: "invalid SHA format",
			ref: Ref{
				Path: "refs/heads/main",
				SHA:  "not-a-sha",
			},
			wantErr: true,
			errMsg:  "invalid SHA format",
		},
		{
			name: "SHA too short",
			ref: Ref{
				Path: "refs/heads/main",
				SHA:  "abc123",
			},
			wantErr: true,
			errMsg:  "invalid SHA format",
		},
		{
			name: "symbolic ref with empty target",
			ref: Ref{
				Path: "HEAD",
				SHA:  "ref:   ",
			},
			wantErr: true,
			errMsg:  "symbolic ref has empty target",
		},
		{
			name: "symbolic ref with just prefix",
			ref: Ref{
				Path: "HEAD",
				SHA:  "ref:",
			},
			wantErr: true,
			errMsg:  "symbolic ref has empty target",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ref.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Ref.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Error message should contain %q, got %q", tt.errMsg, err.Error())
				}
			}
		})
	}
}

// TestRefIsSymbolic tests the Ref.IsSymbolic method.
func TestRefIsSymbolic(t *testing.T) {
	tests := []struct {
		name string
		ref  Ref
		want bool
	}{
		{
			name: "direct ref",
			ref: Ref{
				Path: "refs/heads/main",
				SHA:  "abc123def456789abc123def456789abc1234567",
			},
			want: false,
		},
		{
			name: "symbolic ref",
			ref: Ref{
				Path: "HEAD",
				SHA:  "ref: refs/heads/main",
			},
			want: true,
		},
		{
			name: "symbolic ref with spaces",
			ref: Ref{
				Path: "HEAD",
				SHA:  "ref:  refs/heads/main  ",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ref.IsSymbolic()
			if got != tt.want {
				t.Errorf("Ref.IsSymbolic() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestRefSymbolicTarget tests the Ref.SymbolicTarget method.
func TestRefSymbolicTarget(t *testing.T) {
	tests := []struct {
		name string
		ref  Ref
		want string
	}{
		{
			name: "direct ref returns empty",
			ref: Ref{
				Path: "refs/heads/main",
				SHA:  "abc123def456789abc123def456789abc1234567",
			},
			want: "",
		},
		{
			name: "symbolic ref returns target",
			ref: Ref{
				Path: "HEAD",
				SHA:  "ref: refs/heads/main",
			},
			want: "refs/heads/main",
		},
		{
			name: "symbolic ref with spaces trims",
			ref: Ref{
				Path: "HEAD",
				SHA:  "ref:  refs/heads/main  ",
			},
			want: "refs/heads/main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ref.SymbolicTarget()
			if got != tt.want {
				t.Errorf("Ref.SymbolicTarget() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestParseRefMetadata_ValidInputs tests ParseRefMetadata with valid inputs.
func TestParseRefMetadata_ValidInputs(t *testing.T) {
	validSHA := "abc123def456789abc123def456789abc1234567"

	tests := []struct {
		name     string
		members  []TarballMember
		wantRefs []Ref
	}{
		{
			name: "legacy format single ref",
			members: []TarballMember{
				{Name: "ref", Data: []byte("refs/heads/main " + validSHA)},
			},
			wantRefs: []Ref{
				{Path: "refs/heads/main", SHA: validSHA},
			},
		},
		{
			name: "new format single head ref",
			members: []TarballMember{
				{Name: "refs/heads/main", Data: []byte(validSHA)},
			},
			wantRefs: []Ref{
				{Path: "refs/heads/main", SHA: validSHA},
			},
		},
		{
			name: "new format multiple head refs",
			members: []TarballMember{
				{Name: "refs/heads/main", Data: []byte(validSHA)},
				{Name: "refs/heads/develop", Data: []byte("def456789abc123def456789abc123def45678")},
			},
			wantRefs: []Ref{
				{Path: "refs/heads/main", SHA: validSHA},
				{Path: "refs/heads/develop", SHA: "def456789abc123def456789abc123def45678"},
			},
		},
		{
			name: "new format tag refs",
			members: []TarballMember{
				{Name: "refs/tags/v1.0", Data: []byte(validSHA)},
				{Name: "refs/tags/v2.0", Data: []byte("1111111111111111111111111111111111111111")},
			},
			wantRefs: []Ref{
				{Path: "refs/tags/v1.0", SHA: validSHA},
				{Path: "refs/tags/v2.0", SHA: "1111111111111111111111111111111111111111"},
			},
		},
		{
			name: "new format remote refs",
			members: []TarballMember{
				{Name: "refs/remotes/origin/main", Data: []byte(validSHA)},
				{Name: "refs/remotes/origin/develop", Data: []byte("2222222222222222222222222222222222222222")},
			},
			wantRefs: []Ref{
				{Path: "refs/remotes/origin/main", SHA: validSHA},
				{Path: "refs/remotes/origin/develop", SHA: "2222222222222222222222222222222222222222"},
			},
		},
		{
			name: "new format mixed ref types",
			members: []TarballMember{
				{Name: "refs/heads/main", Data: []byte(validSHA)},
				{Name: "refs/tags/v1.0", Data: []byte("3333333333333333333333333333333333333333")},
				{Name: "refs/remotes/origin/main", Data: []byte("4444444444444444444444444444444444444444")},
				{Name: "HEAD", Data: []byte("ref: refs/heads/main")},
			},
			wantRefs: []Ref{
				{Path: "refs/heads/main", SHA: validSHA},
				{Path: "refs/tags/v1.0", SHA: "3333333333333333333333333333333333333333"},
				{Path: "refs/remotes/origin/main", SHA: "4444444444444444444444444444444444444444"},
				{Path: "HEAD", SHA: "ref: refs/heads/main"},
			},
		},
		{
			name: "new format symbolic HEAD",
			members: []TarballMember{
				{Name: "HEAD", Data: []byte("ref: refs/heads/main")},
			},
			wantRefs: []Ref{
				{Path: "HEAD", SHA: "ref: refs/heads/main"},
			},
		},
		{
			name:    "empty members list",
			members: []TarballMember{},
			wantRefs: []Ref{},
		},
		{
			name: "ignores non-ref members",
			members: []TarballMember{
				{Name: "config.json", Data: []byte("{}")},
				{Name: "objects/pack/test.pack", Data: []byte("pack data")},
				{Name: "refs/heads/main", Data: []byte(validSHA)},
			},
			wantRefs: []Ref{
				{Path: "refs/heads/main", SHA: validSHA},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRefMetadata(tt.members)
			if err != nil {
				t.Fatalf("ParseRefMetadata() unexpected error: %v", err)
			}
			if len(got) != len(tt.wantRefs) {
				t.Fatalf("ParseRefMetadata() returned %d refs, want %d", len(got), len(tt.wantRefs))
			}
			for i := range tt.wantRefs {
				if got[i].Path != tt.wantRefs[i].Path {
					t.Errorf("Ref[%d].Path = %q, want %q", i, got[i].Path, tt.wantRefs[i].Path)
				}
				if got[i].SHA != tt.wantRefs[i].SHA {
					t.Errorf("Ref[%d].SHA = %q, want %q", i, got[i].SHA, tt.wantRefs[i].SHA)
				}
			}
		})
	}
}

// TestParseRefMetadata_MalformedInputs tests ParseRefMetadata error handling.
func TestParseRefMetadata_MalformedInputs(t *testing.T) {
	validSHA := "abc123def456789abc123def456789abc1234567"

	tests := []struct {
		name        string
		members     []TarballMember
		wantErr     bool
		errContains string
	}{
		{
			name: "legacy format empty data",
			members: []TarballMember{
				{Name: "ref", Data: []byte("")},
			},
			wantErr:     true,
			errContains: "empty ref data",
		},
		{
			name: "legacy format only refpath",
			members: []TarballMember{
				{Name: "ref", Data: []byte("refs/heads/main")},
			},
			wantErr:     true,
			errContains: "invalid legacy ref format",
		},
		{
			name: "legacy format three parts",
			members: []TarballMember{
				{Name: "ref", Data: []byte("refs/heads/main abc123 extra")},
			},
			wantErr:     true,
			errContains: "invalid legacy ref format",
		},
		{
			name: "legacy format invalid SHA",
			members: []TarballMember{
				{Name: "ref", Data: []byte("refs/heads/main not-a-sha")},
			},
			wantErr:     true,
			errContains: "invalid SHA format",
		},
		{
			name: "legacy format SHA too short",
			members: []TarballMember{
				{Name: "ref", Data: []byte("refs/heads/main abc123")},
			},
			wantErr:     true,
			errContains: "invalid SHA format",
		},
		{
			name: "new format empty ref path",
			members: []TarballMember{
				{Name: "", Data: []byte(validSHA)},
			},
			wantErr:     true,
			errContains: "ref path is empty",
		},
		{
			name: "new format empty SHA",
			members: []TarballMember{
				{Name: "refs/heads/main", Data: []byte("")},
			},
			wantErr:     true,
			errContains: "ref SHA is empty",
		},
		{
			name: "new format invalid SHA",
			members: []TarballMember{
				{Name: "refs/heads/main", Data: []byte("not-a-sha")},
			},
			wantErr:     true,
			errContains: "invalid SHA format",
		},
		{
			name: "new format SHA with newline",
			members: []TarballMember{
				{Name: "refs/heads/main", Data: []byte(validSHA + "\n")},
			},
			wantErr:     true,
			errContains: "invalid SHA format",
		},
		{
			name: "mixed legacy and new format",
			members: []TarballMember{
				{Name: "ref", Data: []byte("refs/heads/main " + validSHA)},
				{Name: "refs/heads/develop", Data: []byte(validSHA)},
			},
			wantErr:     true,
			errContains: "mixed legacy and new ref formats",
		},
		{
			name: "symbolic ref with empty target",
			members: []TarballMember{
				{Name: "HEAD", Data: []byte("ref:   ")},
			},
			wantErr:     true,
			errContains: "symbolic ref has empty target",
		},
		{
			name: "symbolic ref with just prefix",
			members: []TarballMember{
				{Name: "HEAD", Data: []byte("ref:")},
			},
			wantErr:     true,
			errContains: "symbolic ref has empty target",
		},
		{
			name: "new format handles trailing whitespace",
			members: []TarballMember{
				{Name: "refs/heads/main", Data: []byte(validSHA + "  ")},
			},
			wantErr: false,
		},
		{
			name: "new format handles leading whitespace",
			members: []TarballMember{
				{Name: "refs/heads/main", Data: []byte("  " + validSHA)},
			},
			wantErr: true, // Leading whitespace makes it invalid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseRefMetadata(tt.members)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRefMetadata() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errContains)
					return
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Error should contain %q, got %q", tt.errContains, err.Error())
				}
			}
		})
	}
}

// TestRefsByType tests the RefsByType filtering function.
func TestRefsByType(t *testing.T) {
	validSHA := "abc123def456789abc123def456789abc1234567"

	refs := []Ref{
		{Path: "refs/heads/main", SHA: validSHA},
		{Path: "refs/heads/develop", SHA: validSHA},
		{Path: "refs/tags/v1.0", SHA: validSHA},
		{Path: "refs/tags/v2.0", SHA: validSHA},
		{Path: "refs/remotes/origin/main", SHA: validSHA},
		{Path: "HEAD", SHA: "ref: refs/heads/main"},
	}

	tests := []struct {
		name    string
		refType string
		wantLen int
	}{
		{
			name:    "filter heads",
			refType: "heads",
			wantLen: 2,
		},
		{
			name:    "filter tags",
			refType: "tags",
			wantLen: 2,
		},
		{
			name:    "filter remotes",
			refType: "remotes",
			wantLen: 1,
		},
		{
			name:    "empty filter returns all",
			refType: "",
			wantLen: 6,
		},
		{
			name:    "unknown type returns empty",
			refType: "unknown",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RefsByType(refs, tt.refType)
			if len(got) != tt.wantLen {
				t.Errorf("RefsByType() returned %d refs, want %d", len(got), tt.wantLen)
			}
		})
	}
}

// TestFindRef tests the FindRef lookup function.
func TestFindRef(t *testing.T) {
	validSHA := "abc123def456789abc123def456789abc1234567"

	refs := []Ref{
		{Path: "refs/heads/main", SHA: validSHA},
		{Path: "refs/tags/v1.0", SHA: validSHA},
		{Path: "HEAD", SHA: "ref: refs/heads/main"},
	}

	tests := []struct {
		name    string
		path    string
		want    string // Expected SHA (or "NOT_FOUND")
	}{
		{
			name: "find existing head ref",
			path: "refs/heads/main",
			want: validSHA,
		},
		{
			name: "find existing tag ref",
			path: "refs/tags/v1.0",
			want: validSHA,
		},
		{
			name: "find HEAD",
			path: "HEAD",
			want: "ref: refs/heads/main",
		},
		{
			name: "find non-existent ref",
			path: "refs/heads/nonexistent",
			want: "NOT_FOUND",
		},
		{
			name: "find with wrong case",
			path: "refs/heads/Main",
			want: "NOT_FOUND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := FindRef(refs, tt.path)
			if tt.want == "NOT_FOUND" {
				if found {
					t.Errorf("FindRef() found %q, expected not found", tt.path)
				}
				if got != nil {
					t.Errorf("FindRef() returned non-nil ref for non-existent path")
				}
			} else {
				if !found {
					t.Errorf("FindRef() did not find %q", tt.path)
				}
				if got == nil {
					t.Errorf("FindRef() returned nil for existing ref")
				} else if got.SHA != tt.want {
					t.Errorf("FindRef().SHA = %q, want %q", got.SHA, tt.want)
				}
			}
		})
	}
}

// TestParseRefMetadata_RealWorldExamples tests with realistic git ref scenarios.
func TestParseRefMetadata_RealWorldExamples(t *testing.T) {
	tests := []struct {
		name        string
		members     []TarballMember
		wantHeadSHA string // SHA for HEAD (or symbolic target)
		wantTags    int    // Expected number of tags
	}{
		{
			name: "typical GitHub repository",
			members: []TarballMember{
				{Name: "HEAD", Data: []byte("ref: refs/heads/main")},
				{Name: "refs/heads/main", Data: []byte("a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2")},
				{Name: "refs/heads/develop", Data: []byte("f1e2d3c4b5a6f1e2d3c4b5a6f1e2d3c4b5a6f1e2")},
				{Name: "refs/tags/v1.0.0", Data: []byte("1234567890abcdef1234567890abcdef12345678")},
				{Name: "refs/tags/v2.0.0", Data: []byte("abcdef1234567890abcdef1234567890abcdef12")},
			},
			wantHeadSHA: "ref: refs/heads/main",
			wantTags:    2,
		},
		{
			name: "repository with annotated tags",
			members: []TarballMember{
				{Name: "refs/heads/main", Data: []byte("0000000000000000000000000000000000000000")},
				{Name: "refs/tags/v1.0", Data: []byte("1111111111111111111111111111111111111111")},
				{Name: "refs/tags/v1.0^{}", Data: []byte("2222222222222222222222222222222222222222")},
			},
			wantHeadSHA: "0000000000000000000000000000000000000000",
			wantTags:    2,
		},
		{
			name: "repository with remote tracking",
			members: []TarballMember{
				{Name: "refs/heads/main", Data: []byte("3333333333333333333333333333333333333333")},
				{Name: "refs/remotes/origin/HEAD", Data: []byte("ref: refs/remotes/origin/main")},
				{Name: "refs/remotes/origin/main", Data: []byte("3333333333333333333333333333333333333333")},
				{Name: "refs/remotes/upstream/main", Data: []byte("4444444444444444444444444444444444444444")},
			},
			wantHeadSHA: "3333333333333333333333333333333333333333",
			wantTags:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs, err := ParseRefMetadata(tt.members)
			if err != nil {
				t.Fatalf("ParseRefMetadata() unexpected error: %v", err)
			}

			// Check HEAD SHA
			headRef, found := FindRef(refs, "HEAD")
			if found && headRef.SHA != tt.wantHeadSHA {
				t.Errorf("HEAD SHA = %q, want %q", headRef.SHA, tt.wantHeadSHA)
			}

			// Check tag count
			tags := RefsByType(refs, "tags")
			if len(tags) != tt.wantTags {
				t.Errorf("Got %d tags, want %d", len(tags), tt.wantTags)
			}
		})
	}
}

// TestParseRefMetadata_SHAExtraction tests that SHAs are correctly extracted and validated.
func TestParseRefMetadata_SHAExtraction(t *testing.T) {
	// Test various valid SHA formats
	validSHAs := []struct {
		sha string
		desc string
	}{
		{"0000000000000000000000000000000000000000", "all zeros"},
		{"ffffffffffffffffffffffffffffffffffffffff", "all f's"},
		{"abc123def456789abc123def456789abc1234567", "lowercase"},
		{"ABC123DEF456789ABC123DEF456789ABC12345", "uppercase"},
		{"AbC123DeF456789AbC123DeF456789AbC12345", "mixed case"},
		{"1234567890abcdef1234567890abcdef12345678", "numeric start"},
		{"abcdef1234567890abcdef1234567890abcdef12", "alpha start"},
	}

	for _, tt := range validSHAs {
		t.Run(tt.desc, func(t *testing.T) {
			members := []TarballMember{
				{Name: "refs/heads/main", Data: []byte(tt.sha)},
			}
			refs, err := ParseRefMetadata(members)
			if err != nil {
				t.Fatalf("ParseRefMetadata() unexpected error: %v", err)
			}
			if len(refs) != 1 {
				t.Fatalf("ParseRefMetadata() returned %d refs, want 1", len(refs))
			}
			if refs[0].SHA != tt.sha {
				t.Errorf("SHA = %q, want %q", refs[0].SHA, tt.sha)
			}
		})
	}
}

// TestParseRefMetadata_RefNaming tests various ref path naming conventions.
func TestParseRefMetadata_RefNaming(t *testing.T) {
	validSHA := "abc123def456789abc123def456789abc1234567"

	testPaths := []struct {
		path string
		valid bool
	}{
		// Valid branch names
		{"refs/heads/main", true},
		{"refs/heads/master", true},
		{"refs/heads/develop", true},
		{"refs/heads/feature/new-feature", true},
		{"refs/heads/bugfix/fix-123", true},
		{"refs/heads/release/v1.0.0", true},

		// Valid tag names
		{"refs/tags/v1.0.0", true},
		{"refs/tags/v2.0-beta", true},
		{"refs/tags/1.0.0", true},

		// Valid remote names
		{"refs/remotes/origin/main", true},
		{"refs/remotes/upstream/master", true},
		{"refs/remotes/github/main", true},

		// Special refs
		{"HEAD", true},

		// Invalid paths (these should be rejected by validation if they have content)
		{"refs/heads/", false}, // Trailing slash would be caught as empty SHA
		{"refs/", false},       // Just namespace
	}

	for _, tt := range testPaths {
		t.Run(tt.path, func(t *testing.T) {
			members := []TarballMember{
				{Name: tt.path, Data: []byte(validSHA)},
			}
			refs, err := ParseRefMetadata(members)
			if tt.valid {
				if err != nil {
					t.Errorf("ParseRefMetadata() unexpected error for valid path %q: %v", tt.path, err)
				}
				if len(refs) != 1 {
					t.Errorf("ParseRefMetadata() returned %d refs for valid path %q, want 1", len(refs), tt.path)
				}
			} else {
				// Invalid paths should either be skipped or error
				if err == nil && len(refs) > 0 && refs[0].Path == tt.path {
					// If it didn't error, it might have been skipped (empty SHA for directory-like paths)
					if refs[0].SHA != "" {
						t.Errorf("ParseRefMetadata() accepted invalid path %q", tt.path)
					}
				}
			}
		})
	}
}
