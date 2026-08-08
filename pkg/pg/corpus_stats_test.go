package pg

// Fixture tests for corpus_stats table (cg-faiar)
// Tests table constraints, idempotent upserts, and the three-stat convention.

import (
	"context"
	"database/sql"
	"regexp"
	"testing"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// setupCorpusStatsTestDB starts a Postgres container, applies the corpus_stats
// table schema, and returns an open *sql.DB plus a cleanup function.
func setupCorpusStatsTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Skipf("skipping integration test: failed to start postgres container: %v", err)
	}

	cleanupContainer := func() {
		if err := testcontainers.TerminateContainer(pgContainer); err != nil {
			t.Logf("failed to terminate postgres container: %v", err)
		}
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		cleanupContainer()
		t.Fatalf("failed to get connection string: %v", err)
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		cleanupContainer()
		t.Fatalf("failed to connect to database: %v", err)
	}

	// Apply the corpus_stats table schema
	schema := `
		CREATE TABLE IF NOT EXISTS corpus_stats (
		  stat  TEXT PRIMARY KEY,
		  value BIGINT NOT NULL
		);
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		cleanupContainer()
		t.Fatalf("failed to create corpus_stats table: %v", err)
	}

	cleanup := func() {
		db.Close()
		cleanupContainer()
	}

	return db, cleanup
}

// TestCorpusStats_PrimaryKeyEnforcement tests that duplicate stat inserts
// fail without ON CONFLICT, confirming stat is a PRIMARY KEY.
func TestCorpusStats_PrimaryKeyEnforcement(t *testing.T) {
	db, cleanup := setupCorpusStatsTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Insert first row successfully
	_, err := db.ExecContext(ctx, `
		INSERT INTO corpus_stats (stat, value) VALUES ($1, $2)
	`, "commits", int64(100))
	if err != nil {
		t.Fatalf("failed to insert first row: %v", err)
	}

	// Attempt to insert duplicate stat without ON CONFLICT - should fail
	_, err = db.ExecContext(ctx, `
		INSERT INTO corpus_stats (stat, value) VALUES ($1, $2)
	`, "commits", int64(200))

	if err == nil {
		t.Error("expected primary key violation when inserting duplicate stat, but insert succeeded")
	} else {
		// Verify the error message mentions primary key or unique constraint
		if !regexp.MustCompile(`(?i)primary.*key.*constraint|duplicate.*key|unique.*constraint`).MatchString(err.Error()) {
			t.Errorf("expected primary key constraint violation error, got: %v", err)
		}
	}
}

// TestCorpusStats_NotNullConstraints tests that both stat and value columns
// enforce NOT NULL constraints.
func TestCorpusStats_NotNullConstraints(t *testing.T) {
	db, cleanup := setupCorpusStatsTestDB(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("stat is NOT NULL", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `
			INSERT INTO corpus_stats (value) VALUES ($1)
		`, int64(100))

		if err == nil {
			t.Error("expected NOT NULL violation when inserting without stat, but insert succeeded")
		} else {
			if !regexp.MustCompile(`(?i)null.*constraint|cannot.*null`).MatchString(err.Error()) {
				t.Errorf("expected NOT NULL constraint violation error, got: %v", err)
			}
		}
	})

	t.Run("value is NOT NULL", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `
			INSERT INTO corpus_stats (stat) VALUES ($1)
		`, "commits")

		if err == nil {
			t.Error("expected NOT NULL violation when inserting without value, but insert succeeded")
		} else {
			if !regexp.MustCompile(`(?i)null.*constraint|cannot.*null`).MatchString(err.Error()) {
				t.Errorf("expected NOT NULL constraint violation error, got: %v", err)
			}
		}
	})
}

// TestCorpusStats_UpsertIdempotent tests that ON CONFLICT (stat) DO UPDATE SET
// value = excluded.value upsert correctly replaces a prior value rather than
// erroring or duplicating.
func TestCorpusStats_UpsertIdempotent(t *testing.T) {
	db, cleanup := setupCorpusStatsTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Insert initial value
	_, err := db.ExecContext(ctx, `
		INSERT INTO corpus_stats (stat, value) VALUES ($1, $2)
	`, "commits", int64(100))
	if err != nil {
		t.Fatalf("failed to insert initial value: %v", err)
	}

	// Verify initial value
	var initialValue int64
	err = db.QueryRowContext(ctx, `
		SELECT value FROM corpus_stats WHERE stat = $1
	`, "commits").Scan(&initialValue)
	if err != nil {
		t.Fatalf("failed to query initial value: %v", err)
	}
	if initialValue != 100 {
		t.Errorf("initial value mismatch: got %d, want 100", initialValue)
	}

	// Upsert with new value using ON CONFLICT
	_, err = db.ExecContext(ctx, `
		INSERT INTO corpus_stats (stat, value) VALUES ($1, $2)
		ON CONFLICT (stat) DO UPDATE SET value = excluded.value
	`, "commits", int64(200))
	if err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	// Verify value was updated (not duplicated)
	var updatedValue int64
	err = db.QueryRowContext(ctx, `
		SELECT value FROM corpus_stats WHERE stat = $1
	`, "commits").Scan(&updatedValue)
	if err != nil {
		t.Fatalf("failed to query updated value: %v", err)
	}
	if updatedValue != 200 {
		t.Errorf("updated value mismatch: got %d, want 200", updatedValue)
	}

	// Verify only one row exists (no duplicate)
	var rowCount int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM corpus_stats WHERE stat = $1
	`, "commits").Scan(&rowCount)
	if err != nil {
		t.Fatalf("failed to count rows: %v", err)
	}
	if rowCount != 1 {
		t.Errorf("row count mismatch: got %d, want 1 (duplicate created?)", rowCount)
	}
}

