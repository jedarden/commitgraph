// verify-line-numbers checks the accuracy of line numbers in the signature catalog
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
)

type Signature struct {
	Name      string `json:"name"`
	Package   string `json:"package"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	Signature string `json:"signature"`
}

type Catalog struct {
	Signatures []Signature `json:"signatures"`
}

func main() {
	data, err := os.ReadFile("signatures.json")
	if err != nil {
		log.Fatalf("Failed to read signatures.json: %v", err)
	}

	var catalog Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		log.Fatalf("Failed to parse signatures.json: %v", err)
	}

	fmt.Println("=== Verifying Line Numbers in Signature Catalog ===\n")

	discrepancies := []string{}
	for _, sig := range catalog.Signatures {
		actualLine, err := findFunctionLine(sig.File, sig.Name, sig.Package)
		if err != nil {
			fmt.Printf("⚠️  %s (%s:%s): %v\n", sig.Name, sig.Package, sig.File, err)
			continue
		}

		if actualLine != sig.Line {
			discrepancies = append(discrepancies, fmt.Sprintf("%s (%s): catalog says %d, actual is %d", sig.Name, sig.File, sig.Line, actualLine))
			fmt.Printf("❌ %s (%s): catalog says line %d, actual is line %d\n", sig.Name, sig.File, sig.Line, actualLine)
		}
	}

	if len(discrepancies) == 0 {
		fmt.Println("\n✅ All line numbers are accurate!")
	} else {
		fmt.Printf("\n⚠️  Found %d line number discrepancies\n", len(discrepancies))
	}
}

func findFunctionLine(filePath, funcName, pkg string) (int, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	// Build regex pattern to match the function signature
	// Handle both regular functions and methods
	pattern := `func ` + funcName + `\(`

	// For methods, try with receiver pattern
	if strings.Contains(funcName, "Ingest") || strings.Contains(funcName, "Upsert") {
		// These are likely methods
		receiverMap := map[string]string{
			"IngestResolution":     `\(i \*Ingester\) `,
			"IngestEmailResolution": `\(i \*IdentityIngester\) `,
			"UpsertAliases":         `\(a \*AliasIngester\) `,
		}
		if receiver, ok := receiverMap[funcName]; ok {
			pattern = `func ` + receiver + funcName + `\(`
		}
	}

	re := regexp.MustCompile(pattern)

	for i, line := range lines {
		if re.MatchString(line) {
			return i + 1, nil // 1-based line number
		}
	}

	return 0, fmt.Errorf("function not found")
}
