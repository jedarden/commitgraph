// main loads the queue-api email_resolution dump into Postgres via the ingest path.
//
// This one-off migration script reads the SQLite dump extracted in cg-2v70
// and loads it into Postgres using the identity ingest endpoint with:
//   - source='live' (these are live enrichment worker results)
//   - resolved_at = attempted_at (the timestamp from queue-api)
//   - Only resolved entries (status='resolved' with non-NULL github_login)
//
// Usage:
//   go run main.go /path/to/email_resolution_fresh_*.sql
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jedarden/commitgraph/pkg/identity"
	"github.com/jedarden/commitgraph/pkg/pg"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run main.go <dump-file.sql>")
	}

	dumpFile := os.Args[1]
	log.Printf("Reading dump from: %s", dumpFile)

	// Connect to Postgres
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// Fallback to default for local development
		dbURL = "postgres://postgres:postgres@localhost:5432/commitgraph?sslmode=disable"
	}
	log.Printf("Connecting to Postgres: %s", strings.Split(dbURL, "@")[1]) // Log without credentials

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Read the dump file
	content, err := os.ReadFile(dumpFile)
	if err != nil {
		log.Fatalf("Failed to read dump file: %v", err)
	}

	log.Printf("Read %d bytes from dump file", len(content))

	// Parse INSERT statements
	rows, err := parseDump(string(content))
	if err != nil {
		log.Fatalf("Failed to parse dump: %v", err)
	}

	log.Printf("Parsed %d total rows from dump", len(rows))

	// Filter to only resolved entries
	var resolvedRows []identity.ResolutionRow
	for _, row := range rows {
		if row.Status == "resolved" && row.GitHubLogin != "" {
			// AttemptedAt is *time.Time; skip if nil, otherwise dereference
			if row.AttemptedAt == nil {
				log.Printf("Warning: skipping resolved entry %s with nil AttemptedAt", row.AuthorEmail)
				continue
			}
			resolvedRows = append(resolvedRows, identity.ResolutionRow{
				Email:      row.AuthorEmail,
				Login:      row.GitHubLogin,
				Source:     identity.SourceLive,
				ResolvedAt: *row.AttemptedAt,
			})
		}
	}

	log.Printf("Filtered to %d resolved entries (status='resolved' with non-NULL login)", len(resolvedRows))

	if len(resolvedRows) == 0 {
		log.Fatal("No resolved entries to load - aborting")
	}

	// Create ingester
	executor := pg.NewSQLExecutor(db)
	ingester := identity.NewIngester(pg.NewIdentityIngester(executor))

	// Batch size - process in chunks of 10000 rows
	const batchSize = 10000
	ctx := context.Background()

	totalRows := int64(len(resolvedRows))
	processedRows := int64(0)

	for i := 0; i < len(resolvedRows); i += batchSize {
		end := i + batchSize
		if end > len(resolvedRows) {
			end = len(resolvedRows)
		}

		batch := resolvedRows[i:end]
		log.Printf("Processing batch %d-%d of %d total rows (%.1f%%)",
			i, end, totalRows, float64(end)/float64(totalRows)*100)

		start := time.Now()
		if err := ingester.IngestResolution(ctx, batch); err != nil {
			log.Fatalf("Failed to ingest batch %d-%d: %v", i, end, err)
		}

		duration := time.Since(start)
		log.Printf("  Batch %d-%d completed in %v (%.0f rows/sec)",
			i, end, duration, float64(len(batch))/duration.Seconds())

		processedRows = int64(end)
		log.Printf("  Progress: %d/%d processed, %d ingested, %d skipped",
			processedRows, totalRows,
			ingester.GetIngested(), ingester.GetSkipped())
	}

	// Final report
	log.Println("\n=== LOAD COMPLETE ===")
	log.Printf("Total rows processed: %d", ingester.GetProcessed())
	log.Printf("Successfully ingested: %d", ingester.GetIngested())
	log.Printf("Skipped (conflict): %d", ingester.GetSkipped())

	if len(ingester.GetSkipDetails()) > 0 {
		log.Println("\nSkip breakdown:")
		for reason, count := range ingester.GetSkipDetails() {
			log.Printf("  %s: %d", reason, count)
		}
	}

	log.Println("\nNext step: Verify Postgres row count matches expected resolved count")
}

