// main verifies the email_resolution dump and shows statistics.
package main

import (
	"log"
	"os"
	"regexp"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run main.go <dump-file.sql>")
	}

	dumpFile := os.Args[1]
	log.Printf("Reading dump from: %s", dumpFile)

	content, err := os.ReadFile(dumpFile)
	if err != nil {
		log.Fatalf("Failed to read dump file: %v", err)
	}

	log.Printf("Read %d bytes", len(content))

	// Count INSERT lines
	lines := strings.Split(string(content), "\n")
	insertCount := 0
	var resolvedCount int
	var unresolvableCount int
	var pendingClaimedCount int

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "INSERT INTO email_resolution VALUES") {
			continue
		}

		insertCount++
		status, attemptedAt, updatedAt := parseInsertLine(line)

		switch status {
		case "resolved":
			resolvedCount++
		case "unresolvable":
			unresolvableCount++
		default:
			pendingClaimedCount++
		}

		if insertCount <= 5 {
			log.Printf("Sample %d: status=%s, attempted_at=%s, updated_at=%s",
				insertCount, status, attemptedAt, updatedAt)
		}
	}

	log.Printf("\n=== DUMP STATISTICS ===")
	log.Printf("Total INSERT statements: %d", insertCount)
	log.Printf("Resolved (with login): %d", resolvedCount)
	log.Printf("Unresolvable (no login): %d", unresolvableCount)
	log.Printf("Other (pending/claimed): %d", pendingClaimedCount)
	log.Printf("\nRows to load (resolved only): %d", resolvedCount)
}

func parseInsertLine(line string) (status, attemptedAt, updatedAt string) {
	// Extract VALUES(...) content
	re := regexp.MustCompile(`^INSERT INTO email_resolution VALUES\((.+)\);$`)
	matches := re.FindStringSubmatch(line)
	if len(matches) < 2 {
		return "", "", ""
	}

	valuesStr := matches[1]
	values := splitByComma(valuesStr)

	if len(values) < 12 {
		return "", "", ""
	}

	// Field indices based on schema:
	// 0: author_email, 1: github_login, 2: provider, 3: status
	// 4: priority, 5: is_alias_candidate, 6: claimed_by
	// 7: claimed_at, 8: lease_expires_at, 9: attempted_at
	// 10: created_at, 11: updated_at

	status = unquote(values[3])
	attemptedAt = unquote(values[9])
	updatedAt = unquote(values[11])

	return status, attemptedAt, updatedAt
}

func splitByComma(s string) []string {
	var result []string
	var current strings.Builder
	inQuotes := false

	for _, r := range s {
		switch r {
		case '\'':
			inQuotes = !inQuotes
			current.WriteRune(r)
		case ',':
			if inQuotes {
				current.WriteRune(r)
			} else {
				result = append(result, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.String() != "" {
		result = append(result, current.String())
	}

	return result
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		return s[1 : len(s)-1]
	}
	return s
}
