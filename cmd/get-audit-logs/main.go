// get-audit-logs is a CLI tool for querying exclusion audit logs.
//
// This tool retrieves audit log records with filtering and pagination support.
// It outputs structured JSON or table-formatted text for human review.
//
// Usage:
//
//	get-audit-logs -output json
//	get-audit-logs -repo-id 123 -output table
//	get-audit-logs -actor admin -event-type exclude -start-date 2024-01-01
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/jedarden/commitgraph/pkg/audit"
)

var (
	// Connection flags
	dbHost     = flag.String("db-host", "", "PostgreSQL host (required)")
	dbPort     = flag.String("db-port", "5432", "PostgreSQL port")
	dbName     = flag.String("db-name", "commitgraph", "PostgreSQL database name")
	dbUser     = flag.String("db-user", "", "PostgreSQL user (required)")
	dbPassword = flag.String("db-password", "", "PostgreSQL password (required, use env var in production)")
	sslMode    = flag.String("sslmode", "require", "PostgreSQL SSL mode")

	// Filter flags
	repoID     = flag.Int64("repo-id", 0, "Filter by repository ID (0 = all repos)")
	actor      = flag.String("actor", "", "Filter by actor (empty = all actors)")
	eventType  = flag.String("event-type", "", "Filter by event type ('exclude' or 'unexclude', empty = all)")
	startDate  = flag.String("start-date", "", "Start date for filtering (YYYY-MM-DD format, inclusive)")
	endDate    = flag.String("end-date", "", "End date for filtering (YYYY-MM-DD format, inclusive)")
	activeOnly = flag.Bool("active-only", false, "Show only currently active exclusions (overrides other filters)")
	longstanding = flag.Int("longstanding", 0, "Show exclusions older than N days (requires active-only=true)")

	// Pagination flags
	offset = flag.Int("offset", 0, "Pagination offset (0 = first page)")
	limit  = flag.Int("limit", 100, "Limit results (0 = use default of 100)")

	// Output format
	outputFormat = flag.String("output", "table", "Output format: 'json' or 'table'")
	showCount    = flag.Bool("count", false, "Show total count of matching records (no pagination)")
)

func main() {
	flag.Usage = usage
	flag.Parse()

	// Validate required connection flags
	if *dbHost == "" {
		log.Fatal("error: -db-host is required")
	}
	if *dbUser == "" {
		log.Fatal("error: -db-user is required")
	}
	if *dbPassword == "" {
		log.Fatal("error: -db-password is required")
	}

	// Validate output format
	if *outputFormat != "json" && *outputFormat != "table" {
		log.Fatal("error: -output must be 'json' or 'table'")
	}

	// Validate longstanding flag
	if *longstanding > 0 && !*activeOnly {
		log.Fatal("error: -longstanding requires -active-only=true")
	}

	ctx := context.Background()

	// Connect to database
	connStr := fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=%s",
		*dbHost, *dbPort, *dbName, *dbUser, *dbPassword, *sslMode)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("error: failed to connect to PostgreSQL: %v\n", err)
	}
	defer db.Close()

	// Verify connection works
	if err := db.Ping(); err != nil {
		log.Fatalf("error: PostgreSQL ping failed: %v\n", err)
	}

	querier := audit.NewExclusionAuditQuerier(db)

	if *showCount {
		showRecordsCount(ctx, querier)
		return
	}

	if *activeOnly {
		showActiveExclusions(ctx, querier)
	} else {
		showAuditLogs(ctx, querier)
	}
}