// QueueAPIRow represents a row from the queue-api dump
type QueueAPIRow struct {
	AuthorEmail    string
	GitHubLogin     string
	Provider        string
	Status          string
	Priority        int
	IsAliasCandidate int
	ClaimedBy       string
	ClaimedAt       *time.Time
	LeaseExpiresAt  *time.Time
	AttemptedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// parseDump extracts INSERT statements from a SQLite dump
func parseDump(dump string) ([]QueueAPIRow, error) {
	var rows []QueueAPIRow

	// Find all INSERT INTO email_resolution VALUES statements
	insertRegex := regexp.MustCompile(`^INSERT INTO email_resolution VALUES\((.+)\);$`)
	lines := strings.Split(dump, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "INSERT INTO email_resolution VALUES") {
			continue
		}

		matches := insertRegex.FindStringSubmatch(line)
		if len(matches) < 2 {
			log.Printf("Warning: could not parse INSERT line: %s", line)
			continue
		}

		// Parse the comma-separated values
		valuesStr := matches[1]
		row, err := parseValuesString(valuesStr)
		if err != nil {
			log.Printf("Warning: failed to parse values: %v", err)
			continue
		}

		rows = append(rows, row)
	}

	return rows, nil
}

// parseValuesString parses the comma-separated values from an INSERT statement
func parseValuesString(valuesStr string) (QueueAPIRow, error) {
	var row QueueAPIRow
	var err error

	// Split by comma - careful with quoted strings
	values := splitCSV(valuesStr)

	if len(values) != 12 {
		return row, fmt.Errorf("expected 12 values, got %d", len(values))
	}

	// Parse each value (remove surrounding quotes)
	row.AuthorEmail = unquoteString(values[0])
	row.GitHubLogin = unquoteString(values[1])
	row.Provider = unquoteString(values[2])
	row.Status = unquoteString(values[3])

	// Priority is an integer (no quotes)
	priorityStr := strings.TrimSpace(values[4])
	if priorityStr == "NULL" {
		row.Priority = 0
	} else {
		fmt.Sscanf(priorityStr, "%d", &row.Priority)
	}

	// is_alias_candidate is an integer
	isAliasStr := strings.TrimSpace(values[5])
	if isAliasStr == "NULL" {
		row.IsAliasCandidate = 0
	} else {
		fmt.Sscanf(isAliasStr, "%d", &row.IsAliasCandidate)
	}

	row.ClaimedBy = unquoteString(values[6])
	row.ClaimedAt = parseTimePtr(unquoteString(values[7]))
	row.LeaseExpiresAt = parseTimePtr(unquoteString(values[8]))
	row.AttemptedAt = parseTimePtr(unquoteString(values[9]))

	// created_at and updated_at are always non-NULL
	row.CreatedAt, err = parseTime(unquoteString(values[10]))
	if err != nil {
		return row, fmt.Errorf("failed to parse created_at: %w", err)
	}

	row.UpdatedAt, err = parseTime(unquoteString(values[11]))
	if err != nil {
		return row, fmt.Errorf("failed to parse updated_at: %w", err)
	}

	return row, nil
}

// splitCSV splits a CSV string, respecting quoted values
func splitCSV(s string) []string {
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

	// Don't forget the last value
	if current.String() != "" {
		result = append(result, current.String())
	}

	return result
}

// unquoteString removes surrounding single quotes from a string
func unquoteString(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		return s[1 : len(s)-1]
	}
	return s
}

// parseTime parses a SQLite timestamp string
func parseTime(s string) (time.Time, error) {
	if s == "NULL" || s == "" {
		return time.Time{}, fmt.Errorf("null or empty time")
	}

	// SQLite datetime format: "2026-07-21 13:22:00"
 layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse time: %s", s)
}

// parseTimePtr parses a timestamp string or returns nil for NULL
func parseTimePtr(s string) *time.Time {
	if s == "NULL" || s == "" {
		return nil
	}

	t, err := parseTime(s)
	if err != nil {
		log.Printf("Warning: failed to parse time %s: %v", s, err)
		return nil
	}

	return &t
}
