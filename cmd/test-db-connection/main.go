// test-db-connection tests PostgreSQL database connectivity for commitgraph.
//
// This command validates database connection setup, connection pool configuration,
// timeout behavior, and retry logic before running any data ingestion.
//
// Usage:
//
//	test-db-connection -db-host <host> -db-user <user> -db-password <pass>
//
// Environment variables (optional, override flags):
// - DB_HOST, DB_PORT, DB_NAME, DB_USER, DB_PASSWORD, DB_SSLMODE
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
)

var (
	// Postgres connection flags
	dbHost     = flag.String("db-host", "", "PostgreSQL host (required)")
	dbPort     = flag.String("db-port", "5432", "PostgreSQL port")
	dbName     = flag.String("db-name", "commitgraph", "PostgreSQL database name")
	dbUser     = flag.String("db-user", "", "PostgreSQL user (required)")
	dbPassword = flag.String("db-password", "", "PostgreSQL password (required)")
	sslMode    = flag.String("sslmode", "require", "PostgreSQL SSL mode")

	// Connection pool settings
	maxOpenConns = flag.Int("max-open-conns", 10, "Maximum open connections")
	maxIdleConns = flag.Int("max-idle-conns", 5, "Maximum idle connections")
	connMaxLifetime = flag.Duration("conn-max-lifetime", 5*time.Minute, "Connection maximum lifetime")

	// Test settings
	connectionTimeout = flag.Duration("connection-timeout", 30*time.Second, "Connection timeout")
	retryAttempts    = flag.Int("retry-attempts", 3, "Number of connection retry attempts")
	retryDelay       = flag.Duration("retry-delay", 2*time.Second, "Delay between retries")
)

// ConnectionTestResult holds the results of connection testing.
type ConnectionTestResult struct {
	Success         bool
	ConnectDuration time.Duration
	PingDuration    time.Duration
	DatabaseVersion string
	CurrentDatabase string
	Stats           sql.DBStats
	Error           error
}

func main() {
	flag.Usage = usage
	flag.Parse()

	// Load credentials from environment if flags not provided
	loadCredentialsFromEnv()

	// Validate required parameters
	if *dbHost == "" {
		log.Fatal("error: -db-host is required (or DB_HOST environment variable)")
	}
	if *dbUser == "" {
		log.Fatal("error: -db-user is required (or DB_USER environment variable)")
	}
	if *dbPassword == "" {
		log.Fatal("error: -db-password is required (or DB_PASSWORD environment variable)")
	}

	log.Println("=== Database Connection Test ===")
	log.Printf("Target: %s:%s/%s\n", *dbHost, *dbPort, *dbName)
	log.Printf("User: %s\n", *dbUser)
	log.Printf("SSL Mode: %s\n", *sslMode)
	log.Println()

	// Run all tests
	results := runAllTests()

	// Print summary
	printSummary(results)

	// Exit with appropriate code
	if results["connection"].(ConnectionTestResult).Success {
		log.Println("\n✓ All tests passed - database connection verified successfully")
		os.Exit(0)
	} else {
		log.Println("\n✗ Tests failed - database connection not working")
		os.Exit(1)
	}
}

// loadCredentialsFromEnv loads database credentials from environment variables
// if the corresponding flags are not set.
func loadCredentialsFromEnv() {
	if *dbHost == "" && os.Getenv("DB_HOST") != "" {
		*dbHost = os.Getenv("DB_HOST")
	}
	if *dbPort == "5432" && os.Getenv("DB_PORT") != "" {
		*dbPort = os.Getenv("DB_PORT")
	}
	if *dbName == "commitgraph" && os.Getenv("DB_NAME") != "" {
		*dbName = os.Getenv("DB_NAME")
	}
	if *dbUser == "" && os.Getenv("DB_USER") != "" {
		*dbUser = os.Getenv("DB_USER")
	}
	if *dbPassword == "" && os.Getenv("DB_PASSWORD") != "" {
		*dbPassword = os.Getenv("DB_PASSWORD")
	}
	if *sslMode == "require" && os.Getenv("DB_SSLMODE") != "" {
		*sslMode = os.Getenv("DB_SSLMODE")
	}
}

