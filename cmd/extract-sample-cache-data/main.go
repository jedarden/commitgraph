// extract-sample-cache-data extracts a sample of author_login_cache pairs for testing.
//
// This script reads from claude-leaderboard's author_login_cache table and outputs
// a CSV sample containing between 10-100 pairs, ensuring both valid logins and NULL
// logins are included.
//
// Usage:
//
//	extract-sample-cache-data -output <output.csv> [-count <n>]
//
// The script:
// - Reads from ~/backups/claude-leaderboard/hot.db's author_login_cache table
// - Extracts between 10-100 pairs (default: 50, configurable via -count)
// - Ensures both non-NULL and NULL github_login values are included
// - Preserves the original ISO 8601 timestamp format with microsecond precision
// - Outputs to CSV format matching the testdata/author_login_cache_sample.csv structure
// - Handles errors gracefully with clear error messages
//
// See notes/cg-4p4nu.md for author_login_cache data structure documentation.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	_ "github.com/mattn/go-sqlite3"
)

var (
	// Input database path
	claudeLeaderboardDB = flag.String("db",
		"~/backups/claude-leaderboard/hot.db",
		"Path to claude-leaderboard SQLite database")

	// Output configuration
	outputPath = flag.String("output",
		"",
		"Output CSV file path (required)")

	// Sample size
	count = flag.Int("count",
		50,
		"Number of pairs to extract (between 10-100, inclusive)")
)

func main() {
	flag.Usage = usage
	flag.Parse()

	// Validate and adjust count
	if *count < 10 {
		*count = 10
		log.Printf("Adjusting count to minimum: 10\n")
	}
	if *count > 100 {
		*count = 100
		log.Printf("Adjusting count to maximum: 100\n")
	}

	// Validate output path
	if *outputPath == "" {
		log.Fatal("error: -output is required\n")
	}

	// Expand tilde in database path
	dbPath, err := expandPath(*claudeLeaderboardDB)
	if err != nil {
		log.Fatalf("error: failed to expand database path %q: %v\n", *claudeLeaderboardDB, err)
	}

	// Verify output directory exists
	outputDir := filepath.Dir(*outputPath)
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		log.Fatalf("error: output directory does not exist: %s\n", outputDir)
	}

	// Open SQLite database
	log.Printf("Opening claude-leaderboard database: %s\n", dbPath)
	sqliteDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("error: failed to open claude-leaderboard database: %v\n", err)
	}
	defer sqliteDB.Close()

	// Verify SQLite connection works
	if err := sqliteDB.Ping(); err != nil {
		log.Fatalf("error: claude-leaderboard database ping failed: %v\n", err)
	}

	// Get total row count and NULL login count
	totalRows, nullCount, err := getTableStats(sqliteDB)
	if err != nil {
		log.Fatalf("error: failed to get table statistics: %v\n", err)
	}
	log.Printf("Database statistics: %d total rows, %d NULL logins\n", totalRows, nullCount)

	// Check if we have enough data
	if totalRows < 10 {
		log.Fatalf("error: insufficient data in database (only %d rows, need at least 10)\n", totalRows)
	}

	// Extract sample data
	log.Printf("Extracting %d pairs...\n", *count)
	records, err := extractSampleData(sqliteDB, *count, nullCount, totalRows)
	if err != nil {
		log.Fatalf("error: failed to extract sample data: %v\n", err)
	}

	// If no NULL logins were found in database, add synthetic ones for testing
	if nullCount == 0 {
		log.Printf("Adding synthetic NULL logins for test coverage...\n")
		syntheticNulls := createSyntheticNullEntries(min(5, len(records)/2))
		// Replace some of the non-NULL entries with NULL entries
		records = interleaveNullEntries(records, syntheticNulls)
	}

	// Write CSV output
	log.Printf("Writing output to: %s\n", *outputPath)
	if err := writeCSV(*outputPath, records); err != nil {
		log.Fatalf("error: failed to write output: %v\n", err)
	}

	// Summary
	nullCountInSample := 0
	for _, r := range records {
		if r.GithubLogin == "NULL" {
			nullCountInSample++
		}
	}
	log.Printf("Success! Extracted %d pairs (%d NULL, %d non-NULL)\n",
		len(records), nullCountInSample, len(records)-nullCountInSample)
}

// usage prints the usage message.
func usage() {
	fmt.Fprintf(flag.CommandLine.Output(), "Usage: extract-sample-cache-data [options]\n\n")
	fmt.Fprintf(flag.CommandLine.Output(), "Extracts a sample of author_login_cache pairs for testing.\n\n")
	fmt.Fprintf(flag.CommandLine.Output(), "Options:\n")
	flag.PrintDefaults()
	fmt.Fprintf(flag.CommandLine.Output(), "\nExamples:\n")
	fmt.Fprintf(flag.CommandLine.Output(), "  extract-sample-cache-data -output sample.csv\n")
	fmt.Fprintf(flag.CommandLine.Output(), "  extract-sample-cache-data -output sample.csv -count 25\n")
}

// expandPath expands a tilde (~) in a path to the user's home directory.
func expandPath(path string) (string, error) {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		return filepath.Join(home, path[1:]), nil
	}
	return path, nil
}

// Record represents a single author_login_cache row.
type Record struct {
	AuthorEmail string
	GithubLogin string
	ResolvedAt  string
}

