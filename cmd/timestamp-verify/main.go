// timestamp_verify compares timestamps between source and target databases.
package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// Open source SQLite database
	sourceDB, err := sql.Open("sqlite3", "/home/coding/commitgraph/test_sample_cache.db")
	if err != nil {
		log.Fatalf("error: failed to open source SQLite database: %v\n", err)
	}
	defer sourceDB.Close()

	// Open target PostgreSQL database
	targetDB, err := sql.Open("postgres", "host=localhost port=15432 dbname=commitgraph user=postgres password=postgres sslmode=disable")
	if err != nil {
		log.Fatalf("error: failed to open target PostgreSQL database: %v\n", err)
	}
	defer targetDB.Close()

	// Query source data
	sourceRows, err := sourceDB.Query("SELECT author_email, github_login, resolved_at FROM author_login_cache WHERE github_login != '' ORDER BY resolved_at DESC LIMIT 5")
	if err != nil {
		log.Fatalf("error: failed to query source data: %v\n", err)
	}
	defer sourceRows.Close()

	type SourceRecord struct {
		Email      string
		Login      string
		ResolvedAt string
	}

	var sourceRecords []SourceRecord
	for sourceRows.Next() {
		var email, login, resolvedAt string
		if err := sourceRows.Scan(&email, &login, &resolvedAt); err != nil {
			log.Fatalf("error: failed to scan source row: %v\n", err)
		}
		sourceRecords = append(sourceRecords, SourceRecord{Email: email, Login: login, ResolvedAt: resolvedAt})
	}

	fmt.Println("Comparing timestamps between source and target databases:")
	fmt.Println()

	allMatch := true
	for _, src := range sourceRecords {
		// Query corresponding record from target
		var targetEmail, targetLogin, targetSource, targetResolvedAt string
		err := targetDB.QueryRow("SELECT email, login, source, resolved_at FROM email_resolution WHERE email = $1", src.Email).Scan(
			&targetEmail, &targetLogin, &targetSource, &targetResolvedAt)
		if err != nil {
			if err == sql.ErrNoRows {
				fmt.Printf("✗ %s → %s: NOT FOUND in target\n", src.Email, src.Login)
				allMatch = false
				continue
			}
			log.Fatalf("error: failed to query target for %s: %v\n", src.Email, err)
		}

		// Parse timestamps for comparison
		srcTime, err := time.Parse(time.RFC3339Nano, src.ResolvedAt)
		if err != nil {
			log.Fatalf("error: failed to parse source timestamp %s: %v\n", src.ResolvedAt, err)
		}

		targetTime, err := time.Parse(time.RFC3339Nano, targetResolvedAt)
		if err != nil {
			log.Fatalf("error: failed to parse target timestamp %s: %v\n", targetResolvedAt, err)
		}

		// Compare timestamps
		timestampMatch := srcTime.Equal(targetTime)
		loginMatch := src.Login == targetLogin
		sourceMatch := targetSource == "seed"

		matchSymbol := "✓"
		if !timestampMatch || !loginMatch || !sourceMatch {
			matchSymbol = "✗"
			allMatch = false
		}

		fmt.Printf("%s %s → %s\n", matchSymbol, src.Email, src.Login)
		fmt.Printf("   Source timestamp: %s\n", src.ResolvedAt)
		fmt.Printf("   Target timestamp: %s\n", targetResolvedAt)
		fmt.Printf("   Target source: '%s'\n", targetSource)

		if !timestampMatch {
			fmt.Printf("   ✗ TIMESTAMP MISMATCH (diff: %v)\n", targetTime.Sub(srcTime))
		}
		if !loginMatch {
			fmt.Printf("   ✗ LOGIN MISMATCH (source: '%s', target: '%s')\n", src.Login, targetLogin)
		}
		if !sourceMatch {
			fmt.Printf("   ✗ SOURCE MISMATCH (expected 'seed', got '%s')\n", targetSource)
		}
		fmt.Println()
	}

	if allMatch {
		fmt.Println("✓ All sampled records match: source='seed', logins correct, timestamps preserved exactly")
	} else {
		fmt.Println("✗ Some records do not match - see details above")
	}
}
