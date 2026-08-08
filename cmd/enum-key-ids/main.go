package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/jedarden/commitgraph/pkg/clierror"
)

// Manifest represents the structure of a manifest file
type Manifest struct {
	EncryptionKeys []EncryptionKey `json:"encryption_keys"`
}

// EncryptionKey represents a single encryption key entry in a manifest
type EncryptionKey struct {
	KeyID     string `json:"key_id"`
	Epoch     string `json:"epoch"`
	Status    string `json:"status"`
	KeyPath   string `json:"key_path"`
	CreatedAt string `json:"created_at"`
}

func main() {
	clierror.Run(run)
}

func run() error {
	corpusPath := flag.String("corpus", "", "Path to the corpus directory containing manifest files")
	outputPath := flag.String("output", "", "Optional path to write output file (default: stdout")
	flag.Parse()

	if *corpusPath == "" {
		return fmt.Errorf("--corpus path is required")
	}
	// Track unique key_ids
	keyIDSet := make(map[string]struct{})
	manifestCount := 0

	// Walk the corpus directory
	err := filepath.WalkDir(*corpusPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Process JSON files (manifest files)
		if filepath.Ext(path) == ".json" {
			manifestCount++
			keyIDs, err := extractKeyIDs(path)
			if err != nil {
				// Log error but continue processing other files
				fmt.Fprintf(os.Stderr, "Warning: failed to process %s: %v\n", path, err)
				return nil
			}

			// Add key_ids to the set
			for _, keyID := range keyIDs {
				keyIDSet[keyID] = struct{}{}
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("walking corpus: %w", err)
	}

	// Convert set to sorted slice
	keyIDs := make([]string, 0, len(keyIDSet))
	for keyID := range keyIDSet {
		keyIDs = append(keyIDs, keyID)
	}
	sort.Strings(keyIDs)

	// Prepare output
	output := fmt.Sprintf("Manifest Key ID Enumeration\n")
	output += fmt.Sprintf("===========================\n")
	output += fmt.Sprintf("Corpus path: %s\n", *corpusPath)
	output += fmt.Sprintf("Manifests scanned: %d\n", manifestCount)
	output += fmt.Sprintf("Unique key_ids found: %d\n", len(keyIDs))
	output += fmt.Sprintf("\nKey IDs:\n")
	for _, keyID := range keyIDs {
		output += fmt.Sprintf("  - %s\n", keyID)
	}

	// Write output
	if *outputPath == "" {
		fmt.Println(output)
	} else {
		if err := os.WriteFile(*outputPath, []byte(output), 0644); err != nil {
			return fmt.Errorf("writing output file: %w", err)
		}
		fmt.Printf("Output written to: %s\n", *outputPath)
	}

	return nil
}

// extractKeyIDs reads a manifest file and extracts all key_id values
func extractKeyIDs(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}

	keyIDs := make([]string, len(manifest.EncryptionKeys))
	for i, key := range manifest.EncryptionKeys {
		keyIDs[i] = key.KeyID
	}

	return keyIDs, nil
}
