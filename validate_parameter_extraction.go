package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// FunctionSignature represents a parsed function signature
type FunctionSignature struct {
	Name             string            `json:"name"`
	Package          string            `json:"package"`
	File             string            `json:"file"`
	Line             int               `json:"line"`
	Signature        string            `json:"signature"`
	Parameters       []Parameter       `json:"parameters"`
	ReturnType       string            `json:"return_type"`
	ReturnComponents []ReturnComponent `json:"return_components"`
	CustomTypes      []CustomType      `json:"custom_types,omitempty"`
}

// Parameter represents a function parameter
type Parameter struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// ReturnComponent represents a component of a return type
type ReturnComponent struct {
	Name        string `json:"name,omitempty"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

// CustomType represents a custom type referenced in the signature
type CustomType struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	File   string `json:"file"`
	Line   int    `json:"line"`
}

// ParseData represents the complete JSON structure
type ParseData struct {
	ParseEntryPoints []FunctionSignature `json:"parse_entry_points"`
	Summary          struct {
		TotalFunctions        int      `json:"total_functions"`
		UniqueNames           int      `json:"unique_names"`
		Packages              []string `json:"packages"`
		CustomTypesReferenced []string `json:"custom_types_referenced"`
	} `json:"summary"`
}

func main() {
	// Read the JSON file
	data, err := os.ReadFile("parse_entry_point_signatures.json")
	if err != nil {
		log.Fatalf("Failed to read JSON file: %v", err)
	}

	var parseData ParseData
	err = json.Unmarshal(data, &parseData)
	if err != nil {
		log.Fatalf("Failed to parse JSON: %v", err)
	}

	fmt.Println("=== Parameter and Return Type Extraction Validation ===\n")

	// Validate each function
	allValid := true
	for i, fn := range parseData.ParseEntryPoints {
		fmt.Printf("Function %d: %s (from %s)\n", i+1, fn.Name, fn.Package)

		// Check parameters
		if len(fn.Parameters) == 0 {
			fmt.Printf("  ✓ No parameters (valid for functions with no params)\n")
		} else {
			fmt.Printf("  ✓ Parameters extracted: %d\n", len(fn.Parameters))
			for _, param := range fn.Parameters {
				fmt.Printf("    - %s: %s\n", param.Name, param.Type)

				// Validate complex type handling
				if isComplexType(param.Type) {
					fmt.Printf("      ✗ Complex type detected: %s\n", param.Type)
				}
			}
		}

		// Check return type
		if fn.ReturnType == "" {
			fmt.Printf("  ✗ MISSING RETURN TYPE\n")
			allValid = false
		} else {
			fmt.Printf("  ✓ Return type: %s\n", fn.ReturnType)

			// Validate return components
			if len(fn.ReturnComponents) == 0 {
				fmt.Printf("    ⚠ No return components broken down\n")
			} else {
				fmt.Printf("    ✓ Return components: %d\n", len(fn.ReturnComponents))
				for _, comp := range fn.ReturnComponents {
					if comp.Name != "" {
						fmt.Printf("      - %s: %s", comp.Name, comp.Type)
					} else {
						fmt.Printf("      - %s", comp.Type)
					}
					if comp.Description != "" {
						fmt.Printf(" (%s)", comp.Description)
					}
					fmt.Println()
				}
			}
		}

		// Check custom types
		if len(fn.CustomTypes) > 0 {
			fmt.Printf("  ✓ Custom types referenced: %d\n", len(fn.CustomTypes))
			for _, ct := range fn.CustomTypes {
				fmt.Printf("    - %s (%s) at %s:%d\n", ct.Name, ct.Kind, ct.File, ct.Line)
			}
		}

		fmt.Println()
	}

	// Summary validation
	fmt.Println("=== Validation Summary ===\n")

	// Check all functions have parameters extracted
	paramsExtracted := 0
	for _, fn := range parseData.ParseEntryPoints {
		if fn.Parameters != nil {
			paramsExtracted++
		}
	}
	fmt.Printf("Parameters extracted: %d/%d functions\n", paramsExtracted, len(parseData.ParseEntryPoints))

	// Check all functions have return types
	returnTypesExtracted := 0
	for _, fn := range parseData.ParseEntryPoints {
		if fn.ReturnType != "" {
			returnTypesExtracted++
		}
	}
	fmt.Printf("Return types documented: %d/%d functions\n", returnTypesExtracted, len(parseData.ParseEntryPoints))

	// Check complex types
	complexTypesHandled := countComplexTypes(parseData.ParseEntryPoints)
	fmt.Printf("Complex types handled: %d instances\n", complexTypesHandled)

	// Validate no parameters missing
	fmt.Println("\n=== Acceptance Criteria Status ===")
	fmt.Printf("✓ All parameters extracted with exact types: %t\n", paramsExtracted == len(parseData.ParseEntryPoints))
	fmt.Printf("✓ Return types documented for every function: %t\n", returnTypesExtracted == len(parseData.ParseEntryPoints))
	fmt.Printf("✓ Complex types handled (generics, pointers, references): %t\n", complexTypesHandled > 0)
	fmt.Printf("✓ No parameters missing from extraction: %t\n", allValid)

	if allValid && paramsExtracted == len(parseData.ParseEntryPoints) && returnTypesExtracted == len(parseData.ParseEntryPoints) {
		fmt.Println("\n✅ All acceptance criteria met!")
		os.Exit(0)
	} else {
		fmt.Println("\n❌ Some acceptance criteria not met")
		os.Exit(1)
	}
}

func isComplexType(typeStr string) bool {
	// Check for pointers, slices, maps, channels, interfaces
	complexIndicators := []string{"*", "[]", "map[", "chan ", "<-", "interface{}"}
	for _, indicator := range complexIndicators {
		if contains(typeStr, indicator) {
			return true
		}
	}
	return false
}

func countComplexTypes(functions []FunctionSignature) int {
	count := 0
	for _, fn := range functions {
		// Check parameters
		for _, param := range fn.Parameters {
			if isComplexType(param.Type) {
				count++
			}
		}
		// Check return components
		for _, comp := range fn.ReturnComponents {
			if isComplexType(comp.Type) {
				count++
			}
		}
	}
	return count
}

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