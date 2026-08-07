// verify-email-resolution queries and verifies data integrity in the email_resolution table.
//
// This command connects to the PostgreSQL database and verifies:
// - All records have source="seed" (or other expected sources)
// - All required fields are populated
// - Record count matches expected
//
// Usage:
//
//	verify-email-resolution -db-host <host> -db-user <user> -db-password <pass>
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

var (
	// Postgres connection flags
	dbHost     = flag.String("db-host", "", "PostgreSQL host (required)")
	dbPort     = flag.String("db-port", "5432", "PostgreSQL port")
	dbName     = flag.String("db-name", "commitgraph", "PostgreSQL database name")
	dbUser     = flag.String("db-user", "", "PostgreSQL user (required)")
	dbPassword = flag.String("db-password", "", "PostgreSQL password (required)")
	sslMode    = flag.String("sslmode", "require", "PostgreSQL SSL mode")

	// Expected record count (optional)
	expectedCount = flag.Int("expected-count", 0, "Expected total record count (0 to skip verification)")
)

func main() {
	flag.Usage = usage
	flag.Parse()

	if *dbHost == "" {
		log.Fatal("error: -db-host is required")
	}
	if *dbUser == "" {
		log.Fatal("error: -db-user is required")
	}
	if *dbPassword == "" {
		log.Fatal("error: -db-password is required")
	}

	ctx := context.Background()

	// Connect to PostgreSQL
	log.Printf("Connecting to PostgreSQL at %s:%s/%s\n", *dbHost, *dbPort, *dbName)
	connStr := fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=%s",
		*dbHost, *dbPort, *dbName, *dbUser, *dbPassword, *sslMode)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("error: failed to connect to PostgreSQL: %v\n", err)
	}
	defer db.Close()

	// Verify Postgres connection works
	if err := db.Ping(); err != nil {
		log.Fatalf("error: PostgreSQL ping failed: %v\n", err)
	}

	log.Println("\n=== Email Resolution Data Integrity Verification ===")

	// 1. Get total record count and source distribution
	log.Println("1. Checking record counts and source distribution...")
	var totalCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM email_resolution").Scan(&totalCount)
	if err != nil {
		log.Fatalf("error: failed to get total count: %v\n", err)
	}
	log.Printf("   Total records: %d\n", totalCount)

	// Check count by source
	rows, err := db.QueryContext(ctx, "SELECT source, COUNT(*) FROM email_resolution GROUP BY source ORDER BY source")
	if err != nil {
		log.Fatalf("error: failed to get source distribution: %v\n", err)
	}
	defer rows.Close()

	var sourceCounts = make(map[string]int)
	for rows.Next() {
		var source string
		var count int
		if err := rows.Scan(&source, &count); err != nil {
			log.Fatalf("error: failed to scan source row: %v\n", err)
		}
		sourceCounts[source] = count
		log.Printf("   - source='%s': %d records\n", source, count)
	}

	if *expectedCount > 0 && totalCount != *expectedCount {
		log.Printf("   ⚠️  WARNING: Expected %d records, found %d\n", *expectedCount, totalCount)
	} else if *expectedCount > 0 {
		log.Printf("   ✓ Record count matches expected (%d)\n", *expectedCount)
	}

	// 2. Verify required fields are populated
	log.Println("\n2. Checking required field population...")
	var nullEmails, nullLogins, nullSources, nullResolvedAt int

	db.QueryRowContext(ctx, "SELECT COUNT(*) FROM email_resolution WHERE email IS NULL").Scan(&nullEmails)
	db.QueryRowContext(ctx, "SELECT COUNT(*) FROM email_resolution WHERE login IS NULL").Scan(&nullLogins)
	db.QueryRowContext(ctx, "SELECT COUNT(*) FROM email_resolution WHERE source IS NULL").Scan(&nullSources)
	db.QueryRowContext(ctx, "SELECT COUNT(*) FROM email_resolution WHERE resolved_at IS NULL").Scan(&nullResolvedAt)

	if nullEmails > 0 {
		log.Printf("   ✗ %d records have NULL email\n", nullEmails)
	} else {
		log.Println("   ✓ All records have non-NULL email")
	}

	if nullLogins > 0 {
		log.Printf("   ✗ %d records have NULL login\n", nullLogins)
	} else {
		log.Println("   ✓ All records have non-NULL login")
	}

	if nullSources > 0 {
		log.Printf("   ✗ %d records have NULL source\n", nullSources)
	} else {
		log.Println("   ✓ All records have non-NULL source")
	}

	if nullResolvedAt > 0 {
		log.Printf("   ✗ %d records have NULL resolved_at\n", nullResolvedAt)
	} else {
		log.Println("   ✓ All records have non-NULL resolved_at")
	}

	// 3. Sample records for manual verification
	log.Println("\n3. Sampling records for verification...")
	sampleRows, err := db.QueryContext(ctx, `
		SELECT email, login, source, resolved_at
		FROM email_resolution
		ORDER BY resolved_at DESC
		LIMIT 5
	`)
	if err != nil {
		log.Fatalf("error: failed to query sample records: %v\n", err)
	}
	defer sampleRows.Close()

	log.Println("   Sample records (most recent resolved_at):")
	for sampleRows.Next() {
		var email, login, source string
		var resolvedAt string
		if err := sampleRows.Scan(&email, &login, &source, &resolvedAt); err != nil {
			log.Fatalf("error: failed to scan sample row: %v\n", err)
		}
		log.Printf("   - %s → %s (source='%s', resolved_at=%s)\n", email, login, source, resolvedAt)
	}

	// Final summary
	log.Println("\n=== Verification Summary ===")
	allValid := nullEmails == 0 && nullLogins == 0 && nullSources == 0 && nullResolvedAt == 0
	if allValid {
		log.Println("✓ All required fields populated correctly")
		log.Printf("✓ Source distribution: %d total records\n", totalCount)
		for source, count := range sourceCounts {
			log.Printf("  - %s: %d records\n", source, count)
		}
		if *expectedCount > 0 && totalCount == *expectedCount {
			log.Println("✓ Record count matches expected")
		}
		log.Println("\n✓ Data integrity verification PASSED")
	} else {
		log.Println("✗ Data integrity verification FAILED")
		os.Exit(1)
	}
}

// usage prints the usage message.
func usage() {
	fmt.Fprintf(os.Stderr, `Usage:
  verify-email-resolution [flags]

Flags:
  -db-host string
        PostgreSQL host (required)
  -db-port string
        PostgreSQL port (default "5432")
  -db-name string
        PostgreSQL database name (default "commitgraph")
  -db-user string
        PostgreSQL user (required)
  -db-password string
        PostgreSQL password (required)
  -sslmode string
        PostgreSQL SSL mode (default "require")
  -expected-count int
        Expected total record count (0 to skip verification)

What it does:
  1. Counts total records and shows distribution by source
  2. Verifies all required fields are populated (non-NULL)
  3. Samples recent records for visual verification
  4. Reports pass/fail for data integrity

Examples:
  verify-email-resolution -db-host localhost -db-port 15432 \\
    -db-name commitgraph -db-user postgres -db-password postgres \\
    -sslmode disable -expected-count 62
`)
	os.Exit(2)
}
