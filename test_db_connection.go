package main

import (
	"context"
	"database/sql"
	"log"
	"os"

	_ "github.com/lib/pq" // PostgreSQL driver
)

func main() {
	// Get database URL from environment or use default
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// Default to localhost PostgreSQL for testing
		dbURL = "postgres://postgres:postgres@localhost:5432/commitgraph?sslmode=disable"
	}

	log.Printf("Testing database connection...")
	log.Printf("Connection URL (password masked): %s", maskPassword(dbURL))

	// Attempt to connect to database
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Test connection with Ping
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	log.Println("✓ Database connection successful")

	// Test if we can query the schema
	var tableName string
	err = db.QueryRowContext(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = 'email_resolution'
	`).Scan(&tableName)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Println("✗ email_resolution table does not exist")
			log.Println("Schema needs to be created")
		} else {
			log.Printf("✗ Error querying schema: %v", err)
		}
	} else {
		log.Printf("✓ Found required table: %s", tableName)

		// Test table structure
		var columnCount int
		err = db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM information_schema.columns
			WHERE table_name = 'email_resolution'
		`).Scan(&columnCount)

		if err != nil {
			log.Printf("✗ Error checking column count: %v", err)
		} else {
			log.Printf("✓ Table has %d columns", columnCount)
		}

		// Test if we can query the table
		var rowCount int64
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM email_resolution").Scan(&rowCount)
		if err != nil {
			log.Printf("✗ Error querying table: %v", err)
		} else {
			log.Printf("✓ Can query table - current row count: %d", rowCount)
		}
	}

	// Get database version
	var version string
	err = db.QueryRowContext(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		log.Printf("✗ Error getting database version: %v", err)
	} else {
		log.Printf("✓ Database version: %s", truncateString(version, 80))
	}

	log.Println("\n=== Database Connection Test Complete ===")
	log.Println("Connection parameters:")
	log.Printf("  - Host: localhost")
	log.Printf("  - Port: 5432")
	log.Printf("  - Database: commitgraph")
	log.Printf("  - User: postgres")
	log.Printf("  - SSL Mode: disable")
}

// maskPassword replaces the password in a connection string for logging
func maskPassword(connStr string) string {
	// Simple masking - find password between :// and @
	for i := 0; i < len(connStr); i++ {
		if i+3 < len(connStr) && connStr[i:i+3] == "://" {
			for j := i + 3; j < len(connStr); j++ {
				if connStr[j] == '@' {
					return connStr[:i+3] + "****:****" + connStr[j:]
				}
			}
		}
	}
	return connStr
}

// truncateString shortens a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}