func showAuditLogs(ctx context.Context, querier *audit.ExclusionAuditQuerier) {
	// Parse date filters
	var parsedStart, parsedEnd time.Time
	var err error

	if startDateFlag := *startDate; startDateFlag != "" {
		parsedStart, err = parseDate(startDateFlag)
		if err != nil {
			log.Fatalf("error: invalid start-date: %v\n", err)
		}
	}
	if endDateFlag := *endDate; endDateFlag != "" {
		parsedEnd, err = parseDate(endDateFlag)
		if err != nil {
			log.Fatalf("error: invalid end-date: %v\n", err)
		}
		// Make end date inclusive (end of day)
		parsedEnd = parsedEnd.Add(24*time.Hour - time.Second)
	}

	opts := audit.ExclusionAuditQueryOptions{
		RepoID:    *repoID,
		Actor:     *actor,
		EventType: *eventType,
		StartDate: parsedStart,
		EndDate:   parsedEnd,
		Offset:    *offset,
		Limit:     *limit,
	}

	records, err := querier.QueryExclusionAuditLogs(ctx, opts)
	if err != nil {
		log.Fatalf("error: failed to query audit logs: %v\n", err)
	}

	if *outputFormat == "json" {
		outputJSON(records)
	} else {
		outputTable(records)
	}
}

func showActiveExclusions(ctx context.Context, querier *audit.ExclusionAuditQuerier) {
	records, err := querier.GetActiveExclusions(ctx)
	if err != nil {
		log.Fatalf("error: failed to query active exclusions: %v\n", err)
	}

	if *longstanding > 0 {
		// Filter by duration
		minDuration := time.Duration(*longstanding) * 24 * time.Hour
		longstandingRecords, err := querier.GetLongstandingExclusions(ctx, minDuration)
		if err != nil {
			log.Fatalf("error: failed to query longstanding exclusions: %v\n", err)
		}

		if *outputFormat == "json" {
			outputJSONLongstanding(longstandingRecords)
		} else {
			outputTableLongstanding(longstandingRecords)
		}
	} else {
		if *outputFormat == "json" {
			outputJSON(records)
		} else {
			outputTable(records)
		}
	}
}

func showRecordsCount(ctx context.Context, querier *audit.ExclusionAuditQuerier) {
	// Parse date filters
	var parsedStart, parsedEnd time.Time
	var err error

	if startDateFlag := *startDate; startDateFlag != "" {
		parsedStart, err = parseDate(startDateFlag)
		if err != nil {
			log.Fatalf("error: invalid start-date: %v\n", err)
		}
	}
	if endDateFlag := *endDate; endDateFlag != "" {
		parsedEnd, err = parseDate(endDateFlag)
		if err != nil {
			log.Fatalf("error: invalid end-date: %v\n", err)
		}
		// Make end date inclusive (end of day)
		parsedEnd = parsedEnd.Add(24*time.Hour - time.Second)
	}

	opts := audit.ExclusionAuditQueryOptions{
		RepoID:    *repoID,
		Actor:     *actor,
		EventType: *eventType,
		StartDate: parsedStart,
		EndDate:   parsedEnd,
	}

	count, err := querier.CountExclusionAuditLogs(ctx, opts)
	if err != nil {
		log.Fatalf("error: failed to count audit logs: %v\n", err)
	}

	fmt.Printf("Total matching records: %d\n", count)
}

func parseDate(dateStr string) (time.Time, error) {
	return time.Parse("2006-01-02", dateStr)
}

func outputJSON(records []audit.ExclusionAuditRecord) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(records); err != nil {
		log.Fatalf("error: failed to encode JSON: %v\n", err)
	}
}

func outputJSONLongstanding(records []audit.LongstandingExclusionV2) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(records); err != nil {
		log.Fatalf("error: failed to encode JSON: %v\n", err)
	}
}

func outputTable(records []audit.ExclusionAuditRecord) {
	if len(records) == 0 {
		fmt.Println("No records found.")
		return
	}

	// Print table header
	fmt.Println("┃ ID     │ Repo ID │ Actor      │ Timestamp           │ Event    │ Old Excluded │ New Excluded │ Reason")
	fmt.Println("┃───────┼─────────┼────────────┼─────────────────────┼──────────┼──────────────┼──────────────┼────────")

	for _, rec := range records {
		timestamp := rec.Timestamp.Format("2006-01-02 15:04:05")
		oldExcluded := formatNullableTime(rec.OldExcludedAt)
		newExcluded := formatNullableTime(rec.NewExcludedAt)
		reason := formatNullableString(rec.NewExcludedReason)

		fmt.Printf("┃ %-5d │ %-7d │ %-10s │ %-19s │ %-8s │ %-12s │ %-12s │ %s\n",
			rec.ID, rec.RepoID, rec.Actor, timestamp, rec.EventType, oldExcluded, newExcluded, reason)
	}

	fmt.Printf("\nTotal: %d records\n", len(records))
}

