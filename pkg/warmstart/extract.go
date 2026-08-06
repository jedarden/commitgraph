// Package warmstart implements materialization of git repository warm-start snapshots.
//
// A warm-start snapshot enables incremental fetch by restoring a repository's
// state from a previous scan, avoiding full re-clone. The artifact contains:
//   - Raw pack files (*.pack, *.idx, *.promisor, *.rev) from objects/pack/
//   - A single loose ref file (e.g., refs/heads/main)
//   - Three required git config values for partial clone support
//
// See docs/research/incremental-fetch-warm-start.md for empirical validation.
package warmstart

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrInvalidTarball indicates the tarball is corrupted or truncated.
	ErrInvalidTarball = errors.New("warmstart: invalid tarball")

	// ErrMissingPackFiles indicates required pack files are missing.
	ErrMissingPackFiles = errors.New("warmstart: missing required pack files")

	// ErrMissingRef indicates the ref file is missing.
	ErrMissingRef = errors.New("warmstart: missing ref file")

	// ErrMissingConfig indicates the git config is missing.
	ErrMissingConfig = errors.New("warmstart: missing git config")

	// ErrInvalidConfig indicates the git config is malformed.
	ErrInvalidConfig = errors.New("warmstart: invalid git config")

	// ErrNotAGitRepo indicates the target directory is not a git repository.
	ErrNotAGitRepo = errors.New("warmstart: not a git repository")
)

// Config represents the git configuration values required for warm-start.
type Config struct {
	// CoreRepositoryFormatVersion must be "1" for promisor packs.
	CoreRepositoryFormatVersion string `json:"core.repositoryformatversion"`

	// RemoteOriginPromisor enables promisor pack functionality.
	RemoteOriginPromisor string `json:"remote.origin.promisor"`

	// RemoteOriginPartialCloneFilter specifies the filter (e.g., "blob:none").
	RemoteOriginPartialCloneFilter string `json:"remote.origin.partialclonefilter"`
}

// Validate checks that all required config values are present and valid.
func (c *Config) Validate() error {
	if c.CoreRepositoryFormatVersion == "" {
		return fmt.Errorf("%w: missing core.repositoryformatversion", ErrInvalidConfig)
	}
	if c.RemoteOriginPromisor == "" {
		return fmt.Errorf("%w: missing remote.origin.promisor", ErrInvalidConfig)
	}
	if c.RemoteOriginPartialCloneFilter == "" {
		return fmt.Errorf("%w: missing remote.origin.partialclonefilter", ErrInvalidConfig)
	}
	return nil
}

// TarballMember represents a file in the warm-start tarball.
type TarballMember struct {
	Name string
	Data []byte
}

// WarmStartSnapshot represents the extracted warm-start data.
type WarmStartSnapshot struct {
	// PackFiles contains all pack-related files (.pack, .idx, .promisor, .rev).
	PackFiles []TarballMember

	// RefPath is the ref path (e.g., "refs/heads/main").
	RefPath string

	// RefSHA is the SHA the ref points to.
	RefSHA string

	// Config holds the git configuration.
	Config Config
}

// ParseTarball extracts and validates a warm-start tarball.
func ParseTarball(data []byte) (*WarmStartSnapshot, error) {
	tr := tar.NewReader(bytes.NewReader(data))

	snapshot := &WarmStartSnapshot{}
	var configData []byte
	var refData []byte
	var foundConfig, foundRef bool
	packExtensions := map[string]bool{
		".pack":    true,
		".idx":     true,
		".promisor": true,
		".rev":     true,
	}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidTarball, err)
		}

		// Read file content
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, tr); err != nil {
			return nil, fmt.Errorf("%w: failed to read %s: %v", ErrInvalidTarball, hdr.Name, err)
		}
		data := buf.Bytes()

		switch hdr.Name {
		case "config.json":
			foundConfig = true
			configData = data
		case "ref":
			foundRef = true
			refData = data
		default:
			// Check if it's a pack file (objects/pack/*.pack, etc.)
			if strings.HasPrefix(hdr.Name, "objects/pack/") {
				ext := filepath.Ext(hdr.Name)
				// Handle double extensions like .pack.promisor
				base := filepath.Base(hdr.Name)
				if packExtensions[ext] || strings.HasSuffix(base, ".promisor") || strings.HasSuffix(base, ".rev") {
					snapshot.PackFiles = append(snapshot.PackFiles, TarballMember{
						Name: hdr.Name,
						Data: data,
					})
				}
			}
		}
	}

	// Validate required components
	if !foundConfig {
		return nil, ErrMissingConfig
	}
	if !foundRef {
		return nil, ErrMissingRef
	}
	if len(snapshot.PackFiles) == 0 {
		return nil, ErrMissingPackFiles
	}

	// Parse config
	if err := json.Unmarshal(configData, &snapshot.Config); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}
	if err := snapshot.Config.Validate(); err != nil {
		return nil, err
	}

	// Parse ref (format: "refs/heads/main SHA")
	refParts := strings.TrimSpace(string(refData))
	if refParts == "" {
		return nil, fmt.Errorf("%w: empty ref data", ErrInvalidTarball)
	}
	parts := strings.Fields(refParts)
	if len(parts) != 2 {
		return nil, fmt.Errorf("%w: invalid ref format, expected 'refpath SHA'", ErrInvalidTarball)
	}
	snapshot.RefPath = parts[0]
	snapshot.RefSHA = parts[1]

	return snapshot, nil
}