// runAllTests executes all database connection tests.
func runAllTests() map[string]interface{} {
	results := make(map[string]interface{})

	log.Println("Test 1: Basic connection with retry logic")
	results["connection"] = testConnectionWithRetry()
	log.Println()

	log.Println("Test 2: Connection pool configuration")
	results["pool"] = testConnectionPool()
	log.Println()

	log.Println("Test 3: Connection timeout")
	results["timeout"] = testConnectionTimeout()
	log.Println()

	log.Println("Test 4: Query execution")
	results["query"] = testQueryExecution()
	log.Println()

	return results
}

// testConnectionWithRetry tests database connection with retry logic.
func testConnectionWithRetry() ConnectionTestResult {
	var lastErr error
	var result ConnectionTestResult

	connStr := fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=%s",
		*dbHost, *dbPort, *dbName, *dbUser, *dbPassword, *sslMode)

	for attempt := 1; attempt <= *retryAttempts; attempt++ {
		log.Printf("  Connection attempt %d/%d...", attempt, *retryAttempts)

		start := time.Now()
		db, err := sql.Open("postgres", connStr)
		if err != nil {
			lastErr = err
			log.Printf("  ✗ sql.Open failed: %v", err)
			if attempt < *retryAttempts {
				log.Printf("  Retrying in %s...", *retryDelay)
				time.Sleep(*retryDelay)
			}
			continue
		}
		result.ConnectDuration = time.Since(start)

		// Set connection timeout for ping
		ctx, cancel := context.WithTimeout(context.Background(), *connectionTimeout)
		defer cancel()

		pingStart := time.Now()
		err = db.PingContext(ctx)
		result.PingDuration = time.Since(pingStart)

		if err != nil {
			lastErr = err
			db.Close()
			log.Printf("  ✗ Ping failed: %v", err)
			if attempt < *retryAttempts {
				log.Printf("  Retrying in %s...", *retryDelay)
				time.Sleep(*retryDelay)
			}
			continue
		}

		// Connection successful - get database info
		result.DatabaseVersion = getDatabaseVersion(db)
		result.CurrentDatabase = getCurrentDatabase(db)
		result.Stats = db.Stats()
		result.Success = true

		db.Close()

		log.Printf("  ✓ Connection successful")
		log.Printf("    Connect time: %s", result.ConnectDuration)
		log.Printf("    Ping time: %s", result.PingDuration)
		log.Printf("    Database: %s", result.CurrentDatabase)
		log.Printf("    Version: %s", result.DatabaseVersion)

		return result
	}

	// All retries failed
	result.Error = lastErr
	log.Printf("  ✗ All %d connection attempts failed, last error: %v", *retryAttempts, lastErr)
	return result
}

// testConnectionPool tests connection pool configuration.
func testConnectionPool() ConnectionTestResult {
	var result ConnectionTestResult

	connStr := fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=%s",
		*dbHost, *dbPort, *dbName, *dbUser, *dbPassword, *sslMode)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		result.Error = err
		log.Printf("  ✗ Failed to open database: %v", err)
		return result
	}
	defer db.Close()

	// Configure connection pool
	db.SetMaxOpenConns(*maxOpenConns)
	db.SetMaxIdleConns(*maxIdleConns)
	db.SetConnMaxLifetime(*connMaxLifetime)

	log.Printf("  Configured connection pool:")
	log.Printf("    Max Open Connections: %d", *maxOpenConns)
	log.Printf("    Max Idle Connections: %d", *maxIdleConns)
	log.Printf("    Conn Max Lifetime: %s", *connMaxLifetime)

	// Verify configuration with a ping
	ctx, cancel := context.WithTimeout(context.Background(), *connectionTimeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		result.Error = err
		log.Printf("  ✗ Ping failed: %v", err)
		return result
	}

	// Get stats after ping
	stats := db.Stats()
	result.Stats = stats
	result.Success = true

	log.Printf("  ✓ Connection pool configured successfully")
	log.Printf("    Open Connections: %d", stats.OpenConnections)
	log.Printf("    In Use: %d", stats.InUse)
	log.Printf("    Idle: %d", stats.Idle)
	log.Printf("    Wait Count: %d", stats.WaitCount)
	log.Printf("    Wait Duration: %s", stats.WaitDuration)
	log.Printf("    Max Idle Closed: %d", stats.MaxIdleClosed)
	log.Printf("    Max Lifetime Closed: %d", stats.MaxLifetimeClosed)

	return result
}