func outputTableLongstanding(records []audit.LongstandingExclusionV2) {
	if len(records) == 0 {
		fmt.Println("No longstanding exclusions found.")
		return
	}

	// Print table header
	fmt.Println("┃ Repo ID │ Provider  │ Repo Full Name         │ Excluded At         │ Duration │ Actor      │ Reason")
	fmt.Println("┃─────────┼───────────┼────────────────────────┼─────────────────────┼──────────┼────────────┼────────")

	for _, rec := range records {
		excludedAt := rec.ExcludedAt.Format("2006-01-02 15:04:05")
		duration := fmt.Sprintf("%d days", int(rec.Duration.Hours()/24))
		reason := rec.Reason
		if len(reason) > 40 {
			reason = reason[:37] + "..."
		}

		fmt.Printf("┃ %-7d │ %-9s │ %-22s │ %-19s │ %-8s │ %-10s │ %s\n",
			rec.RepoID, rec.Provider, rec.RepoFullName, excludedAt, duration, rec.Actor, reason)
	}

	fmt.Printf("\nTotal: %d longstanding exclusions\n", len(records))
}

func formatNullableTime(t *time.Time) string {
	if t == nil {
		return "NULL"
	}
	return t.Format("2006-01-02")
}

func formatNullableString(s *string) string {
	if s == nil {
		return "NULL"
	}
	return *s
}

func usage() {
	fmt.Fprintf(os.Stderr, `get-audit-logs: CLI tool for querying exclusion audit logs

This tool retrieves audit log records with filtering and pagination support.
Outputs structured JSON or table-formatted text.

Usage:
  get-audit-logs [flags]

Flags:
  Connection:
    -db-host string
        PostgreSQL host (required)
    -db-port string
        PostgreSQL port (default "5432")
    -db-name string
        PostgreSQL database name (default "commitgraph")
    -db-user string
        PostgreSQL user (required)
    -db-password string
        PostgreSQL password (required, use env var in production)
    -sslmode string
        PostgreSQL SSL mode (default "require")

  Filters:
    -repo-id int
        Filter by repository ID (default 0 = all repos)
    -actor string
        Filter by actor (empty = all actors)
    -event-type string
        Filter by event type ('exclude' or 'unexclude', empty = all)
    -start-date string
        Start date for filtering (YYYY-MM-DD format, inclusive)
    -end-date string
        End date for filtering (YYYY-MM-DD format, inclusive)
    -active-only
        Show only currently active exclusions (overrides other filters)
    -longstanding int
        Show exclusions older than N days (requires -active-only=true)

  Pagination:
    -offset int
        Pagination offset (default 0 = first page)
    -limit int
        Limit results (default 100, 0 = use default)

  Output:
    -output string
        Output format: 'json' or 'table' (default "table")
    -count
        Show total count of matching records (no pagination)

Examples:
  # Get all audit logs as JSON
  get-audit-logs -output json

  # Get audit logs for a specific repository
  get-audit-logs -repo-id 123

  # Get audit logs by actor with date range
  get-audit-logs -actor admin -start-date 2024-01-01 -end-date 2024-12-31

  # Get only active exclusions
  get-audit-logs -active-only

  # Get exclusions older than 30 days (alerting on stale exclusions)
  get-audit-logs -active-only -longstanding 30

  # Count total records matching filters
  get-audit-logs -actor admin -count

Retention Policy:
  Audit logs should be retained for 90 days by default.
  This is configurable via database retention policies or
  application-level cleanup jobs. See docs/plan.md for details.

Output Formats:
  json:   Full structured JSON with all fields
  table:  Human-readable table format for terminal review

Trust boundary:
  This tool is internal-only and requires database credentials.
  It should be cluster-access-gated like repo-admin.
`)
	os.Exit(2)
}
