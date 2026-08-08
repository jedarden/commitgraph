package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestExtractKeyIDs verifies that extractKeyIDs correctly extracts key_id values from manifests
func TestExtractKeyIDs(t *testing.T) {
	tests := []struct {
		name     string
		manifest Manifest
		expected []string
	}{
		{
			name: "single key",
			manifest: Manifest{
				EncryptionKeys: []EncryptionKey{
					{KeyID: "epoch-2024-08-current"},
				},
			},
			expected: []string{"epoch-2024-08-current"},
		},
		{
			name: "multiple keys",
			manifest: Manifest{
				EncryptionKeys: []EncryptionKey{
					{KeyID: "epoch-2022-06-ancient"},
					{KeyID: "epoch-2023-12-retired"},
					{KeyID: "epoch-2024-08-current"},
				},
			},
			expected: []string{
				"epoch-2022-06-ancient",
				"epoch-2023-12-retired",
				"epoch-2024-08-current",
			},
		},
		{
			name:     "empty manifest",
			manifest: Manifest{EncryptionKeys: []EncryptionKey{}},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary manifest file
			tmpDir := t.TempDir()
			manifestPath := filepath.Join(tmpDir, "manifest.json")

			data, err := json.Marshal(tt.manifest)
			if err != nil {
				t.Fatalf("failed to marshal manifest: %v", err)
			}

			if err := os.WriteFile(manifestPath, data, 0644); err != nil {
				t.Fatalf("failed to write manifest file: %v", err)
			}

			// Extract key_ids
			keyIDs, err := extractKeyIDs(manifestPath)
			if err != nil {
				t.Fatalf("extractKeyIDs failed: %v", err)
			}

			// Verify results
			if len(keyIDs) != len(tt.expected) {
				t.Errorf("got %d key_ids, expected %d", len(keyIDs), len(tt.expected))
			}

			for i, keyID := range keyIDs {
				if keyID != tt.expected[i] {
					t.Errorf("key_id[%d]: got %s, expected %s", i, keyID, tt.expected[i])
				}
			}
		})
	}
}

// TestExtractKeyIDsFromSampleManifests verifies extraction from real fixture manifests
func TestExtractKeyIDsFromSampleManifests(t *testing.T) {
	fixtureDir := "../../testdata/fixtures/retired-epoch"

	tests := []struct {
		name             string
		manifestFile     string
		expectedKeyCount int
		expectedKeyIDs   []string
	}{
		{
			name:             "current epoch manifest",
			manifestFile:     "manifest-current-epoch.json",
			expectedKeyCount: 1,
			expectedKeyIDs:   []string{"epoch-2024-08-current"},
		},
		{
			name:             "retired epoch manifest",
			manifestFile:     "manifest-retired-epoch.json",
			expectedKeyCount: 1,
			expectedKeyIDs:   []string{"epoch-2023-12-retired"},
		},
		{
			name:             "multi-epoch manifest",
			manifestFile:     "manifest-multi-epoch.json",
			expectedKeyCount: 3,
			expectedKeyIDs: []string{
				"epoch-2022-06-ancient",
				"epoch-2023-12-retired",
				"epoch-2024-08-current",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifestPath := filepath.Join(fixtureDir, tt.manifestFile)

			keyIDs, err := extractKeyIDs(manifestPath)
			if err != nil {
				t.Fatalf("extractKeyIDs failed: %v", err)
			}

			if len(keyIDs) != tt.expectedKeyCount {
				t.Errorf("got %d key_ids, expected %d", len(keyIDs), tt.expectedKeyCount)
			}

			for i, keyID := range keyIDs {
				if keyID != tt.expectedKeyIDs[i] {
					t.Errorf("key_id[%d]: got %s, expected %s", i, keyID, tt.expectedKeyIDs[i])
				}
			}
		})
	}
}

// TestRunWithCorpus verifies the full run function with a test corpus
func TestRunWithCorpus(t *testing.T) {
	// Create a temporary corpus directory
	tmpDir := t.TempDir()
	corpusPath := filepath.Join(tmpDir, "corpus")
	outputPath := filepath.Join(tmpDir, "output.txt")

	if err := os.Mkdir(corpusPath, 0755); err != nil {
		t.Fatalf("failed to create corpus directory: %v", err)
	}

	// Create sample manifest files
	manifests := []struct {
		name     string
		manifest Manifest
	}{
		{
			name: "manifest1.json",
			manifest: Manifest{
				EncryptionKeys: []EncryptionKey{
					{KeyID: "key-1"},
					{KeyID: "key-2"},
				},
			},
		},
		{
			name: "manifest2.json",
			manifest: Manifest{
				EncryptionKeys: []EncryptionKey{
					{KeyID: "key-2"}, // duplicate
					{KeyID: "key-3"},
				},
			},
		},
		{
			name: "empty.json",
			manifest: Manifest{
				EncryptionKeys: []EncryptionKey{},
			},
		},
	}

	for _, m := range manifests {
		data, err := json.Marshal(m.manifest)
		if err != nil {
			t.Fatalf("failed to marshal manifest: %v", err)
		}

		path := filepath.Join(corpusPath, m.name)
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatalf("failed to write manifest file: %v", err)
		}
	}

	// Run the enumeration
	if err := run(corpusPath, outputPath); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// Verify output file was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Fatalf("output file was not created")
	}

	// Read and verify output
	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	outputStr := string(output)

	// Check for expected content
	expectedSubstrings := []string{
		"Manifests scanned: 3",
		"Unique key_ids found: 3",
		"key-1",
		"key-2",
		"key-3",
	}

	for _, expected := range expectedSubstrings {
		if !contains(outputStr, expected) {
			t.Errorf("output missing expected substring: %s", expected)
		}
	}
}

// contains is a helper to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
