// load-admin-aliases loads admin-defined user aliases from declarative-config ConfigMap.
//
// This command reads the admin-alias-configmap.yml from declarative-config and
// upserts source_login → target_login mappings into the user_aliases table with
// reason='admin'.
//
// Usage:
//
//	load-admin-aliases -db-host <host> -db-user <user> -db-password <pass>
//
// The command:
// - Reads admin-alias-configmap.yml directly from declarative-config (no hand-copied duplicates)
// - Parses the YAML structure: provider -> list of {source, target} entries
// - Upserts each pair with reason='admin' and created_at = now()
// - Detects and logs removed aliases (no auto-delete - requires manual review)
// - Uses ON CONFLICT (source_login) DO UPDATE for idempotent upserts
//
// See plan.md "Identity ingest endpoint" section for full context.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "github.com/lib/pq"
	"github.com/jedarden/commitgraph/pkg/pg"
	"gopkg.in/yaml.v3"
)

var (
	// Postgres connection flags
	dbHost     = flag.String("db-host", "", "PostgreSQL host (required)")
	dbPort     = flag.String("db-port", "5432", "PostgreSQL port")
	dbName     = flag.String("db-name", "commitgraph", "PostgreSQL database name")
	dbUser     = flag.String("db-user", "", "PostgreSQL user (required)")
	dbPassword = flag.String("db-password", "", "PostgreSQL password (required)")
	sslMode    = flag.String("sslmode", "require", "PostgreSQL SSL mode")

	// ConfigMap path
	configMapPath = flag.String("configmap", "",
		"Path to admin-alias-configmap.yml (required, "+
			"e.g., ~/jedarden/declarative-config/k8s/ord-devimprint/commitgraph/admin-alias-configmap.yml)")

	// Removal behavior
	autoDeleteRemoved = flag.Bool("auto-delete-removed", false,
		"Automatically delete aliases removed from the ConfigMap (default false: log only)")
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
	if *configMapPath == "" {
		log.Fatal("error: -configmap is required")
	}

	ctx := context.Background()

	// Expand tilde in config map path
	configPath, err := expandPath(*configMapPath)
	if err != nil {
		log.Fatalf("error: failed to expand configmap path %q: %v\n", *configMapPath, err)
	}

	// Read and parse ConfigMap
	log.Printf("Reading ConfigMap from: %s\n", configPath)
	configMap, err := readConfigMap(configPath)
	if err != nil {
		log.Fatalf("error: failed to read ConfigMap: %v\n", err)
	}

	// Parse alias YAML from ConfigMap data
	aliases, err := parseAliasesFromConfigMap(configMap)
	if err != nil {
		log.Fatalf("error: failed to parse aliases from ConfigMap: %v\n", err)
	}

	log.Printf("Parsed %d alias entries from ConfigMap\n", len(aliases))

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

	// Create alias ingester
	ingester := pg.NewAliasIngester(postgresDB)

	// Get current admin aliases from database (for removal detection)
	currentAliases, err := ingester.GetAdminAliases(ctx)
	if err != nil {
		log.Fatalf("error: failed to read current admin aliases: %v\n", err)
	}

	log.Printf("Found %d existing admin aliases in database\n", len(currentAliases))

	// Build alias rows for upsert
	now := time.Now()
	rows := make([]pg.AliasRow, 0, len(aliases))

	for _, alias := range aliases {
		rows = append(rows, pg.AliasRow{
			SourceLogin: alias.Source,
			TargetLogin: alias.Target,
			Reason:      "admin",
			CreatedAt:   now,
		})
	}

	// Upsert aliases
	if len(rows) > 0 {
		log.Printf("Upserting %d alias entries...\n", len(rows))
		if err := ingester.UpsertAliases(ctx, rows); err != nil {
			log.Fatalf("error: failed to upsert aliases: %v\n", err)
		}
		log.Printf("Successfully upserted %d aliases\n", len(rows))
	} else {
		log.Println("No aliases to upsert (ConfigMap is empty or only contains comments)")
	}

	// Detect and handle removed aliases
	removedAliases := detectRemovedAliases(currentAliases, aliases)
	if len(removedAliases) > 0 {
		log.Printf("\nDetected %d aliases removed from ConfigMap:\n", len(removedAliases))
		for source, target := range removedAliases {
			log.Printf("  - %s → %s\n", source, target)
		}

		if *autoDeleteRemoved {
			log.Println("\nAuto-deleting removed aliases from database...")
			sourceLogins := make([]string, 0, len(removedAliases))
			for source := range removedAliases {
				sourceLogins = append(sourceLogins, source)
			}
			deleted, err := ingester.DeleteAdminAliases(ctx, sourceLogins)
			if err != nil {
				log.Fatalf("error: failed to delete removed aliases: %v\n", err)
			}
			log.Printf("Deleted %d removed aliases\n", deleted)
		} else {
			log.Println("\nRemovals logged only (no auto-delete). To delete, re-run with -auto-delete-removed=true")
			log.Println("or manually delete the rows from the user_aliases table.")
		}
	} else {
		log.Println("\nNo aliases removed from ConfigMap")
	}

	// Log summary
	log.Println("\n=== Load Summary ===")
	log.Printf("Aliases parsed from ConfigMap: %d\n", len(aliases))
	log.Printf("Aliases upserted to database:  %d\n", len(rows))
	log.Printf("Existing admin aliases (before): %d\n", len(currentAliases))
	log.Printf("Aliases removed from ConfigMap:  %d\n", len(removedAliases))
	if *autoDeleteRemoved && len(removedAliases) > 0 {
		log.Printf("Aliases deleted:                %d\n", len(removedAliases))
	}
}