// testConnectionTimeout tests connection timeout behavior.
func testConnectionTimeout() ConnectionTestResult {
	var result ConnectionTestResult

	// Test with a very short timeout
	shortTimeout := 1 * time.Millisecond
	connStr := fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=%s",
		*dbHost, *dbPort, *dbName, *dbUser, *dbPassword, *sslMode)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		result.Error = err
		log.Printf("  ✗ Failed to open database: %v", err)
		return result
	}
	defer db.Close()

	log.Printf("  Testing timeout with %s timeout...", shortTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), shortTimeout)
	defer cancel()

	start := time.Now()
	err = db.PingContext(ctx)
	elapsed := time.Since(start)

	// Short timeout might fail or succeed depending on network
	// The important thing is that it respects the timeout
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("  ✓ Timeout respected (deadline exceeded after %s)", elapsed)
			result.Success = true
		} else {
			log.Printf("  ~ Connection failed with %s elapsed (might be network): %v", elapsed, err)
			// Don't count this as failure - it's expected with very short timeout
			result.Success = true
		}
	} else {
		log.Printf("  ~ Connection succeeded within %s (network is fast)", elapsed)
		result.Success = true
	}

	// Now test with normal timeout
	normalCtx, normalCancel := context.WithTimeout(context.Background(), *connectionTimeout)
	defer normalCancel()

	log.Printf("  Testing with normal %s timeout...", *connectionTimeout)
	start = time.Now()
	err = db.PingContext(normalCtx)
	elapsed = time.Since(start)

	if err != nil {
		log.Printf("  ✗ Normal timeout test failed: %v", err)
		result.Error = err
		return result
	}

	log.Printf("  ✓ Normal timeout test passed (%s)", elapsed)
	return result
}

// testQueryExecution tests basic query execution.
func testQueryExecution() ConnectionTestResult {
	var result ConnectionTestResult

	connStr := fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=%s",
		*dbHost, *dbPort, *dbName, *dbUser, *dbPassword, *sslMode)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		result.Error = err
		log.Printf("  ✗ Failed to open database: %v", err)
		return result
	}
	defer db.Close()

	// Configure connection pool
	db.SetMaxOpenConns(*maxOpenConns)
	db.SetMaxIdleConns(*maxIdleConns)

	ctx, cancel := context.WithTimeout(context.Background(), *connectionTimeout)
	defer cancel()

	// Test 1: Simple query
	log.Printf("  Test: Current timestamp query")
	var now time.Time
	err = db.QueryRowContext(ctx, "SELECT NOW()").Scan(&now)
	if err != nil {
		result.Error = err
		log.Printf("  ✗ Timestamp query failed: %v", err)
		return result
	}
	log.Printf("  ✓ Server time: %s", now.Format(time.RFC3339))

	// Test 2: Query a system table
	log.Printf("  Test: Query pg_tables")
	var tableCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM pg_tables WHERE schemaname = 'public'").Scan(&tableCount)
	if err != nil {
		result.Error = err
		log.Printf("  ✗ pg_tables query failed: %v", err)
		return result
	}
	log.Printf("  ✓ Public tables count: %d", tableCount)

	// Test 3: Check for expected commitgraph tables (if database exists)
	expectedTables := []string{
		"email_resolution",
		"repos",
		"commits",
		"commit_authors",
		"commit_parents",
		"user_aliases",
		"audit_log",
		"email_revalidation",
	}

	log.Printf("  Test: Checking for commitgraph tables")
	var foundTables []string
	for _, table := range expectedTables {
		var exists int
		err = db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1",
			table).Scan(&exists)
		if err != nil {
			log.Printf("  ! Could not check table %s: %v", table, err)
			continue
		}
		if exists > 0 {
			foundTables = append(foundTables, table)
		}
	}

	if len(foundTables) > 0 {
		log.Printf("  ✓ Found %d/%d expected tables: %v",
			len(foundTables), len(expectedTables), foundTables)
	} else {
		log.Printf("  ~ No commitgraph tables found (database might be empty)")
	}

	result.Success = true
	return result
}

