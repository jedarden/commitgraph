package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// FunctionSignature represents a complete function signature
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
	Category         string            `json:"category"` // "parse" or "stream"
	Purpose          string            `json:"purpose,omitempty"`
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

// SignaturesData represents the complete JSON structure
type SignaturesData struct {
	Signatures       []FunctionSignature `json:"signatures"`
	Summary          Summary             `json:"summary"`
	SourceFiles      []string            `json:"source_files"`
	GeneratedAt      string              `json:"generated_at"`
	GeneratedByVersion string            `json:"generated_by_version"`
}

// Summary provides statistics about the signatures
type Summary struct {
	TotalFunctions      int      `json:"total_functions"`
	ParseFunctions       int      `json:"parse_functions"`
	StreamFunctions      int      `json:"stream_functions"`
	UniqueNames         int      `json:"unique_names"`
	Packages            []string `json:"packages"`
	CustomTypesCount    int      `json:"custom_types_count"`
}

func main() {
	fmt.Println("=== Signature Extraction Tool ===")

	// Get the workspace root
	workspaceRoot := getWorkspaceRoot()
	fmt.Printf("Workspace root: %s\n", workspaceRoot)

	// Read existing parse_entry_point_signatures.json
	parseSigs, err := readParseSignatures(filepath.Join(workspaceRoot, "parse_entry_point_signatures.json"))
	if err != nil {
		log.Printf("Warning: Could not read parse_entry_point_signatures.json: %v", err)
		parseSigs = []FunctionSignature{}
	}

	// Read existing stream_entry_points.json
	streamSigs, err := readStreamSignatures(filepath.Join(workspaceRoot, "stream_entry_points.json"))
	if err != nil {
		log.Printf("Warning: Could not read stream_entry_points.json: %v", err)
		streamSigs = []FunctionSignature{}
	}

	// Enhance stream signatures with detailed parameter/return info
	streamSigs = enhanceStreamSignatures(streamSigs, workspaceRoot)

	// Combine all signatures
	allSignatures := combineSignatures(parseSigs, streamSigs)

	// Build summary
	summary := buildSummary(allSignatures)

	// Create the final data structure
	data := SignaturesData{
		Signatures:       allSignatures,
		Summary:          summary,
		SourceFiles:      getSourceFiles(workspaceRoot),
		GeneratedAt:     getTimestamp(),
		GeneratedByVersion: "1.0.0",
	}

	// Write to output file
	outputFile := filepath.Join(workspaceRoot, "signatures.json")
	fmt.Printf("\nWriting signatures to: %s\n", outputFile)
	if err := writeSignatures(data, outputFile); err != nil {
		log.Fatalf("Failed to write signatures: %v", err)
	}

	// Validate the output
	if err := validateSignatures(outputFile); err != nil {
		log.Fatalf("Validation failed: %v", err)
	}

	fmt.Println("\n✅ Signature extraction complete!")
	fmt.Printf("Total signatures: %d\n", len(allSignatures))
	fmt.Printf("Output file: %s\n", outputFile)
}

func getWorkspaceRoot() string {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}
	return cwd
}

func readParseSignatures(filePath string) ([]FunctionSignature, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var parseData struct {
		ParseEntryPoints []FunctionSignature `json:"parse_entry_points"`
	}

	if err := json.Unmarshal(data, &parseData); err != nil {
		return nil, err
	}

	// Add category to each signature
	for i := range parseData.ParseEntryPoints {
		parseData.ParseEntryPoints[i].Category = "parse"
	}

	return parseData.ParseEntryPoints, nil
}

func readStreamSignatures(filePath string) ([]FunctionSignature, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var streamData struct {
		StreamEntryPoints []struct {
			Name      string `json:"name"`
			Package   string `json:"package"`
			File      string `json:"file"`
			Line      int    `json:"line"`
			Signature string `json:"signature"`
			Purpose   string `json:"purpose,omitempty"`
		} `json:"stream_entry_points"`
	}

	if err := json.Unmarshal(data, &streamData); err != nil {
		return nil, err
	}

	signatures := make([]FunctionSignature, len(streamData.StreamEntryPoints))
	for i, entry := range streamData.StreamEntryPoints {
		signatures[i] = FunctionSignature{
			Name:      entry.Name,
			Package:   entry.Package,
			File:      entry.File,
			Line:      entry.Line,
			Signature: entry.Signature,
			Purpose:   entry.Purpose,
			Category:  "stream",
			// Will populate parameters and return types later
		}
	}

	return signatures, nil
}

func enhanceStreamSignatures(signatures []FunctionSignature, workspaceRoot string) []FunctionSignature {
	fmt.Println("\nEnhancing stream signatures with detailed parameter and return type information...")

	for i := range signatures {
		sig := &signatures[i]

		// Parse the Go file to extract detailed signature information
		filePath := filepath.Join(workspaceRoot, sig.File)
		if details, err := extractSignatureDetails(filePath, sig.Name); err == nil {
			sig.Parameters = details.Parameters
			sig.ReturnType = details.ReturnType
			sig.ReturnComponents = details.ReturnComponents
			sig.CustomTypes = details.CustomTypes
			fmt.Printf("  ✓ Enhanced: %s (%d parameters, %d return components)\n",
				sig.Name, len(sig.Parameters), len(sig.ReturnComponents))
		} else {
			fmt.Printf("  ⚠ Could not enhance %s: %v\n", sig.Name, err)
		}
	}

	return signatures
}

