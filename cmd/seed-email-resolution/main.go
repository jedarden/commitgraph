// seed-email-resolution seeds email_resolution from claude-leaderboard's frozen author_login_cache.
//
// This command reads all 349,425 (email, login, resolved_at) triples from
// ~/backups/claude-leaderboard/hot.db and submits them through the identity
// ingest endpoint with source='seed'.
//
// Usage:
//
//	seed-email-resolution -db-host <host> -db-user <user> -db-password <pass>
//
// The command:
// - Reads author_login_cache table from claude-leaderboard SQLite database
// - Validates each row (non-empty email, login, resolved_at)
// - Submits all 349,425 pairs with source='seed' and original resolved_at
// - Logs summary: pairs read, accepted (won conflict rule), rejected (lost)
// - Skips rows with empty github_login (no negative-cache seeding)
//
// See plan.md "Identity ingest endpoint" and "Explicitly out of scope" sections.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/jedarden/commitgraph/pkg/identity"
	"github.com/jedarden/commitgraph/pkg/pg"
)

var (
	// Postgres connection flags
	dbHost     = flag.String("db-host", "", "PostgreSQL host (required)")
	dbPort     = flag.String("db-port", "5432", "PostgreSQL port")
	dbName     = flag.String("db-name", "commitgraph", "PostgreSQL database name")
	dbUser     = flag.String("db-user", "", "PostgreSQL user (required)")
	dbPassword = flag.String("db-password", "", "PostgreSQL password (required)")
	sslMode    = flag.String("sslmode", "require", "PostgreSQL SSL mode")

	// Source database path
	seedDBPath = flag.String("seed-db", "",
		"Path to claude-leaderboard hot.db (required, e.g., ~/backups/claude-leaderboard/hot.db)")

	// Batch size for bulk ingest
	batchSize = flag.Int("batch-size", 1000, "Number of rows per batch (default 1000)")
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
	if *seedDBPath == "" {
		log.Fatal("error: -seed-db is required")
	}

	ctx := context.Background()

	// Connect to source SQLite database
	log.Printf("Opening claude-leaderboard SQLite database: %s\n", *seedDBPath)
	seedDB, err := sql.Open("sqlite3", *seedDBPath)
	if err != nil {
		log.Fatalf("error: failed to open SQLite database: %v\n", err)
	}
	defer seedDB.Close()

	// Verify SQLite connection works
	if err := seedDB.Ping(); err != nil {
		log.Fatalf("error: SQLite ping failed: %v\n", err)
	}

	// Read all rows from author_login_cache
	log.Println("Reading author_login_cache table...")
	rows, err := seedDB.Query("SELECT author_email, github_login, resolved_at FROM author_login_cache")
	if err != nil {
		log.Fatalf("error: failed to query author_login_cache: %v\n", err)
	}
	defer rows.Close()

	// Parse all rows into ResolutionRow structs
	var allRows []identity.ResolutionRow
	var readCount, skippedEmpty int

	for rows.Next() {
		var email, login string
		var resolvedAtStr string

		if err := rows.Scan(&email, &login, &resolvedAtStr); err != nil {
			log.Fatalf("error: failed to scan row: %v\n", err)
		}

		readCount++

		// Skip rows with empty login (no negative-cache seeding)
		if login == "" {
			skippedEmpty++
			continue
		}

		// Parse timestamp
		resolvedAt, err := time.Parse(time.RFC3339Nano, resolvedAtStr)
		if err != nil {
			log.Fatalf("error: failed to parse resolved_at %q for email %s: %v\n",
				resolvedAtStr, email, err)
		}

		allRows = append(allRows, identity.ResolutionRow{
			Email:      email,
			Login:      login,
			Source:     identity.SourceSeed,
			ResolvedAt: resolvedAt,
		})
	}

	if err := rows.Err(); err != nil {
		log.Fatalf("error: rows iteration failed: %v\n", err)
	}

	log.Printf("Read %d rows from author_login_cache (skipped %d with empty login)\n",
		readCount, skippedEmpty)
	log.Printf("Valid rows to ingest: %d\n", len(allRows))

	// Connect to PostgreSQL
	log.Printf("Connecting to PostgreSQL at %s:%s/%s\n", *dbHost, *dbPort, *dbName)
	connStr := fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=%s",
		*dbHost, *dbPort, *dbName, *dbUser, *dbPassword, *sslMode)

	postgresDB, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("error: failed to connect to PostgreSQL: %v\n", err)
	}
	defer postgresDB.Close()

	// Verify Postgres connection works
	if err := postgresDB.Ping(); err != nil {
		log.Fatalf("error: PostgreSQL ping failed: %v\n", err)
	}

	// Create identity ingester
	ingester := identity.NewIngester(pg.NewIdentityIngester(pg.NewSQLExecutor(postgresDB)))

	// Get baseline row count before ingest
	var beforeCount int
	if err := postgresDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM email_resolution").Scan(&beforeCount); err != nil {
		log.Fatalf("error: failed to get baseline row count: %v\n", err)
	}
	log.Printf("email_resolution rows before ingest: %d\n", beforeCount)

	// Ingest in batches
	startTime := time.Now()
	log.Printf("Ingesting %d rows in batches of %d...\n", len(allRows), *batchSize)

	batchNum := 0
	totalBatches := (len(allRows) + *batchSize - 1) / *batchSize
	lastProgressUpdate := startTime

	for i := 0; i < len(allRows); i += *batchSize {
		end := i + *batchSize
		if end > len(allRows) {
			end = len(allRows)
		}

		batch := allRows[i:end]
		batchNum++

		if err := ingester.IngestResolution(ctx, batch); err != nil {
			log.Fatalf("error: failed to ingest batch %d (rows %d-%d): %v\n",
				batchNum, i, end-1, err)
		}

		// Update progress every 5 batches (more frequent for full production run)
		if batchNum%5 == 0 || batchNum == totalBatches {
			now := time.Now()
			batchElapsed := now.Sub(lastProgressUpdate)
			totalElapsed := now.Sub(startTime)
			percentComplete := float64(end) / float64(len(allRows)) * 100

			// Estimate time remaining
			avgRate := float64(end) / totalElapsed.Seconds()
			rowsRemaining := len(allRows) - end
			etaSeconds := float64(rowsRemaining) / avgRate
			eta := time.Duration(etaSeconds) * time.Second

			log.Printf("  Progress: %d/%d batches (%d rows, %.1f%%) | Rate: %.0f rows/sec | ETA: %v (batch took: %v)\n",
				batchNum, totalBatches, end, percentComplete, avgRate, eta.Round(time.Second), batchElapsed.Round(time.Millisecond))
			lastProgressUpdate = now
		}
	}

	elapsed := time.Since(startTime)
	rate := float64(len(allRows)) / elapsed.Seconds()
	log.Printf("Ingest completed in %s (%.2f rows/sec)\n",
		elapsed, rate)

	// Get final row count
	var afterCount int
	if err := postgresDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM email_resolution").Scan(&afterCount); err != nil {
		log.Fatalf("error: failed to get final row count: %v\n", err)
	}

	accepted := afterCount - beforeCount
	rejected := len(allRows) - accepted
	acceptRate := float64(accepted) / float64(len(allRows)) * 100

	// Log summary
	log.Println("\n=== Seed Summary ===")
	log.Printf("Rows read from author_login_cache: %d\n", readCount)
	log.Printf("Rows skipped (empty login):        %d (%.1f%%)\n", skippedEmpty, float64(skippedEmpty)/float64(readCount)*100)
	log.Printf("Valid rows submitted:               %d\n", len(allRows))
	log.Printf("email_resolution rows before:       %d\n", beforeCount)
	log.Printf("email_resolution rows after:        %d\n", afterCount)
	log.Printf("Rows accepted (won conflict):       %d (%.1f%% of submitted)\n", accepted, acceptRate)
	log.Printf("Rows rejected (lost conflict):      %d (%.1f%% of submitted)\n", rejected, 100-acceptRate)
	log.Printf("Source:                            'seed'\n")
	log.Printf("Batch size:                         %d\n", *batchSize)
	log.Printf("Total time:                         %s\n", elapsed.Round(time.Millisecond))
	log.Printf("Average rate:                       %.2f rows/sec\n", rate)
}

