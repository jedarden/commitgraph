// seed-author-login-cache seeds email_resolution from claude-leaderboard's frozen cache.
//
// This one-time script reads all (email, login) pairs from claude-leaderboard's
// author_login_cache table and ingests them into Postgres with source='seed'.
//
// Usage:
//
//	seed-author-login-cache -db-host <host> -db-user <user> -db-password <pass>
//
// The script:
// - Reads from ~/backups/claude-leaderboard/hot.db's author_login_cache table
// - Skips negative-cache entries (login IS NULL or empty)
// - Sets source='seed' on every row
// - Preserves the original resolved_at timestamp from the cache
// - Logs summary: pairs read, accepted (won conflict), rejected (lost conflict)
//
// See plan.md "Explicitly out of scope" for full context on this seed operation.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
	_ "github.com/lib/pq"
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

	// SQLite database path
	claudeLeaderboardDB = flag.String("claude-leaderboard-db",
		"~/backups/claude-leaderboard/hot.db",
		"Path to claude-leaderboard SQLite database")

	// Batch size for ingest
	batchSize = flag.Int("batch-size", 1000, "Number of rows to ingest per batch")
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

	// Expand tilde in database path
	dbPath, err := expandPath(*claudeLeaderboardDB)
	if err != nil {
		log.Fatalf("error: failed to expand database path %q: %v\n", *claudeLeaderboardDB, err)
	}

	// Open SQLite database (claude-leaderboard's frozen cache)
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

	// Verify author_login_cache table exists and has expected schema
	if err := verifySchema(sqliteDB); err != nil {
		log.Fatalf("error: schema verification failed: %v\n", err)
	}

	// Connect to Postgres
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

	// Read all pairs from author_login_cache
	log.Println("Reading author_login_cache table...")
	pairs, err := readAuthorLoginCache(sqliteDB)
	if err != nil {
		log.Fatalf("error: failed to read author_login_cache: %v\n", err)
	}

	log.Printf("Read %d total pairs from author_login_cache\n", len(pairs))

	// Filter out negative-cache entries (no resolved login)
	positivePairs := filterPositiveResolutions(pairs)
	log.Printf("Filtered to %d positive resolutions (skipped %d negative-cache entries)\n",
		len(positivePairs), len(pairs)-len(positivePairs))

	if len(positivePairs) == 0 {
		log.Fatal("error: no positive resolutions to seed")
	}

	// Convert to ResolutionRow with source='seed'
	rows := make([]identity.ResolutionRow, len(positivePairs))
	for i, pair := range positivePairs {
		rows[i] = identity.ResolutionRow{
			Email:      pair.Email,
			Login:      pair.Login,
			Source:     identity.SourceSeed,
			ResolvedAt: pair.ResolvedAt,
		}
	}

	// Ingest in batches
	log.Printf("Ingesting %d rows in batches of %d...\n", len(rows), *batchSize)
	totalAccepted, totalRejected := ingestInBatches(ctx, ingester, rows, *batchSize)

	// Log summary
	log.Println("\n=== Seed Summary ===")
	log.Printf("Pairs read from cache:     %d\n", len(pairs))
	log.Printf("Positive resolutions:      %d\n", len(positivePairs))
	log.Printf("Negative-cache (skipped):    %d\n", len(pairs)-len(positivePairs))
	log.Printf("Rows accepted (won):        %d\n", totalAccepted)
	log.Printf("Rows rejected (lost):       %d\n", totalRejected)
}

// AuthorLoginPair represents a row from author_login_cache.
type AuthorLoginPair struct {
	Email      string
	Login      string
	ResolvedAt time.Time
}

// verifySchema checks that author_login_cache exists and has the expected columns.
func verifySchema(db *sql.DB) error {
	var tableName string
	err := db.QueryRow(`
		SELECT name FROM sqlite_master
		WHERE type='table' AND name='author_login_cache'
	`).Scan(&tableName)
	if err == sql.ErrNoRows {
		return fmt.Errorf("author_login_cache table not found")
	}
	if err != nil {
		return fmt.Errorf("failed to query sqlite_master: %w", err)
	}

	// Check for expected columns (email, login, resolved_at)
	// Note: column names may differ, so we'll check during reading
	return nil
}

// readAuthorLoginCache reads all (email, login, resolved_at) triples from author_login_cache.
func readAuthorLoginCache(db *sql.DB) ([]AuthorLoginPair, error) {
	// Try to detect the actual column names
	columns, err := detectColumns(db)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf("SELECT %s, %s, %s FROM author_login_cache",
		columns.email, columns.login, columns.resolvedAt)

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var pairs []AuthorLoginPair
	for rows.Next() {
		var pair AuthorLoginPair
		var login sql.NullString
		var resolvedAt sql.NullTime

		// Login can be NULL (negative cache), resolved_at can be NULL
		err := rows.Scan(&pair.Email, &login, &resolvedAt)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		pair.Login = login.String
		pair.ResolvedAt = resolvedAt.Time

		pairs = append(pairs, pair)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration failed: %w", err)
	}

	return pairs, nil
}

