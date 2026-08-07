// investigate checks the current state of email_resolution table
package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)

func main() {
	// Connect to PostgreSQL
	connStr := "host=localhost port=5432 dbname=commitgraph user=coding password='password' sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("error: failed to connect to PostgreSQL: %v\n", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("error: PostgreSQL ping failed: %v\n", err)
	}

	fmt.Println("=== Current email_resolution Table State ===")

	// Get all records to understand what's in the database
	rows, err := db.Query("SELECT email, login, source, resolved_at FROM email_resolution ORDER BY resolved_at DESC")
	if err != nil {
		log.Fatalf("error: failed to query email_resolution: %v\n", err)
	}
	defer rows.Close()

	type Record struct {
		Email      string
		Login      string
		Source     string
		ResolvedAt time.Time
	}

	var allRecords []Record
	for rows.Next() {
		var r Record
		if err := rows.Scan(&r.Email, &r.Login, &r.Source, &r.ResolvedAt); err != nil {
			log.Fatalf("error: failed to scan row: %v\n", err)
		}
		allRecords = append(allRecords, r)
	}

	fmt.Printf("Total records: %d\n\n", len(allRecords))

	// Group by source
	sourceCounts := make(map[string]int)
	for _, r := range allRecords {
		sourceCounts[r.Source]++
	}

	fmt.Println("Records by source:")
	for source, count := range sourceCounts {
		fmt.Printf("  %s: %d\n", source, count)
	}

	// Show our test dataset records
	fmt.Println("\n=== Test Dataset Records ===")

	testEmails := map[string]bool{
		"bot@quantifieduncertainty.org":    true,
		"lukeleeai@gmail.com":             true,
		"davebuda256@gmail.com":           true,
		"smigolsmigol@protonmail.com":     true,
		"andrewmbourne@gmail.com":         true,
		"tobert@gmail.com":                true,
		"aj@ajbrown.org":                  true,
		"kronosderet@gmail.com":           true,
		"tabhay@hotmail.com":              true,
		"bayze6584@gmail.com":             true,
		"cheonilt@gmail.com":              true,
		"dhnpmp@gmail.com":                true,
		"github@jedarden.com":            true,
		"coder@jedarden.com":             true,
		"julian@aiacrobatics.com":         true,
		"JohnCreighton_@hotmail.com":     true,
		"root@localhost.localdomain":      true,
		"marketing@eclipseadagency.com":  true,
		"pwnetsuite@outlook.com":          true,
		"Heytale.Pazguato@gmail.com":     true,
	}

	testRecordsFound := 0
	for _, r := range allRecords {
		if testEmails[r.Email] {
			testRecordsFound++
			fmt.Printf("✅ %-35s | %-20s | %s | %s\n",
				r.Email, r.Login, r.Source, r.ResolvedAt.Format(time.RFC3339Nano))
		}
	}

	fmt.Printf("\nTest records found: %d/%d\n", testRecordsFound, len(testEmails))

	// Show non-test records
	fmt.Println("\n=== Other Records (Not in Test Dataset) ===")
	otherCount := 0
	for _, r := range allRecords {
		if !testEmails[r.Email] {
			otherCount++
			fmt.Printf("%-3d %-35s | %-20s | %s | %s\n",
				otherCount, r.Email, r.Login, r.Source, r.ResolvedAt.Format(time.RFC3339Nano))
		}
	}

	fmt.Printf("\nNon-test records: %d\n", otherCount)
}