// ConfigMap represents the Kubernetes ConfigMap structure.
type ConfigMap struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
	Data map[string]string `yaml:"data"`
}

// AliasEntry represents a single alias mapping from the ConfigMap.
type AliasEntry struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
}

// AliasesConfig represents the aliases.yml content from ConfigMap data.
type AliasesConfig struct {
	Github []AliasEntry `yaml:"github"`
}

// readConfigMap reads and parses the admin-alias-configmap.yml file.
func readConfigMap(path string) (*ConfigMap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file failed: %w", err)
	}

	var configMap ConfigMap
	if err := yaml.Unmarshal(data, &configMap); err != nil {
		return nil, fmt.Errorf("yaml unmarshal failed: %w", err)
	}

	// Validate it's actually a ConfigMap
	if configMap.Kind != "ConfigMap" {
		return nil, fmt.Errorf("unexpected kind %q (expected ConfigMap)", configMap.Kind)
	}

	return &configMap, nil
}

// parseAliasesFromConfigMap extracts alias entries from the ConfigMap data field.
func parseAliasesFromConfigMap(configMap *ConfigMap) ([]AliasEntry, error) {
	aliasesYAML, ok := configMap.Data["aliases.yml"]
	if !ok {
		return nil, fmt.Errorf("ConfigMap missing aliases.yml data field")
	}

	var config AliasesConfig
	if err := yaml.Unmarshal([]byte(aliasesYAML), &config); err != nil {
		return nil, fmt.Errorf("failed to parse aliases.yml: %w", err)
	}

	return config.Github, nil
}

// detectRemovedAliases finds source_logins that exist in the database but not
// in the current ConfigMap.
func detectRemovedAliases(currentDB map[string]string, configMapEntries []AliasEntry) map[string]string {
	// Build a map of current ConfigMap entries for quick lookup
	configMapSet := make(map[string]string)
	for _, entry := range configMapEntries {
		configMapSet[entry.Source] = entry.Target
	}

	// Find aliases in DB but not in ConfigMap
	removed := make(map[string]string)
	for source, target := range currentDB {
		if _, exists := configMapSet[source]; !exists {
			removed[source] = target
		}
	}

	return removed
}

// expandPath expands ~ to the user's home directory and resolves the path.
func expandPath(path string) (string, error) {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = home + path[1:]
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	return absPath, nil
}

// usage prints the usage message.
func usage() {
	fmt.Fprintf(os.Stderr, `load-admin-aliases: Load admin-defined user aliases from ConfigMap

This command reads admin-alias-configmap.yml from declarative-config and
upserts source_login → target_login mappings into the user_aliases table
with reason='admin'.

Usage:
  load-admin-aliases [flags]

Flags:
  -configmap string
        Path to admin-alias-configmap.yml (required)
        Example: ~/jedarden/declarative-config/k8s/ord-devimprint/commitgraph/admin-alias-configmap.yml
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
  -auto-delete-removed
        Automatically delete aliases removed from the ConfigMap (default false: log only)

What it does:
  1. Reads admin-alias-configmap.yml from declarative-config
  2. Parses the YAML structure: github -> list of {source, target} entries
  3. Upserts each pair to user_aliases table:
     - reason = 'admin'
     - created_at = now()
  4. Detects and logs aliases removed from ConfigMap since last run
  5. Optionally auto-deletes removed aliases (requires -auto-delete-removed=true)

Idempotency:
  The command is idempotent - re-running after ConfigMap changes updates
  existing rows via ON CONFLICT (source_login) DO UPDATE, rather than
  erroring or duplicating.

Removal behavior:
  Removed aliases are always logged. The default behavior is to NOT auto-delete
  (requiring manual review). To enable automatic deletion, pass the
  -auto-delete-removed=true flag.

  Decision rationale:
  - Admin aliases are operator-curated identity mappings
  - Accidental removal from the ConfigMap shouldn't immediately delete from the DB
  - Logging-only provides a safety net while still surfacing changes
  - Auto-delete is available for operators who want GitOps-style sync behavior

Trust boundary:
  This is an internal-only CLI tool, cluster-access-gated, and not exposed on
  any public or user-facing surface. See plan.md "Identity ingest endpoint"
  section.
`)
	os.Exit(2)
}