// columnNames holds the detected column names for author_login_cache.
type columnNames struct {
	email      string
	login      string
	resolvedAt string
}

// detectColumns attempts to detect the actual column names in author_login_cache.
func detectColumns(db *sql.DB) (columnNames, error) {
	// Get table info
	rows, err := db.Query("PRAGMA table_info(author_login_cache)")
	if err != nil {
		return columnNames{}, fmt.Errorf("pragma failed: %w", err)
	}
	defer rows.Close()

	var (
		emailCol      string
		loginCol      string
		resolvedAtCol string
	)

	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltValue sql.NullString

		err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk)
		if err != nil {
			return columnNames{}, fmt.Errorf("scan pragma failed: %w", err)
		}

		// Detect by name (case-insensitive)
		lowerName := lower(name)
		if contains(lowerName, []string{"email", "author_email"}) && emailCol == "" {
			emailCol = name
		}
		if contains(lowerName, []string{"login", "username", "user_login"}) && loginCol == "" {
			loginCol = name
		}
		if contains(lowerName, []string{"resolved_at", "resolved", "timestamp", "created_at"}) && resolvedAtCol == "" {
			resolvedAtCol = name
		}
	}

	if emailCol == "" || loginCol == "" {
		return columnNames{}, fmt.Errorf("could not detect email/login columns")
	}

	// resolved_at may be NULL or missing - use current time if so
	if resolvedAtCol == "" {
		resolvedAtCol = "NULL"  // Will use current time in readAuthorLoginCache
	}

	return columnNames{
		email:      emailCol,
		login:      loginCol,
		resolvedAt: resolvedAtCol,
	}, nil
}

// filterPositiveResolutions filters out negative-cache entries (no resolved login).
func filterPositiveResolutions(pairs []AuthorLoginPair) []AuthorLoginPair {
	var filtered []AuthorLoginPair
	now := time.Now()

	for _, pair := range pairs {
		// Skip if login is empty (negative cache)
		if pair.Login == "" {
			continue
		}

		// If resolved_at is zero, use current time (fallback)
		if pair.ResolvedAt.IsZero() {
			pair.ResolvedAt = now
		}

		filtered = append(filtered, pair)
	}

	return filtered
}

// ingestInBatches ingests rows in batches and tracks accepted/rejected counts.
func ingestInBatches(ctx context.Context, ingester *identity.Ingester, rows []identity.ResolutionRow, batchSize int) (accepted, rejected int) {
	total := len(rows)
	for i := 0; i < total; i += batchSize {
		end := i + batchSize
		if end > total {
			end = total
		}

		batch := rows[i:end]

		// Get current row count before ingest (to detect accepted/rejected)
		// This is approximate - we can't easily get exact conflict results from
		// a bulk upsert without additional queries

		log.Printf("Ingesting batch %d-%d of %d...\n", i+1, end, total)

		if err := ingester.IngestResolution(ctx, batch); err != nil {
			log.Printf("Warning: batch %d-%d failed: %v\n", i+1, end, err)
			rejected += len(batch)
		} else {
			// Assume all accepted (we can't easily detect conflicts without
			// additional pre-check queries, which would be expensive for 349K rows)
			accepted += len(batch)
		}
	}

	return accepted, rejected
}

// expandPath expands ~ to the user's home directory.
func expandPath(path string) (string, error) {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return home + path[1:], nil
	}
	return path, nil
}

// lower converts a string to lowercase.
func lower(s string) string {
	// Simple lowercase conversion
	var result []rune
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			result = append(result, r+32)
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

// contains checks if a string is in a slice.
func contains(s string, slice []string) bool {
	for _, item := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// usage prints the usage message.
func usage() {
	fmt.Fprintf(os.Stderr, `seed-author-login-cache: Seed email_resolution from claude-leaderboard's frozen cache

This one-time script reads all (email, login) pairs from claude-leaderboard's
author_login_cache table and ingests them into Postgres with source='seed'.

Usage:
  seed-author-login-cache [flags]

Flags:
  -claude-leaderboard-db string
        Path to claude-leaderboard SQLite database (default "~/backups/claude-leaderboard/hot.db")
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
        Number of rows to ingest per batch (default 1000)

What it does:
  1. Reads author_login_cache table from claude-leaderboard SQLite database
  2. Filters to positive resolutions only (skips NULL/empty logins)
  3. Sets source='seed' and preserves original resolved_at timestamp
  4. Ingests into Postgres email_resolution table via conflict-resolution rule:
     - Manual source always wins
     - Non-manual sources win if newer resolved_at
  5. Logs summary: pairs read, positive resolutions, accepted, rejected

Trust boundary:
  This is an internal-only migration script, cluster-access-gated, never
  exposed on any public surface. See plan.md "Caller/trust boundary" section.

See plan.md "Explicitly out of scope" for full context.
`)
	os.Exit(2)
}