// usage prints the usage message.
func usage() {
	fmt.Fprintf(os.Stderr, `Usage:
  seed-email-resolution [flags]

Flags:
  -seed-db string
        Path to claude-leaderboard hot.db (required)
        Example: ~/backups/claude-leaderboard/hot.db
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
  -batch-size int
        Number of rows per batch (default 1000)

What it does:
  1. Reads author_login_cache table from claude-leaderboard SQLite database
  2. Validates each row (non-empty email, login, resolved_at)
  3. Submits all valid pairs with:
     - source = 'seed'
     - resolved_at = original timestamp from cache (not now)
  4. Applies ON CONFLICT rule from plan.md:
     - Manual source always wins
     - Otherwise, newer resolved_at wins
  5. Logs summary: pairs read, accepted, rejected

No negative-cache entries:
  Rows with empty github_login are skipped (not seeded as unresolvable).
  The claude-leaderboard cache contains only positive resolutions.

Idempotency:
  The command is idempotent - re-running produces the same result.
  The ON CONFLICT rule ensures manual/live resolutions take precedence,
  and older seed resolutions don't overwrite newer ones.

Trust boundary:
  This is an internal-only CLI tool, cluster-access-gated, and not exposed on
  any public or user-facing surface. See plan.md "Identity ingest endpoint"
  section.
`)
	os.Exit(2)
}
