package pg

// Integration tests for IdentityIngester.IngestEmailResolution against a real
// PostgreSQL instance via testcontainers-go. These tests verify the complete
// conflict detection logic, including the ON CONFLICT WHERE clause that implements
// the priority rules: manual source always wins, newer timestamps win for
// non-manual sources.

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/jedarden/commitgraph/pkg/identity"
)

// setupIdentityTestDB starts a real Postgres container, applies the
// email_resolution schema, and returns an open *sql.DB plus a cleanup function.
func setupIdentityTestDB(t *testing.T) (*sql.DB, func()) {
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

	schema := `
		CREATE TABLE IF NOT EXISTS email_resolution (
			email TEXT NOT NULL PRIMARY KEY,
			login TEXT NOT NULL,
			source TEXT NOT NULL,
			resolved_at TIMESTAMPTZ NOT NULL
		);
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		cleanupContainer()
		t.Fatalf("failed to create schema: %v", err)
	}

	cleanup := func() {
		db.Close()
		cleanupContainer()
	}

	return db, cleanup
}

// TestIngestEmailResolution_NewRows_Integration tests inserting new email
// resolutions where no conflicts exist.
func TestIngestEmailResolution_NewRows_Integration(t *testing.T) {
	db, cleanup := setupIdentityTestDB(t)
	defer cleanup()

	ctx := context.Background()
	executor := NewSQLExecutor(db)
	ingester := NewIdentityIngester(executor)

	now := time.Now().UTC()
	rows := []identity.ResolutionRow{
		{Email: "alice@example.com", Login: "alice", Source: identity.SourceLive, ResolvedAt: now},
		{Email: "bob@example.com", Login: "bob", Source: identity.SourceSeed, ResolvedAt: now},
		{Email: "charlie@example.com", Login: "charlie", Source: identity.SourceManual, ResolvedAt: now},
	}

	result, err := ingester.IngestEmailResolution(ctx, rows)
	if err != nil {
		t.Fatalf("IngestEmailResolution failed: %v", err)
	}

	// All new rows should be ingested
	if result.Ingested != 3 {
		t.Errorf("expected 3 ingested, got %d", result.Ingested)
	}
	if result.Skipped != 0 {
		t.Errorf("expected 0 skipped, got %d", result.Skipped)
	}

	// Verify rows were inserted
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM email_resolution").Scan(&count)
	if err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 rows in database, got %d", count)
	}
}

// TestIngestEmailResolution_ManualWins_Integration tests that manual source
// always wins over non-manual sources, regardless of timestamp.
func TestIngestEmailResolution_ManualWins_Integration(t *testing.T) {
	db, cleanup := setupIdentityTestDB(t)
	defer cleanup()

	ctx := context.Background()
	executor := NewSQLExecutor(db)
	ingester := NewIdentityIngester(executor)

	// Insert initial row with live source
	olderTime := time.Now().UTC().Add(-24 * time.Hour)
	_, err := db.ExecContext(ctx, `
		INSERT INTO email_resolution (email, login, source, resolved_at)
		VALUES ($1, $2, $3, $4)
	`, "alice@example.com", "alice-old", "live", olderTime)
	if err != nil {
		t.Fatalf("insert initial row failed: %v", err)
	}

	// Try to update with manual source (older timestamp, but should win)
	rows := []identity.ResolutionRow{
		{Email: "alice@example.com", Login: "alice-manual", Source: identity.SourceManual, ResolvedAt: olderTime.Add(-1 * time.Hour)},
	}

	result, err := ingester.IngestEmailResolution(ctx, rows)
	if err != nil {
		t.Fatalf("IngestEmailResolution failed: %v", err)
	}

	// Manual source should update despite being older
	if result.Ingested != 1 {
		t.Errorf("expected 1 ingested (manual wins), got %d", result.Ingested)
	}
	if result.Skipped != 0 {
		t.Errorf("expected 0 skipped, got %d", result.Skipped)
	}

	// Verify the manual login is in the database
	var login string
	err = db.QueryRowContext(ctx, "SELECT login FROM email_resolution WHERE email = $1", "alice@example.com").Scan(&login)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if login != "alice-manual" {
		t.Errorf("expected login 'alice-manual', got %q", login)
	}
}

// TestIngestEmailResolution_NewerNonManualWins_Integration tests that newer
// non-manual sources win over older non-manual sources.
func TestIngestEmailResolution_NewerNonManualWins_Integration(t *testing.T) {
	db, cleanup := setupIdentityTestDB(t)
	defer cleanup()

	ctx := context.Background()
	executor := NewSQLExecutor(db)
	ingester := NewIdentityIngester(executor)

	// Insert initial row with seed source
	olderTime := time.Now().UTC().Add(-24 * time.Hour)
	_, err := db.ExecContext(ctx, `
		INSERT INTO email_resolution (email, login, source, resolved_at)
		VALUES ($1, $2, $3, $4)
	`, "bob@example.com", "bob-old", "seed", olderTime)
	if err != nil {
		t.Fatalf("insert initial row failed: %v", err)
	}

	// Try to update with newer live source
	now := time.Now().UTC()
	rows := []identity.ResolutionRow{
		{Email: "bob@example.com", Login: "bob-new", Source: identity.SourceLive, ResolvedAt: now},
	}

	result, err := ingester.IngestEmailResolution(ctx, rows)
	if err != nil {
		t.Fatalf("IngestEmailResolution failed: %v", err)
	}

	// Newer live source should update older seed source
	if result.Ingested != 1 {
		t.Errorf("expected 1 ingested (newer wins), got %d", result.Ingested)
	}
	if result.Skipped != 0 {
		t.Errorf("expected 0 skipped, got %d", result.Skipped)
	}

	// Verify the new login is in the database
	var login string
	err = db.QueryRowContext(ctx, "SELECT login FROM email_resolution WHERE email = $1", "bob@example.com").Scan(&login)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if login != "bob-new" {
		t.Errorf("expected login 'bob-new', got %q", login)
	}
}

// TestIngestEmailResolution_OlderNonManualLoses_Integration tests that older
// non-manual sources lose to newer non-manual sources.
func TestIngestEmailResolution_OlderNonManualLoses_Integration(t *testing.T) {
	db, cleanup := setupIdentityTestDB(t)
	defer cleanup()

	ctx := context.Background()
	executor := NewSQLExecutor(db)
	ingester := NewIdentityIngester(executor)

	// Insert initial row with newer seed source
	newerTime := time.Now().UTC()
	_, err := db.ExecContext(ctx, `
		INSERT INTO email_resolution (email, login, source, resolved_at)
		VALUES ($1, $2, $3, $4)
	`, "charlie@example.com", "charlie-new", "seed", newerTime)
	if err != nil {
		t.Fatalf("insert initial row failed: %v", err)
	}

	// Try to update with older live source
	olderTime := newerTime.Add(-24 * time.Hour)
	rows := []identity.ResolutionRow{
		{Email: "charlie@example.com", Login: "charlie-old", Source: identity.SourceLive, ResolvedAt: olderTime},
	}

	result, err := ingester.IngestEmailResolution(ctx, rows)
	if err != nil {
		t.Fatalf("IngestEmailResolution failed: %v", err)
	}

	// Older live source should lose to newer seed source
	if result.Ingested != 0 {
		t.Errorf("expected 0 ingested (older loses), got %d", result.Ingested)
	}
	if result.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", result.Skipped)
	}

	// Verify the original login is still in the database
	var login string
	err = db.QueryRowContext(ctx, "SELECT login FROM email_resolution WHERE email = $1", "charlie@example.com").Scan(&login)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if login != "charlie-new" {
		t.Errorf("expected login 'charlie-new', got %q", login)
	}

	// Verify skip details
	if result.SkipDetails[identity.SkipReasonConflictOlder] != 1 {
		t.Errorf("expected 1 skip with reason ConflictOlder, got %d", result.SkipDetails[identity.SkipReasonConflictOlder])
	}
}

// TestIngestEmailResolution_ManualBeatsManual_Integration tests that when
// both are manual, the newer one wins (since sources are equal, timestamp wins).
func TestIngestEmailResolution_ManualBeatsManual_Integration(t *testing.T) {
	db, cleanup := setupIdentityTestDB(t)
	defer cleanup()

	ctx := context.Background()
	executor := NewSQLExecutor(db)
	ingester := NewIdentityIngester(executor)

	// Insert initial row with older manual source
	olderTime := time.Now().UTC().Add(-24 * time.Hour)
	_, err := db.ExecContext(ctx, `
		INSERT INTO email_resolution (email, login, source, resolved_at)
		VALUES ($1, $2, $3, $4)
	`, "diane@example.com", "diane-old", "manual", olderTime)
	if err != nil {
		t.Fatalf("insert initial row failed: %v", err)
	}

	// Try to update with newer manual source
	now := time.Now().UTC()
	rows := []identity.ResolutionRow{
		{Email: "diane@example.com", Login: "diane-new", Source: identity.SourceManual, ResolvedAt: now},
	}

	result, err := ingester.IngestEmailResolution(ctx, rows)
	if err != nil {
		t.Fatalf("IngestEmailResolution failed: %v", err)
	}

	// Newer manual should update older manual
	if result.Ingested != 1 {
		t.Errorf("expected 1 ingested (newer manual wins), got %d", result.Ingested)
	}
	if result.Skipped != 0 {
		t.Errorf("expected 0 skipped, got %d", result.Skipped)
	}

	// Verify the new login is in the database
	var login string
	err = db.QueryRowContext(ctx, "SELECT login FROM email_resolution WHERE email = $1", "diane@example.com").Scan(&login)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if login != "diane-new" {
		t.Errorf("expected login 'diane-new', got %q", login)
	}
}

// TestIngestEmailResolution_MixedConflicts_Integration tests a batch with
// mixed conflict scenarios: new rows, manual wins, newer wins, older loses.
func TestIngestEmailResolution_MixedConflicts_Integration(t *testing.T) {
	db, cleanup := setupIdentityTestDB(t)
	defer cleanup()

	ctx := context.Background()
	executor := NewSQLExecutor(db)
	ingester := NewIdentityIngester(executor)

	now := time.Now().UTC()
	older := now.Add(-24 * time.Hour)

	// Set up existing data
	existingRows := []struct {
		email, login, source string
		resolvedAt           time.Time
	}{
		{"alice@example.com", "alice-old", "live", older},      // Will be updated by manual
		{"bob@example.com", "bob-old", "seed", older},          // Will be updated by newer live
		{"charlie@example.com", "charlie-new", "seed", now},   // Will NOT be updated by older live
		{"diane@example.com", "diane-old", "manual", older},   // Will be updated by newer manual
	}

	for _, row := range existingRows {
		_, err := db.ExecContext(ctx, `
			INSERT INTO email_resolution (email, login, source, resolved_at)
			VALUES ($1, $2, $3, $4)
		`, row.email, row.login, row.source, row.resolvedAt)
		if err != nil {
			t.Fatalf("insert existing row failed: %v", err)
		}
	}

	// Ingest batch with mixed conflicts
	rows := []identity.ResolutionRow{
		{Email: "alice@example.com", Login: "alice-manual", Source: identity.SourceManual, ResolvedAt: older.Add(-1 * time.Hour)}, // manual wins despite being older
		{Email: "bob@example.com", Login: "bob-new", Source: identity.SourceLive, ResolvedAt: now},                       // newer live wins older seed
		{Email: "charlie@example.com", Login: "charlie-old", Source: identity.SourceLive, ResolvedAt: older},            // older live loses to newer seed
		{Email: "diane@example.com", Login: "diane-new", Source: identity.SourceManual, ResolvedAt: now},                  // newer manual wins older manual
		{Email: "eve@example.com", Login: "eve", Source: identity.SourceSeed, ResolvedAt: now},                           // new row
	}

	result, err := ingester.IngestEmailResolution(ctx, rows)
	if err != nil {
		t.Fatalf("IngestEmailResolution failed: %v", err)
	}

	// Expected: alice (manual wins), bob (newer wins), diane (newer manual wins), eve (new) = 4 ingested
	// Expected: charlie (older loses) = 1 skipped
	if result.Ingested != 4 {
		t.Errorf("expected 4 ingested, got %d", result.Ingested)
	}
	if result.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", result.Skipped)
	}

	// Verify skip details
	if result.SkipDetails[identity.SkipReasonConflictOlder] != 1 {
		t.Errorf("expected 1 skip with reason ConflictOlder, got %d", result.SkipDetails[identity.SkipReasonConflictOlder])
	}
}

// TestNewSQLExecutor tests the SQLExecutor constructor.
func TestNewSQLExecutor(t *testing.T) {
	db, cleanup := setupIdentityTestDB(t)
	defer cleanup()

	executor := NewSQLExecutor(db)
	if executor == nil {
		t.Fatal("NewSQLExecutor returned nil")
	}
	if executor.db == nil {
		t.Error("executor.db is nil")
	}
	if executor.db != db {
		t.Error("executor.db does not match input db")
	}
}

// TestNewIdentityIngester tests the IdentityIngester constructor.
func TestNewIdentityIngester(t *testing.T) {
	db, cleanup := setupIdentityTestDB(t)
	defer cleanup()

	executor := NewSQLExecutor(db)
	ingester := NewIdentityIngester(executor)
	if ingester == nil {
		t.Fatal("NewIdentityIngester returned nil")
	}
	if ingester.db == nil {
		t.Error("ingester.db is nil")
	}
	if ingester.db != executor {
		t.Error("ingester.db does not match input executor")
	}
}