func extractSignatureDetails(filePath, funcName string) (*FunctionSignature, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	var foundFunc *ast.FuncDecl
	ast.Inspect(node, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			if fn.Name.Name == funcName {
				foundFunc = fn
				return false
			}
		}
		return true
	})

	if foundFunc == nil {
		return nil, fmt.Errorf("function not found")
	}

	sig := &FunctionSignature{
		Name: funcName,
	}

	// Extract parameters
	if foundFunc.Type.Params != nil {
		for _, param := range foundFunc.Type.Params.List {
			paramType := exprToString(param.Type)
			if len(param.Names) == 0 {
				sig.Parameters = append(sig.Parameters, Parameter{
					Name: "",
					Type: paramType,
				})
			} else {
				for _, name := range param.Names {
					sig.Parameters = append(sig.Parameters, Parameter{
						Name: name.Name,
						Type: paramType,
					})
				}
			}
		}
	}

	// Extract return type
	if foundFunc.Type.Results != nil {
		var returnTypes []string
		for _, result := range foundFunc.Type.Results.List {
			resultType := exprToString(result.Type)
			returnTypes = append(returnTypes, resultType)
		}
		sig.ReturnType = strings.Join(returnTypes, ", ")
	}

	return sig, nil
}

func exprToString(expr ast.Expr) string {
	// Simple expression to string conversion
	// In a full implementation, you'd use go/types or go/printer for accurate formatting
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + exprToString(t.X)
	case *ast.ArrayType:
		return "[]" + exprToString(t.Elt)
	case *ast.SelectorExpr:
		return exprToString(t.X) + "." + t.Sel.Name
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.MapType:
		return "map[" + exprToString(t.Key) + "]" + exprToString(t.Value)
	case *ast.ChanType:
		if t.Dir == 1 { // SEND
			return "chan<- " + exprToString(t.Value)
		} else if t.Dir == 2 { // RECV
			return "<-chan " + exprToString(t.Value)
		}
		return "chan " + exprToString(t.Value)
	case *ast.Ellipsis:
		return "..." + exprToString(t.Elt)
	case *ast.FuncType:
		return "func(...)"
	case *ast.ParenExpr:
		return "(" + exprToString(t.X) + ")"
	default:
		return "unknown"
	}
}

func combineSignatures(parse, stream []FunctionSignature) []FunctionSignature {
	combined := make([]FunctionSignature, 0, len(parse)+len(stream))
	combined = append(combined, parse...)
	combined = append(combined, stream...)
	return combined
}

func buildSummary(signatures []FunctionSignature) Summary {
	summary := Summary{
		TotalFunctions: len(signatures),
	}

	packages := make(map[string]bool)
	uniqueNames := make(map[string]bool)
	customTypesCount := 0

	for _, sig := range signatures {
		if sig.Category == "parse" {
			summary.ParseFunctions++
		} else if sig.Category == "stream" {
			summary.StreamFunctions++
		}

		packages[sig.Package] = true
		uniqueNames[sig.Name] = true
		customTypesCount += len(sig.CustomTypes)
	}

	summary.UniqueNames = len(uniqueNames)
	summary.CustomTypesCount = customTypesCount

	for pkg := range packages {
		summary.Packages = append(summary.Packages, pkg)
	}

	return summary
}

func getSourceFiles(workspaceRoot string) []string {
	files := []string{
		"parse_entry_point_signatures.json",
		"stream_entry_points.json",
		"validate_parameter_extraction.go",
	}
	return files
}

func getTimestamp() string {
	return "2026-08-08T18:30:00Z"
}

func writeSignatures(data SignaturesData, filePath string) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, jsonData, 0644)
}

func validateSignatures(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var sigData SignaturesData
	if err := json.Unmarshal(data, &sigData); err != nil {
		return err
	}

	// Basic validation
	if len(sigData.Signatures) == 0 {
		return fmt.Errorf("no signatures found")
	}

	// Check each signature has required fields
	for i, sig := range sigData.Signatures {
		if sig.Name == "" {
			return fmt.Errorf("signature %d: missing name", i)
		}
		if sig.Package == "" {
			return fmt.Errorf("signature %d: missing package", i)
		}
		if sig.File == "" {
			return fmt.Errorf("signature %d: missing file", i)
		}
		if sig.Signature == "" {
			return fmt.Errorf("signature %d: missing signature", i)
		}
		if sig.Category == "" {
			return fmt.Errorf("signature %d: missing category", i)
		}
	}

	fmt.Printf("  ✓ Validation passed: %d signatures\n", len(sigData.Signatures))
	return nil
}
