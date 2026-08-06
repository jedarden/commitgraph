// repo-admin is an internal-only CLI tool for repo-level exclusion operations.
//
// This tool is cluster-access-gated and not exposed on any public or
// user-facing surface, consistent with the trust-boundary pattern from
// plan.md's threat model section.
//
// Usage:
//
//	repo-admin exclude github owner/repo "false attribution report from user@example.com"
//	repo-admin clear github owner/repo
//	repo-admin status github owner/repo
//	repo-admin list
//
// Every exclusion/un-exclusion action is logged to the audit log.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jedarden/commitgraph/pkg/audit"
	"github.com/jedarden/commitgraph/pkg/pg"
)

var (
	// Connection flags
	dbHost     = flag.String("db-host", "", "PostgreSQL host (required)")
	dbPort     = flag.String("db-port", "5432", "PostgreSQL port")
	dbName     = flag.String("db-name", "commitgraph", "PostgreSQL database name")
	dbUser     = flag.String("db-user", "", "PostgreSQL user (required)")
	dbPassword = flag.String("db-password", "", "PostgreSQL password (required, use env var in production)")
	sslMode    = flag.String("sslmode", "require", "PostgreSQL SSL mode")

	// Operator flag (for audit logging)
	operator = flag.String("operator", "", "Operator performing this action (required for audit)")
)

func main() {
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() < 1 {
		log.Fatal("error: command required (exclude, clear, status, list)")
	}

	// Validate required flags
	if *dbHost == "" {
		log.Fatal("error: -db-host is required")
	}
	if *dbUser == "" {
		log.Fatal("error: -db-user is required")
	}
	if *dbPassword == "" {
		log.Fatal("error: -db-password is required")
	}
	if *operator == "" {
		log.Fatal("error: -operator is required (for audit logging)")
	}

	ctx := context.Background()

	// Connect to database (would use database/sql in real implementation)
	// For now, this is a placeholder that demonstrates the structure
	excluder := pg.NewRepoExcluder(nil /* real DB connection */)

	command := flag.Arg(0)

	switch command {
	case "exclude":
		if flag.NArg() != 4 {
			log.Fatalf("usage: repo-admin exclude <provider> <repo-full-name> <reason>\n")
		}
		doExclude(ctx, excluder, flag.Arg(1), flag.Arg(2), flag.Arg(3))

	case "clear":
		if flag.NArg() != 3 {
			log.Fatalf("usage: repo-admin clear <provider> <repo-full-name>\n")
		}
		doClear(ctx, excluder, flag.Arg(1), flag.Arg(2))

	case "status":
		if flag.NArg() != 3 {
			log.Fatalf("usage: repo-admin status <provider> <repo-full-name>\n")
		}
		doStatus(ctx, excluder, flag.Arg(1), flag.Arg(2))

	case "list":
		if flag.NArg() != 1 {
			log.Fatalf("usage: repo-admin list\n")
		}
		doList(ctx, excluder)

	default:
		log.Fatalf("error: unknown command %q\n", command)
	}
}

func doExclude(ctx context.Context, excluder *pg.RepoExcluder, provider, repoFullName, reason string) {
	now := time.Now()
	req := pg.ExclusionRequest{
		Provider:       provider,
		RepoFullName:   repoFullName,
		ExcludedAt:     &now,
		ExcludedReason: reason,
		Operator:       *operator,
	}

	rows, err := excluder.ApplyExclusion(ctx, req)
	if err != nil {
		log.Fatalf("error: %v\n", err)
	}

	if rows == 0 {
		log.Printf("warning: repo %s/%s not found in database\n", provider, repoFullName)
	} else {
		log.Printf("excluded %s/%s (reason: %s)\n", provider, repoFullName, reason)
	}

	// Audit log entry (q-threat-exclusion-audit-log)
	auditLog("exclude", provider, repoFullName, reason, rows)
}

func doClear(ctx context.Context, excluder *pg.RepoExcluder, provider, repoFullName string) {
	req := pg.ExclusionRequest{
		Provider:     provider,
		RepoFullName: repoFullName,
		ExcludedAt:   nil, // NULL for clear
		Operator:     *operator,
	}

	rows, err := excluder.ApplyExclusion(ctx, req)
	if err != nil {
		log.Fatalf("error: %v\n", err)
	}

	if rows == 0 {
		log.Printf("warning: repo %s/%s not found in database\n", provider, repoFullName)
	} else {
		log.Printf("cleared exclusion for %s/%s\n", provider, repoFullName)
	}

	// Audit log entry
	auditLog("clear", provider, repoFullName, "", rows)
}

func doStatus(ctx context.Context, excluder *pg.RepoExcluder, provider, repoFullName string) {
	excludedAt, reason, err := excluder.GetExclusion(ctx, provider, repoFullName)
	if err != nil {
		log.Fatalf("error: %v\n", err)
	}

	if excludedAt == nil {
		fmt.Printf("%s/%s: not excluded\n", provider, repoFullName)
	} else {
		fmt.Printf("%s/%s: excluded since %s (reason: %s)\n", provider, repoFullName, excludedAt.Format(time.RFC3339), reason)
	}
}

func doList(ctx context.Context, excluder *pg.RepoExcluder) {
	exclusions, err := excluder.ListExclusions(ctx)
	if err != nil {
		log.Fatalf("error: %v\n", err)
	}

	if len(exclusions) == 0 {
		fmt.Println("no excluded repos")
		return
	}

	fmt.Printf("Found %d excluded repo(s):\n\n", len(exclusions))
	for _, ex := range exclusions {
		fmt.Printf("  %s/%s\n", ex.Provider, ex.RepoFullName)
		fmt.Printf("    Excluded: %s\n", ex.ExcludedAt.Format(time.RFC3339))
		fmt.Printf("    Reason:  %s\n\n", ex.ExcludedReason)
	}
}

// auditLog writes an audit log entry for every exclusion/un-exclusion action.
//
// This feeds the q-threat-exclusion-audit-log, capturing who/when/why for
// incident response and postmortem analysis.
func auditLog(op, provider, repoFullName, reason string, rowsAffected int64) {
	// Use the audit logger for structured logging
	audit.LogExclusionInline(
		op,           // operation: "exclude" or "clear"
		provider,     // provider: e.g., "github"
		repoFullName, // repo_full_name: e.g., "owner/repo"
		*operator,    // operator: who performed this action
		reason,       // reason: why (exclusion reason or empty for clear)
		rowsAffected, // rows_affected: 1 if repo existed, 0 if not found
		"",           // incident_id: optional (can be added via flag later)
	)
}

func usage() {
	fmt.Fprintf(os.Stderr, `repo-admin: internal-only CLI for repo-level exclusion operations

This tool applies or clears repo-level exclusions to mitigate false attribution
threats (see plan.md "Threat model" section). It is cluster-access-gated and
not exposed on any public surface.

Usage:
  repo-admin [flags] <command>

Commands:
  exclude <provider> <repo-full-name> <reason>
        Apply exclusion to a repo (requires human-readable reason)
        Example: repo-admin exclude github owner/repo "false attribution from user@example.com"

  clear <provider> <repo-full-name>
        Remove exclusion from a repo (restores contribution on next aggregation)
        Example: repo-admin clear github owner/repo

  status <provider> <repo-full-name>
        Show current exclusion status for a repo
        Example: repo-admin status github owner/repo

  list
        List all currently excluded repos
        Example: repo-admin list

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
        PostgreSQL password (required, use env var in production)
  -sslmode string
        PostgreSQL SSL mode (default "require")
  -operator string
        Operator performing this action (required for audit logging)

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
