package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// ParseSignature represents a parse function signature
type ParseSignature struct {
	Name       string `json:"name"`
	Package    string `json:"package"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Signature  string `json:"signature"`
}

// ParseEntryPoints represents parse entry point signatures
type ParseEntryPoints struct {
	ParseEntryPoints []ParseSignature `json:"parse_entry_points"`
}

// StreamEntryPoints represents stream entry points
type StreamEntryPoints struct {
	StreamEntryPoints []StreamFunction   `json:"stream_entry_points"`
	StateEntryPoints  []string           `json:"state_entry_points"`
}

// StreamFunction represents a stream function
type StreamFunction struct {
	Name      string `json:"name"`
	Package   string `json:"package"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	Signature string `json:"signature"`
	Purpose   string `json:"purpose"`
}

// SignaturesData represents the complete signatures data
type SignaturesData struct {
	Signatures []CompleteSignature `json:"signatures"`
	Summary    struct {
		TotalFunctions int `json:"total_functions"`
		ParseFunctions int `json:"parse_functions"`
		StreamFunctions int `json:"stream_functions"`
	} `json:"summary"`
}

// CompleteSignature represents a complete function signature in the catalog
type CompleteSignature struct {
	Name       string `json:"name"`
	Package    string `json:"package"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Signature  string `json:"signature"`
	Category   string `json:"category"`
	Purpose    string `json:"purpose,omitempty"`
}

func main() {
	fmt.Println("=== Complete Signature Catalog Verification ===\n")

	// Read all three files
	parseData, err := os.ReadFile("parse_entry_point_signatures.json")
	if err != nil {
		log.Fatalf("❌ Failed to read parse_entry_point_signatures.json: %v", err)
	}

	streamData, err := os.ReadFile("stream_entry_points.json")
	if err != nil {
		log.Fatalf("❌ Failed to read stream_entry_points.json: %v", err)
	}

	sigData, err := os.ReadFile("signatures.json")
	if err != nil {
		log.Fatalf("❌ Failed to read signatures.json: %v", err)
	}

	var parsePoints ParseEntryPoints
	if err := json.Unmarshal(parseData, &parsePoints); err != nil {
		log.Fatalf("❌ Failed to parse parse_entry_point_signatures.json: %v", err)
	}

	var streamPoints StreamEntryPoints
	if err := json.Unmarshal(streamData, &streamPoints); err != nil {
		log.Fatalf("❌ Failed to parse stream_entry_points.json: %v", err)
	}

	var signatures SignaturesData
	if err := json.Unmarshal(sigData, &signatures); err != nil {
		log.Fatalf("❌ Failed to parse signatures.json: %v", err)
	}

	fmt.Printf("✅ Successfully loaded all files\n")
	fmt.Printf("   - Parse entry points: %d\n", len(parsePoints.ParseEntryPoints))
	fmt.Printf("   - Stream entry points: %d\n", len(streamPoints.StreamEntryPoints))
	fmt.Printf("   - Total in catalog: %d\n\n", len(signatures.Signatures))

	// Check 1: Verify all parse entry points are in the catalog
	fmt.Println("=== Check 1: Parse Entry Points Coverage ===")
	missingParse := checkParseCoverage(parsePoints, signatures)
	if len(missingParse) == 0 {
		fmt.Println("✅ All parse entry points present in catalog")
	} else {
		fmt.Printf("❌ Missing %d parse entry points:\n", len(missingParse))
		for _, missing := range missingParse {
			fmt.Printf("   - %s (%s:%d)\n", missing.Name, missing.Package, missing.Line)
		}
	}

	// Check 2: Verify all stream entry points are in the catalog
	fmt.Println("\n=== Check 2: Stream Entry Points Coverage ===")
	missingStream := checkStreamCoverage(streamPoints, signatures)
	if len(missingStream) == 0 {
		fmt.Println("✅ All stream entry points present in catalog")
	} else {
		fmt.Printf("❌ Missing %d stream entry points:\n", len(missingStream))
		for _, missing := range missingStream {
			fmt.Printf("   - %s (%s:%d)\n", missing.Name, missing.Package, missing.Line)
		}
	}

	// Check 3: Verify format consistency
	fmt.Println("\n=== Check 3: Format Consistency ===")
	formatIssues := checkFormatConsistency(signatures)
	if len(formatIssues) == 0 {
		fmt.Println("✅ All signatures have consistent format")
	} else {
		fmt.Printf("❌ Found %d format issues:\n", len(formatIssues))
		for _, issue := range formatIssues {
			fmt.Printf("   - %s: %s\n", issue.Function, issue.Issue)
		}
	}

	// Check 4: Verify completeness
	fmt.Println("\n=== Check 4: Completeness Check ===")
	completenessIssues := checkCompleteness(signatures)
	if len(completenessIssues) == 0 {
		fmt.Println("✅ All signatures are complete")
	} else {
		fmt.Printf("❌ Found %d completeness issues:\n", len(completenessIssues))
		for _, issue := range completenessIssues {
			fmt.Printf("   - %s: %s\n", issue.Function, issue.Issue)
		}
	}

	// Check 5: Category distribution
	fmt.Println("\n=== Check 5: Category Distribution ===")
	categories := make(map[string]int)
	for _, sig := range signatures.Signatures {
		categories[sig.Category]++
	}
	for category, count := range categories {
		fmt.Printf("   - %s: %d\n", category, count)
	}

	// Final summary
	fmt.Println("\n=== Final Summary ===")
	totalExpected := len(parsePoints.ParseEntryPoints) + len(streamPoints.StreamEntryPoints)
	totalActual := len(signatures.Signatures)

	allChecksPassed := len(missingParse) == 0 &&
		len(missingStream) == 0 &&
		len(formatIssues) == 0 &&
		len(completenessIssues) == 0

	fmt.Printf("Expected functions: %d\n", totalExpected)
	fmt.Printf("Actual functions: %d\n", totalActual)
	fmt.Printf("Parse functions: %d (expected: %d)\n",
		signatures.Summary.ParseFunctions, len(parsePoints.ParseEntryPoints))
	fmt.Printf("Stream functions: %d (expected: %d)\n",
		signatures.Summary.StreamFunctions, len(streamPoints.StreamEntryPoints))

	if allChecksPassed {
		fmt.Println("\n✅ All verification checks passed!")
		fmt.Println("The signature catalog is complete and ready for use.")
		os.Exit(0)
	} else {
		fmt.Println("\n❌ Some verification checks failed")
		os.Exit(1)
	}
}

type Issue struct {
	Function string
	Issue    string
	Name     string
	Package  string
	Line     int
}

func checkParseCoverage(parsePoints ParseEntryPoints, signatures SignaturesData) []Issue {
	var missing []Issue

	for _, parseFunc := range parsePoints.ParseEntryPoints {
		found := false
		for _, sig := range signatures.Signatures {
			if sig.Name == parseFunc.Name &&
			   sig.Package == parseFunc.Package &&
			   sig.Line == parseFunc.Line {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, Issue{
				Name:     parseFunc.Name,
				Package:  parseFunc.Package,
				Line:     parseFunc.Line,
				Function: fmt.Sprintf("%s (%s:%d)", parseFunc.Name, parseFunc.Package, parseFunc.Line),
				Issue:    "Not found in catalog",
			})
		}
	}

	return missing
}

func checkStreamCoverage(streamPoints StreamEntryPoints, signatures SignaturesData) []Issue {
	var missing []Issue

	for _, streamFunc := range streamPoints.StreamEntryPoints {
		found := false
		for _, sig := range signatures.Signatures {
			if sig.Name == streamFunc.Name &&
			   sig.Package == streamFunc.Package &&
			   sig.Line == streamFunc.Line {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, Issue{
				Name:     streamFunc.Name,
				Package:  streamFunc.Package,
				Line:     streamFunc.Line,
				Function: fmt.Sprintf("%s (%s:%d)", streamFunc.Name, streamFunc.Package, streamFunc.Line),
				Issue:    "Not found in catalog",
			})
		}
	}

	return missing
}

func checkFormatConsistency(signatures SignaturesData) []Issue {
	var issues []Issue

	for _, sig := range signatures.Signatures {
		// Check if signature is present
		if sig.Signature == "" {
			issues = append(issues, Issue{
				Function: fmt.Sprintf("%s (%s:%d)", sig.Name, sig.Package, sig.Line),
				Issue:    "Missing signature field",
			})
		}

		// Check if package is present
		if sig.Package == "" {
			issues = append(issues, Issue{
				Function: fmt.Sprintf("%s (%s:%d)", sig.Name, sig.Package, sig.Line),
				Issue:    "Missing package field",
			})
		}

		// Check if file is present
		if sig.File == "" {
			issues = append(issues, Issue{
				Function: fmt.Sprintf("%s (%s:%d)", sig.Name, sig.Package, sig.Line),
				Issue:    "Missing file field",
			})
		}

		// Check if line number is valid
		if sig.Line <= 0 {
			issues = append(issues, Issue{
				Function: fmt.Sprintf("%s (%s:%d)", sig.Name, sig.Package, sig.Line),
				Issue:    "Invalid line number",
			})
		}

		// Check if category is present
		if sig.Category == "" {
			issues = append(issues, Issue{
				Function: fmt.Sprintf("%s (%s:%d)", sig.Name, sig.Package, sig.Line),
				Issue:    "Missing category field",
			})
		}
	}

	return issues
}

func checkCompleteness(signatures SignaturesData) []Issue {
	var issues []Issue

	for _, sig := range signatures.Signatures {
		// Check if it's a complete entry (not just a stub)
		if sig.Name == "" {
			issues = append(issues, Issue{
				Function: "unknown",
				Issue:    "Function with no name found",
			})
		}
	}

	return issues
}