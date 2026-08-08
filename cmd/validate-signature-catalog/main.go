// validate-signature-catalog validates the completeness and consistency of the signature catalog
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// Signature represents a function signature in the catalog
type Signature struct {
	Name              string            `json:"name"`
	Package           string            `json:"package"`
	File              string            `json:"file"`
	Line              int               `json:"line"`
	Signature         string            `json:"signature"`
	Parameters        []Parameter       `json:"parameters"`
	ReturnType        string            `json:"return_type"`
	ReturnComponents  []ReturnComponent `json:"return_components,omitempty"`
	CustomTypes       []CustomType      `json:"custom_types,omitempty"`
	Category          string            `json:"category,omitempty"`
	Purpose           string            `json:"purpose,omitempty"`
}

type Parameter struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type ReturnComponent struct {
	Name        string `json:"name,omitempty"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

type CustomType struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	File   string `json:"file"`
	Line   int    `json:"line"`
}

// Catalog represents the full signature catalog
type Catalog struct {
	Signatures []Signature `json:"signatures"`
	Summary    struct {
		TotalFunctions  int      `json:"total_functions"`
		ParseFunctions  int      `json:"parse_functions"`
		StreamFunctions int      `json:"stream_functions"`
		UniqueNames     int      `json:"unique_names"`
		Packages        []string `json:"packages"`
	} `json:"summary"`
	SourceFiles []string `json:"source_files,omitempty"`
}

// ParseEntryPoints represents the parse entry points JSON
type ParseEntryPoints struct {
	ParseEntryPoints []Signature `json:"parse_entry_points"`
	Summary          struct {
		TotalFunctions int `json:"total_functions"`
		UniqueNames    int `json:"unique_names"`
	} `json:"summary"`
}

// StreamEntryPoints represents the stream entry points JSON
type StreamEntryPoints struct {
	StreamEntryPoints []struct {
		Name      string `json:"name"`
		Package   string `json:"package"`
		File      string `json:"file"`
		Line      int    `json:"line"`
		Signature string `json:"signature"`
		Purpose   string `json:"purpose"`
	} `json:"stream_entry_points"`
	StateEntryPoints []string `json:"state_entry_points"`
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Read the main catalog
	catalog, err := readCatalog("signatures.json")
	if err != nil {
		log.Fatalf("Failed to read signatures.json: %v", err)
	}

	// Read parse entry points
	parsePoints, err := readParseEntryPoints("parse_entry_point_signatures.json")
	if err != nil {
		log.Fatalf("Failed to read parse_entry_point_signatures.json: %v", err)
	}

	// Read stream entry points
	streamPoints, err := readStreamEntryPoints("stream_entry_points.json")
	if err != nil {
		log.Fatalf("Failed to read stream_entry_points.json: %v", err)
	}

	// Validate completeness
	fmt.Println("=== Signature Catalog Validation ===\n")

	// Check 1: All parse entry points are in catalog
	fmt.Println("Check 1: Parse entry points completeness")
	validateParseEntryPoints(catalog, parsePoints)

	// Check 2: All stream entry points are in catalog
	fmt.Println("\nCheck 2: Stream entry points completeness")
	validateStreamEntryPoints(catalog, streamPoints)

	// Check 3: Structured format consistency
	fmt.Println("\nCheck 3: Structured format consistency")
	validateFormatConsistency(catalog)

	// Check 4: Category assignment
	fmt.Println("\nCheck 4: Category assignment")
	validateCategories(catalog)

	// Summary
	fmt.Println("\n=== Validation Summary ===")
	fmt.Printf("Total signatures in catalog: %d\n", len(catalog.Signatures))
	fmt.Printf("Parse functions: %d\n", catalog.Summary.ParseFunctions)
	fmt.Printf("Stream functions: %d\n", catalog.Summary.StreamFunctions)
	fmt.Printf("Unique names: %d\n", catalog.Summary.UniqueNames)
	fmt.Printf("Packages covered: %d\n", len(catalog.Summary.Packages))

	// List missing functions if any
	missing := findMissingFunctions(catalog, parsePoints, streamPoints)
	if len(missing) > 0 {
		fmt.Printf("\n⚠️  WARNING: %d functions are missing from the catalog:\n", len(missing))
		for _, fn := range missing {
			fmt.Printf("  - %s (%s)\n", fn.Name, fn.Package)
		}
	} else {
		fmt.Println("\n✅ All expected functions are present in the catalog")
	}
}

func readCatalog(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var catalog Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, err
	}

	return &catalog, nil
}

func readParseEntryPoints(path string) (*ParseEntryPoints, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var points ParseEntryPoints
	if err := json.Unmarshal(data, &points); err != nil {
		return nil, err
	}

	return &points, nil
}

func readStreamEntryPoints(path string) (*StreamEntryPoints, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var points StreamEntryPoints
	if err := json.Unmarshal(data, &points); err != nil {
		return nil, err
	}

	return &points, nil
}

func validateParseEntryPoints(catalog *Catalog, parsePoints *ParseEntryPoints) {
	missing := []string{}
	found := 0

	catalogSignatures := buildSignatureMap(catalog.Signatures)

	for _, parseSig := range parsePoints.ParseEntryPoints {
		key := fmt.Sprintf("%s:%s:%d", parseSig.Package, parseSig.Name, parseSig.Line)
		if _, exists := catalogSignatures[key]; exists {
			found++
		} else {
			missing = append(missing, fmt.Sprintf("%s (%s:%d)", parseSig.Name, parseSig.File, parseSig.Line))
		}
	}

	fmt.Printf("  Found: %d/%d parse entry points\n", found, len(parsePoints.ParseEntryPoints))
	if len(missing) > 0 {
		fmt.Printf("  Missing: %d\n", len(missing))
		for _, m := range missing {
			fmt.Printf("    - %s\n", m)
		}
	} else {
		fmt.Println("  ✅ All parse entry points present")
	}
}

func validateStreamEntryPoints(catalog *Catalog, streamPoints *StreamEntryPoints) {
	missing := []string{}
	found := 0

	catalogSignatures := buildSignatureMap(catalog.Signatures)

	for _, streamSig := range streamPoints.StreamEntryPoints {
		key := fmt.Sprintf("%s:%s:%d", streamSig.Package, streamSig.Name, streamSig.Line)
		if _, exists := catalogSignatures[key]; exists {
			found++
		} else {
			missing = append(missing, fmt.Sprintf("%s (%s:%d)", streamSig.Name, streamSig.File, streamSig.Line))
		}
	}

	fmt.Printf("  Found: %d/%d stream entry points\n", found, len(streamPoints.StreamEntryPoints))
	if len(missing) > 0 {
		fmt.Printf("  Missing: %d\n", len(missing))
		for _, m := range missing {
			fmt.Printf("    - %s\n", m)
		}
	} else {
		fmt.Println("  ✅ All stream entry points present")
	}
}

func validateFormatConsistency(catalog *Catalog) {
	issues := []string{}

	for i, sig := range catalog.Signatures {
		// Check required fields
		if sig.Name == "" {
			issues = append(issues, fmt.Sprintf("Signature %d: missing name", i))
		}
		if sig.Package == "" {
			issues = append(issues, fmt.Sprintf("Signature %d: missing package", i))
		}
		if sig.File == "" {
			issues = append(issues, fmt.Sprintf("Signature %d: missing file", i))
		}
		if sig.Line == 0 {
			issues = append(issues, fmt.Sprintf("Signature %d: missing line number", i))
		}
		if sig.Signature == "" {
			issues = append(issues, fmt.Sprintf("Signature %d: missing signature string", i))
		}
		if sig.Category == "" {
			issues = append(issues, fmt.Sprintf("%s: missing category", sig.Name))
		}
		// Return type should be present
		if sig.ReturnType == "" {
			issues = append(issues, fmt.Sprintf("%s: missing return_type", sig.Name))
		}
	}

	fmt.Printf("  Checked %d signatures for format consistency\n", len(catalog.Signatures))
	if len(issues) > 0 {
		fmt.Printf("  ⚠️  Found %d format issues:\n", len(issues))
		for _, issue := range issues {
			fmt.Printf("    - %s\n", issue)
		}
	} else {
		fmt.Println("  ✅ All signatures properly formatted")
	}
}

func validateCategories(catalog *Catalog) {
	categories := map[string]int{}
	for _, sig := range catalog.Signatures {
		categories[sig.Category]++
	}

	fmt.Printf("  Category distribution:\n")
	for cat, count := range categories {
		fmt.Printf("    %s: %d\n", cat, count)
	}

	// Check for uncategorized functions
	if count, exists := categories[""]; exists && count > 0 {
		fmt.Printf("  ⚠️  %d functions without category\n", count)
	} else {
		fmt.Println("  ✅ All functions have categories assigned")
	}
}

func buildSignatureMap(signatures []Signature) map[string]Signature {
	m := make(map[string]Signature)
	for _, sig := range signatures {
		key := fmt.Sprintf("%s:%s:%d", sig.Package, sig.Name, sig.Line)
		m[key] = sig
	}
	return m
}

func findMissingFunctions(catalog *Catalog, parsePoints *ParseEntryPoints, streamPoints *StreamEntryPoints) []Signature {
	missing := []Signature{}
	catalogSignatures := buildSignatureMap(catalog.Signatures)

	// Check parse functions
	for _, parseSig := range parsePoints.ParseEntryPoints {
		key := fmt.Sprintf("%s:%s:%d", parseSig.Package, parseSig.Name, parseSig.Line)
		if _, exists := catalogSignatures[key]; !exists {
			missing = append(missing, parseSig)
		}
	}

	// Check stream functions (convert to Signature format)
	for _, streamSig := range streamPoints.StreamEntryPoints {
		key := fmt.Sprintf("%s:%s:%d", streamSig.Package, streamSig.Name, streamSig.Line)
		if _, exists := catalogSignatures[key]; !exists {
			// Convert stream signature to catalog format
			sig := Signature{
				Name:      streamSig.Name,
				Package:   streamSig.Package,
				File:      streamSig.File,
				Line:      streamSig.Line,
				Signature: streamSig.Signature,
				Purpose:   streamSig.Purpose,
			}
			missing = append(missing, sig)
		}
	}

	return missing
}
