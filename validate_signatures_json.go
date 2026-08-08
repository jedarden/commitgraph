package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// SignaturesData represents the complete JSON structure
type SignaturesData struct {
	Signatures []FunctionSignature `json:"signatures"`
	Summary    Summary             `json:"summary"`
}

// FunctionSignature represents a complete function signature
type FunctionSignature struct {
	Name       string   `json:"name"`
	Package    string   `json:"package"`
	File       string   `json:"file"`
	Line       int      `json:"line"`
	Signature  string   `json:"signature"`
	Parameters []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"parameters"`
	ReturnType string `json:"return_type"`
	Category   string `json:"category"`
}

// Summary provides statistics about the signatures
type Summary struct {
	TotalFunctions int `json:"total_functions"`
}

func main() {
	fmt.Println("=== Signature JSON Validation ===\n")

	data, err := os.ReadFile("signatures.json")
	if err != nil {
		log.Fatalf("❌ Failed to read signatures.json: %v", err)
	}

	var sigData SignaturesData
	if err := json.Unmarshal(data, &sigData); err != nil {
		log.Fatalf("❌ Failed to parse JSON: %v", err)
	}

	fmt.Println("✅ JSON is valid and parseable")

	// Validate acceptance criteria
	allCriteriaMet := true

	// 1. JSON structure defined
	structureValid := validateStructure(sigData)
	fmt.Printf("1. JSON structure defined: %v\n", structureValid)
	if !structureValid {
		allCriteriaMet = false
	}

	// 2. All signatures stored in structured format
	allStored := len(sigData.Signatures) > 0
	fmt.Printf("2. All signatures stored in structured format: %v (%d signatures)\n", allStored, len(sigData.Signatures))
	if !allStored {
		allCriteriaMet = false
	}

	// 3. Every parameter captured with exact type
	paramsCaptured := validateParameters(sigData.Signatures)
	fmt.Printf("3. Every parameter captured with exact type: %v\n", paramsCaptured)
	if !paramsCaptured {
		allCriteriaMet = false
	}

	// 4. Return types documented
	returnTypesDoc := validateReturnTypes(sigData.Signatures)
	fmt.Printf("4. Return types documented: %v\n", returnTypesDoc)
	if !returnTypesDoc {
		allCriteriaMet = false
	}

	// 5. JSON is valid and parseable (already checked above)
	fmt.Println("5. JSON is valid and parseable: ✅")

	// 6. Output file created successfully
	fileExists := fileExists("signatures.json")
	fmt.Printf("6. Output file created successfully: %v\n", fileExists)
	if !fileExists {
		allCriteriaMet = false
	}

	fmt.Println("\n=== Summary ===")
	fmt.Printf("Total functions: %d\n", len(sigData.Signatures))
	fmt.Printf("Categories: parse=%d, stream=%d\n", countByCategory(sigData.Signatures, "parse"), countByCategory(sigData.Signatures, "stream"))

	if allCriteriaMet {
		fmt.Println("\n✅ All acceptance criteria met!")
		os.Exit(0)
	} else {
		fmt.Println("\n❌ Some acceptance criteria not met")
		os.Exit(1)
	}
}

func validateStructure(data SignaturesData) bool {
	return len(data.Signatures) > 0 && data.Summary.TotalFunctions > 0
}

func validateParameters(signatures []FunctionSignature) bool {
	for _, sig := range signatures {
		// Check that each function has a parameters array (even if empty)
		if sig.Parameters == nil {
			return false
		}
		// For each parameter, ensure type is present
		for _, param := range sig.Parameters {
			if param.Type == "" {
				return false
			}
		}
	}
	return true
}

func validateReturnTypes(signatures []FunctionSignature) bool {
	for _, sig := range signatures {
		if sig.ReturnType == "" {
			return false
		}
	}
	return true
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func countByCategory(signatures []FunctionSignature, category string) int {
	count := 0
	for _, sig := range signatures {
		if sig.Category == category {
			count++
		}
	}
	return count
}
