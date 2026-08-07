// repo-admin is an internal-only CLI tool for repo-level exclusion operations.
//
// This tool is cluster-access-gated and not exposed on any public or
// user-facing surface, consistent with the trust-boundary pattern from
// plan.md's threat model section.
//
// Usage:
//
//	repo-admin -provider github -repo owner/repo -reason "false attribution report from user@example.com"
//	repo-admin -provider github -repo owner/repo -clear
//
// Every exclusion/un-exclusion action is logged to the audit log.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
	"github.com/jedarden/commitgraph/pkg/audit"
	"github.com/jedarden/commitgraph/pkg/service"
)

var (
	// Connection flags
	dbHost     = flag.String("db-host", "", "PostgreSQL host (required)")
	dbPort     = flag.String("db-port", "5432", "PostgreSQL port")
	dbName     = flag.String("db-name", "commitgraph", "PostgreSQL database name")
	dbUser     = flag.String("db-user", "", "PostgreSQL user (required)")
	dbPassword = flag.String("db-password", "", "PostgreSQL password (required, use env var in production)")
	sslMode    = flag.String("sslmode", "require", "PostgreSQL SSL mode")

	// Operation flags
	provider = flag.String("provider", "", "Repository provider (required, e.g., 'github')")
	repo     = flag.String("repo", "", "Repository full name in owner/repo format (required)")
	reason   = flag.String("reason", "", "Reason for exclusion (required when setting, ignored when clearing)")
	clear    = flag.Bool("clear", false, "Clear exclusion instead of setting it")

	// Operator flag (for audit logging)
	operator = flag.String("operator", "", "Operator performing this action (required for audit)")
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

	// Validate required operation flags
	if *provider == "" {
		log.Fatal("error: -provider is required")
	}
	if *repo == "" {
		log.Fatal("error: -repo is required")
	}

	// Validate operator flag (required for audit logging)
	if *operator == "" {
		log.Fatal("error: -operator is required (for audit logging)")
	}

	// Validate reason flag (required when setting, not when clearing)
	if !*clear && *reason == "" {
		log.Fatal("error: -reason is required when setting an exclusion")
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

	// Wrap DB for use with service layer
	sqlDB := service.NewSQLDB(db)

	if *clear {
		doClear(ctx, sqlDB)
	} else {
		doExclude(ctx, sqlDB)
	}
}

func doExclude(ctx context.Context, db *service.SQLDB) {
	log.Printf("Setting exclusion for %s/%s...\n", *provider, *repo)

	err := service.SetRepoExclusionWithActor(ctx, db, *provider, *repo, *reason, *operator)
	if err != nil {
		log.Fatalf("error: failed to set exclusion: %v\n", err)
	}

	log.Printf("Successfully excluded %s/%s (reason: %s)\n", *provider, *repo, *reason)

	// Audit log entry
	audit.LogExclusionInline(
		"exclude",   // operation
		*provider,   // provider
		*repo,       // repo_full_name
		*operator,   // operator
		*reason,     // reason
		1,           // rows_affected (service confirmed repo exists)
		"",          // incident_id (optional)
	)
}

func doClear(ctx context.Context, db *service.SQLDB) {
	log.Printf("Clearing exclusion for %s/%s...\n", *provider, *repo)

	err := service.ClearRepoExclusionWithActor(ctx, db, *provider, *repo, *operator)
	if err != nil {
		log.Fatalf("error: failed to clear exclusion: %v\n", err)
	}

	log.Printf("Successfully cleared exclusion for %s/%s\n", *provider, *repo)

	// Audit log entry
	audit.LogExclusionInline(
		"clear",     // operation
		*provider,   // provider
		*repo,       // repo_full_name
		*operator,   // operator
		"",          // reason (empty for clear)
		1,           // rows_affected (service confirmed repo exists)
		"",          // incident_id (optional)
	)
}

func usage() {
	fmt.Fprintf(os.Stderr, `repo-admin: internal-only CLI for repo-level exclusion operations

This tool applies or clears repo-level exclusions to mitigate false attribution
threats (see plan.md "Threat model" section). It is cluster-access-gated and
not exposed on any public surface.

Usage:
  repo-admin [flags]

Flags:
  -provider string
        Repository provider (required, e.g., "github")
  -repo string
        Repository full name in owner/repo format (required)
  -reason string
        Reason for exclusion (required when setting, ignored when clearing)
  -clear
        Clear exclusion instead of setting it
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
  -operator string
        Operator performing this action (required for audit logging)

Examples:
  # Set exclusion for a repo
  repo-admin -provider github -repo owner/repo -reason "false attribution from user@example.com"

  # Clear exclusion for a repo
  repo-admin -provider github -repo owner/repo -clear

Audit:
  Every exclusion/clear action is logged with:
  - Who (operator flag)
  - When (timestamp)
  - Why (exclusion reason or "clear" reversal)
  - What (provider/repo_full_name)

  This feeds q-threat-exclusion-audit-log for incident response.

Trust boundary:
  This tool is internal-only, cluster-access-gated, and not exposed on any
  public or user-facing surface. Consistent with plan.md's stated pattern
  for internal-only endpoints (ingest path, seed endpoint).
`)
	os.Exit(2)
}