// Materialize writes the warm-start snapshot to a git directory.
//
// The target directory must be an empty git repository (initialized with
// `git init --bare` or `git init`). After materialization, the repository
// will be ready for incremental fetch via `git fetch origin`.
func Materialize(gitDir string, snapshot *WarmStartSnapshot) error {
	// Verify target is a git directory
	headPath := filepath.Join(gitDir, "HEAD")
	if _, err := os.Stat(headPath); err != nil {
		return fmt.Errorf("%w: HEAD not found at %s", ErrNotAGitRepo, headPath)
	}

	// Create objects/pack directory
	packDir := filepath.Join(gitDir, "objects", "pack")
	if err := os.MkdirAll(packDir, 0755); err != nil {
		return fmt.Errorf("failed to create pack directory: %w", err)
	}

	// Write pack files byte-identical
	for _, pf := range snapshot.PackFiles {
		// Strip "objects/pack/" prefix if present
		targetName := strings.TrimPrefix(pf.Name, "objects/pack/")
		targetPath := filepath.Join(packDir, targetName)

		if err := os.WriteFile(targetPath, pf.Data, 0444); err != nil {
			return fmt.Errorf("failed to write pack file %s: %w", targetName, err)
		}
	}

	// Write loose ref file (NOT packed-refs)
	refDir := filepath.Dir(snapshot.RefPath)
	if refDir != "." {
		fullRefDir := filepath.Join(gitDir, refDir)
		if err := os.MkdirAll(fullRefDir, 0755); err != nil {
			return fmt.Errorf("failed to create ref directory: %w", err)
		}
	}
	refPath := filepath.Join(gitDir, snapshot.RefPath)
	if err := os.WriteFile(refPath, []byte(snapshot.RefSHA+"\n"), 0444); err != nil {
		return fmt.Errorf("failed to write ref file: %w", err)
	}

	// Write git config values
	configFile := filepath.Join(gitDir, "config")
	if err := writeGitConfig(configFile, &snapshot.Config); err != nil {
		return fmt.Errorf("failed to write git config: %w", err)
	}

	return nil
}

// writeGitConfig appends the three required promisor config values to git config.
func writeGitConfig(configPath string, config *Config) error {
	// Read existing config to check if we need to add sections
	existingConfig, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Parse existing config to determine what to write
	configStr := string(existingConfig)

	// Ensure core.repositoryformatversion is set
	if !strings.Contains(configStr, "repositoryformatversion") {
		configStr += fmt.Sprintf("\n[core]\n\trepositoryformatversion = %s\n", config.CoreRepositoryFormatVersion)
	}

	// Ensure remote.origin section exists with promisor and partialclonefilter
	needsRemote := !strings.Contains(configStr, "[remote \"origin\"]") ||
		!strings.Contains(configStr, "promisor") ||
		!strings.Contains(configStr, "partialclonefilter")

	if needsRemote {
		// Add [remote "origin"] section if missing or incomplete
		if !strings.Contains(configStr, "[remote \"origin\"]") {
			configStr += "\n[remote \"origin\"]\n"
		}
		configStr += fmt.Sprintf("\tpromisor = %s\n", config.RemoteOriginPromisor)
		configStr += fmt.Sprintf("\tpartialclonefilter = %s\n", config.RemoteOriginPartialCloneFilter)
	}

	return os.WriteFile(configPath, []byte(configStr), 0644)
}