// TestCorpusStats_ThreeStatConvention tests that only three rows exist for
// the three known stat values (commits, developers, repositories) in a
// representative fixture load. This documents the three-stat convention as
// a deliberate, unenforced convention rather than a gap nobody noticed.
func TestCorpusStats_ThreeStatConvention(t *testing.T) {
	db, cleanup := setupCorpusStatsTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Load the three canonical stats
	_, err := db.ExecContext(ctx, `
		INSERT INTO corpus_stats (stat, value) VALUES
			('commits', 76614890),
			('developers', 1094043),
			('repositories', 98747)
	`)
	if err != nil {
		t.Fatalf("failed to load three canonical stats: %v", err)
	}

	// Verify exactly three rows exist
	var totalCount int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM corpus_stats
	`).Scan(&totalCount)
	if err != nil {
		t.Fatalf("failed to count total rows: %v", err)
	}
	if totalCount != 3 {
		t.Errorf("total row count mismatch: got %d, want 3", totalCount)
	}

	// Verify the three expected stat values exist
	expectedStats := []string{"commits", "developers", "repositories"}
	for _, expectedStat := range expectedStats {
		var stat string
		var value int64
		err = db.QueryRowContext(ctx, `
			SELECT stat, value FROM corpus_stats WHERE stat = $1
		`, expectedStat).Scan(&stat, &value)
		if err != nil {
			t.Errorf("failed to query stat %q: %v", expectedStat, err)
		}
		if stat != expectedStat {
			t.Errorf("stat name mismatch: got %q, want %q", stat, expectedStat)
		}
		if value == 0 {
			t.Errorf("stat %q has zero value", expectedStat)
		}
	}

	t.Log("corpus_stats holds exactly the three canonical stats: commits, developers, repositories")

	// Document the convention: note that the schema does NOT enforce a CHECK
	// constraint, so a fourth stat value could be inserted. This is deliberate
	// per docs/plan/plan.md - the three-stat convention is maintained by the
	// rollup-writing pipeline, not by the schema.
	//
	// Test that inserting a fourth stat is allowed at the schema level
	// (this documents the unenforced convention).
	_, err = db.ExecContext(ctx, `
		INSERT INTO corpus_stats (stat, value) VALUES ($1, $2)
	`, "some_other_metric", int64(42))
	if err != nil {
		t.Logf("fourth stat insertion failed (schema-level enforcement): %v", err)
	} else {
		t.Log("fourth stat insertion succeeded (convention is unenforced at schema level, maintained by pipeline)")
	}
}

// TestCorpusStats_UpsertAllThreeStats tests upsert behavior across all three
// canonical stats to ensure the ON CONFLICT clause works correctly for each.
func TestCorpusStats_UpsertAllThreeStats(t *testing.T) {
	db, cleanup := setupCorpusStatsTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Initial insert of all three stats
	_, err := db.ExecContext(ctx, `
		INSERT INTO corpus_stats (stat, value) VALUES
			('commits', 100),
			('developers', 50),
			('repositories', 10)
	`)
	if err != nil {
		t.Fatalf("failed to insert initial stats: %v", err)
	}

	// Upsert each stat with new values
	upserts := []struct {
		stat  string
		value int64
	}{
		{"commits", 200},
		{"developers", 100},
		{"repositories", 25},
	}

	for _, upsert := range upserts {
		_, err := db.ExecContext(ctx, `
			INSERT INTO corpus_stats (stat, value) VALUES ($1, $2)
			ON CONFLICT (stat) DO UPDATE SET value = excluded.value
		`, upsert.stat, upsert.value)
		if err != nil {
			t.Errorf("upsert failed for stat %q: %v", upsert.stat, err)
		}
	}

	// Verify all three stats were updated correctly
	for _, upsert := range upserts {
		var value int64
		err := db.QueryRowContext(ctx, `
			SELECT value FROM corpus_stats WHERE stat = $1
		`, upsert.stat).Scan(&value)
		if err != nil {
			t.Errorf("failed to query stat %q: %v", upsert.stat, err)
		}
		if value != upsert.value {
			t.Errorf("stat %q value mismatch after upsert: got %d, want %d",
				upsert.stat, value, upsert.value)
		}
	}

	// Verify still exactly three rows exist
	var rowCount int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM corpus_stats
	`).Scan(&rowCount)
	if err != nil {
		t.Fatalf("failed to count rows: %v", err)
	}
	if rowCount != 3 {
		t.Errorf("row count after upserts: got %d, want 3 (duplicates created?)", rowCount)
	}
}