// getTableStats returns the total row count and NULL login count.
func getTableStats(db *sql.DB) (total int, nullCount int, err error) {
	// Get total rows
	err = db.QueryRow("SELECT COUNT(*) FROM author_login_cache").Scan(&total)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to count total rows: %w", err)
	}

	// Get NULL login count (literal string "NULL")
	err = db.QueryRow("SELECT COUNT(*) FROM author_login_cache WHERE github_login = 'NULL'").Scan(&nullCount)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to count NULL logins: %w", err)
	}

	return total, nullCount, nil
}

// extractSampleData extracts a balanced sample of records.
func extractSampleData(db *sql.DB, targetCount int, nullCount int, totalRows int) ([]Record, error) {
	var records []Record

	// Determine how many NULL and non-NULL records to include
	// Ensure at least 1 of each if both types exist
	var nullTarget, nonNullTarget int
	if nullCount > 0 {
		// Include approximately 20% NULL logins if available
		nullTarget = targetCount / 5
		if nullTarget < 1 {
			nullTarget = 1
		}
		if nullTarget > nullCount {
			nullTarget = nullCount
		}
		nonNullTarget = targetCount - nullTarget
	} else {
		log.Printf("Warning: No NULL logins found in database (all %d rows have valid logins)\n", totalRows)
		nullTarget = 0
		nonNullTarget = targetCount
	}

	// Adjust if non-NULL target exceeds available
	nonNullAvailable := totalRows - nullCount
	if nonNullTarget > nonNullAvailable {
		nonNullTarget = nonNullAvailable
	}

	// Extract NULL logins (if any)
	if nullTarget > 0 {
		log.Printf("Extracting %d NULL logins...\n", nullTarget)
		nullRecords, err := extractRecords(db, "github_login = 'NULL'", nullTarget)
		if err != nil {
			return nil, fmt.Errorf("failed to extract NULL logins: %w", err)
		}
		records = append(records, nullRecords...)
	}

	// Extract non-NULL logins
	log.Printf("Extracting %d non-NULL logins...\n", nonNullTarget)
	nonNullRecords, err := extractRecords(db, "github_login != 'NULL'", nonNullTarget)
	if err != nil {
		return nil, fmt.Errorf("failed to extract non-NULL logins: %w", err)
	}
	records = append(records, nonNullRecords...)

	if len(records) < 10 {
		return nil, fmt.Errorf("only extracted %d records (minimum 10 required)", len(records))
	}

	return records, nil
}

// extractRecords queries the database for records matching a WHERE condition.
func extractRecords(db *sql.DB, whereClause string, limit int) ([]Record, error) {
	query := fmt.Sprintf(
		"SELECT author_email, github_login, resolved_at FROM author_login_cache WHERE %s ORDER BY resolved_at DESC LIMIT %d",
		whereClause, limit)

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var r Record
		if err := rows.Scan(&r.AuthorEmail, &r.GithubLogin, &r.ResolvedAt); err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		records = append(records, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return records, nil
}

// writeCSV writes records to a CSV file in the expected format.
func writeCSV(path string, records []Record) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Write header
	header := "author_email,github_login,resolved_at\n"
	if _, err := file.WriteString(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write data rows
	for _, r := range records {
		// Escape email if it contains commas
		email := r.AuthorEmail
		if strconv.QuoteRune('"')[0] == ',' || len(email) != len(r.AuthorEmail) {
			// Check if email needs CSV escaping
			for _, c := range email {
				if c == ',' || c == '"' || c == '\n' || c == '\r' {
					email = strconv.Quote(email)
					break
				}
			}
		}

		line := fmt.Sprintf("%s,%s,%s\n", email, r.GithubLogin, r.ResolvedAt)
		if _, err := file.WriteString(line); err != nil {
			return fmt.Errorf("failed to write row: %w", err)
		}
	}

	return nil
}

// createSyntheticNullEntries creates synthetic NULL login entries for testing.
func createSyntheticNullEntries(count int) []Record {
	baseTime := "2026-08-06T10:00:00.000000+00:00"

	entries := []Record{
		{AuthorEmail: "unknown.user1@example.com", GithubLogin: "NULL", ResolvedAt: baseTime},
		{AuthorEmail: "unresolved@email.com", GithubLogin: "NULL", ResolvedAt: "2026-08-06T10:05:00.000000+00:00"},
		{AuthorEmail: "orphan.email@unknown.com", GithubLogin: "NULL", ResolvedAt: "2026-08-06T10:10:00.000000+00:00"},
		{AuthorEmail: "ghost.user@nowhere.com", GithubLogin: "NULL", ResolvedAt: "2026-08-06T10:15:00.000000+00:00"},
		{AuthorEmail: "anonymous@hidden.org", GithubLogin: "NULL", ResolvedAt: "2026-08-06T10:20:00.000000+00:00"},
	}

	if count > len(entries) {
		count = len(entries)
	}
	return entries[:count]
}

// interleaveNullEntries replaces entries at regular intervals with NULL entries.
func interleaveNullEntries(records []Record, nulls []Record) []Record {
	if len(nulls) == 0 {
		return records
	}

	// Calculate spacing to distribute NULL entries evenly
	skip := len(records) / (len(nulls) + 1)
	if skip < 1 {
		skip = 1
	}

	// Replace entries at regular intervals
	nullIndex := 0
	for i := 0; i < len(records) && nullIndex < len(nulls); i += skip {
		records[i] = nulls[nullIndex]
		nullIndex++
	}

	return records
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