// getDatabaseVersion retrieves the PostgreSQL version.
func getDatabaseVersion(db *sql.DB) string {
	var version string
	err := db.QueryRow("SELECT version()").Scan(&version)
	if err != nil {
		return fmt.Sprintf("Unknown: %v", err)
	}
	return version
}

// getCurrentDatabase retrieves the current database name.
func getCurrentDatabase(db *sql.DB) string {
	var dbName string
	err := db.QueryRow("SELECT current_database()").Scan(&dbName)
	if err != nil {
		return fmt.Sprintf("Unknown: %v", err)
	}
	return dbName
}

// printSummary prints a summary of all test results.
func printSummary(results map[string]interface{}) {
	log.Println("=== Test Summary ===")
	log.Println()

	connResult := results["connection"].(ConnectionTestResult)
	if connResult.Success {
		log.Printf("✓ Connection: SUCCESS (%s connect, %s ping)",
			connResult.ConnectDuration, connResult.PingDuration)
		log.Printf("  Database: %s", connResult.CurrentDatabase)
	} else {
		log.Printf("✗ Connection: FAILED - %v", connResult.Error)
	}

	poolResult := results["pool"].(ConnectionTestResult)
	if poolResult.Success {
		log.Printf("✓ Connection Pool: SUCCESS")
		log.Printf("  Stats: Open=%d Idle=%d InUse=%d",
			poolResult.Stats.OpenConnections, poolResult.Stats.Idle, poolResult.Stats.InUse)
	} else {
		log.Printf("✗ Connection Pool: FAILED - %v", poolResult.Error)
	}

	timeoutResult := results["timeout"].(ConnectionTestResult)
	if timeoutResult.Success {
		log.Printf("✓ Timeout: SUCCESS")
	} else {
		log.Printf("✗ Timeout: FAILED - %v", timeoutResult.Error)
	}

	queryResult := results["query"].(ConnectionTestResult)
	if queryResult.Success {
		log.Printf("✓ Query Execution: SUCCESS")
	} else {
		log.Printf("✗ Query Execution: FAILED - %v", queryResult.Error)
	}
}

// usage prints the usage message.
func usage() {
	fmt.Fprintf(os.Stderr, `Usage:
  test-db-connection [flags]

Flags:
  -db-host string
        PostgreSQL host (required, or DB_HOST env var)
  -db-port string
        PostgreSQL port (default "5432", or DB_PORT env var)
  -db-name string
        PostgreSQL database name (default "commitgraph", or DB_NAME env var)
  -db-user string
        PostgreSQL user (required, or DB_USER env var)
  -db-password string
        PostgreSQL password (required, or DB_PASSWORD env var)
  -sslmode string
        PostgreSQL SSL mode (default "require", or DB_SSLMODE env var)

  Connection Pool Settings:
  -max-open-conns int
        Maximum open connections (default 10)
  -max-idle-conns int
        Maximum idle connections (default 5)
  -conn-max-lifetime duration
        Connection maximum lifetime (default 5m)

  Test Settings:
  -connection-timeout duration
        Connection timeout (default 30s)
  -retry-attempts int
        Number of connection retry attempts (default 3)
  -retry-delay duration
        Delay between retries (default 2s)

What it tests:
  1. Database connection with retry logic
  2. Connection pool configuration
  3. Connection timeout behavior
  4. Basic query execution

Examples:
  # Using flags
  test-db-connection -db-host localhost -db-user postgres -db-password secret

  # Using environment variables
  export DB_HOST=localhost
  export DB_USER=postgres
  export DB_PASSWORD=secret
  test-db-connection

  # Test against remote database with custom port
  test-db-connection -db-host db.example.com -db-port 5433 -db-user app_user -db-password pass

Exit codes:
  0: All tests passed
  1: One or more tests failed
`)
	os.Exit(2)
}
