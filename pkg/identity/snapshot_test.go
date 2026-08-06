package identity

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// setupTestDB creates an in-memory SQLite database with the email_resolution
// table schema and returns the database connection for testing.
func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Create email_resolution table
	_, err = db.Exec(`
		CREATE TABLE email_resolution (
			email TEXT PRIMARY KEY,
			login TEXT NOT NULL,
			source TEXT NOT NULL,
			resolved_at TIMESTAMP NOT NULL
		)
	`)
	if err != nil {
		db.Close()
		t.Fatalf("failed to create email_resolution table: %v", err)
	}

	return db
}

// insertTestRow inserts a single row into the test database.
func insertTestRow(t *testing.T, db *sql.DB, email, login, source string, resolvedAt time.Time) {
	_, err := db.Exec(`
		INSERT INTO email_resolution (email, login, source, resolved_at)
		VALUES (?, ?, ?, ?)
	`, email, login, source, resolvedAt)
	if err != nil {
		t.Fatalf("failed to insert test row: %v", err)
	}
}

// TestCaptureSnapshot verifies basic snapshot capture functionality.
func TestCaptureSnapshot(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert test data
	testTime := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	insertTestRow(t, db, "user1@example.com", "user1", "seed", testTime)
	insertTestRow(t, db, "user2@example.com", "user2", "live", testTime.Add(time.Hour))

	// Capture snapshot
	snapshot, err := CaptureSnapshot(db)
	if err != nil {
		t.Fatalf("CaptureSnapshot failed: %v", err)
	}

	// Verify snapshot
	if snapshot.RowCount != 2 {
		t.Errorf("Expected row count 2, got %d", snapshot.RowCount)
	}

	if snapshot.Hash == "" {
		t.Error("Expected non-empty hash")
	}

	if len(snapshot.Rows) > 0 {
		t.Error("Expected Rows to be empty when WithFullRowData() is not used")
	}

	t.Logf("Captured snapshot: %d rows, hash=%s", snapshot.RowCount, snapshot.Hash)
}

// TestCaptureSnapshotWithFullRowData verifies capturing full row data.
func TestCaptureSnapshotWithFullRowData(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert test data
	testTime := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	insertTestRow(t, db, "user1@example.com", "user1", "seed", testTime)
	insertTestRow(t, db, "user2@example.com", "user2", "live", testTime.Add(time.Hour))

	// Capture snapshot with full row data
	snapshot, err := CaptureSnapshot(db, WithFullRowData())
	if err != nil {
		t.Fatalf("CaptureSnapshot with full row data failed: %v", err)
	}

	// Verify snapshot
	if snapshot.RowCount != 2 {
		t.Errorf("Expected row count 2, got %d", snapshot.RowCount)
	}

	if len(snapshot.Rows) != 2 {
		t.Errorf("Expected 2 rows in snapshot, got %d", len(snapshot.Rows))
	}

	// Verify row data
	// Rows should be sorted by email
	if snapshot.Rows[0].Email != "user1@example.com" {
		t.Errorf("Expected first row email 'user1@example.com', got '%s'", snapshot.Rows[0].Email)
	}
	if snapshot.Rows[1].Email != "user2@example.com" {
		t.Errorf("Expected second row email 'user2@example.com', got '%s'", snapshot.Rows[1].Email)
	}

	if snapshot.Rows[0].Login != "user1" {
		t.Errorf("Expected first row login 'user1', got '%s'", snapshot.Rows[0].Login)
	}
	if snapshot.Rows[0].Source != SourceSeed {
		t.Errorf("Expected first row source 'seed', got '%s'", snapshot.Rows[0].Source)
	}
}

// TestCaptureSnapshotEmptyTable verifies snapshot of empty table.
func TestCaptureSnapshotEmptyTable(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Capture snapshot of empty table
	snapshot, err := CaptureSnapshot(db)
	if err != nil {
		t.Fatalf("CaptureSnapshot of empty table failed: %v", err)
	}

	// Verify snapshot
	if snapshot.RowCount != 0 {
		t.Errorf("Expected row count 0 for empty table, got %d", snapshot.RowCount)
	}

	// Empty table should still produce a hash (of empty data)
	if snapshot.Hash == "" {
		t.Error("Expected non-empty hash even for empty table")
	}

	t.Logf("Empty table snapshot: hash=%s", snapshot.Hash)
}

// TestCaptureSnapshotSorting verifies that rows are sorted by email for hashing.
func TestCaptureSnapshotSorting(t *testing.T) {
	db1 := setupTestDB(t)
	defer db1.Close()

	db2 := setupTestDB(t)
	defer db2.Close()

	testTime := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)

	// Insert rows in different order
	// db1: user1, user2, user3
	insertTestRow(t, db1, "user1@example.com", "user1", "seed", testTime)
	insertTestRow(t, db1, "user2@example.com", "user2", "seed", testTime)
	insertTestRow(t, db1, "user3@example.com", "user3", "seed", testTime)

	// db2: user3, user1, user2 (reverse order)
	insertTestRow(t, db2, "user3@example.com", "user3", "seed", testTime)
	insertTestRow(t, db2, "user1@example.com", "user1", "seed", testTime)
	insertTestRow(t, db2, "user2@example.com", "user2", "seed", testTime)

	// Capture snapshots from both databases
	snapshot1, err := CaptureSnapshot(db1)
	if err != nil {
		t.Fatalf("CaptureSnapshot of db1 failed: %v", err)
	}

	snapshot2, err := CaptureSnapshot(db2)
	if err != nil {
		t.Fatalf("CaptureSnapshot of db2 failed: %v", err)
	}

	// Hashes should be identical despite different insert order
	if snapshot1.Hash != snapshot2.Hash {
		t.Errorf("Hashes differ despite identical data (different insert order):\n  db1: %s\n  db2: %s",
			snapshot1.Hash, snapshot2.Hash)
	}

	t.Logf("Sorting verified: both snapshots have hash=%s", snapshot1.Hash)
}

// TestCaptureSnapshotHashSensitivity verifies that hash changes when any column changes.
func TestCaptureSnapshotHashSensitivity(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	testTime := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)

	// Insert base row
	insertTestRow(t, db, "user@example.com", "user", "seed", testTime)

	// Capture base snapshot
	baseSnapshot, err := CaptureSnapshot(db)
	if err != nil {
		t.Fatalf("CaptureSnapshot failed: %v", err)
	}

	// Test 1: Change login
	_, err = db.Exec(`UPDATE email_resolution SET login = ? WHERE email = ?`, "newuser", "user@example.com")
	if err != nil {
		t.Fatalf("Failed to update login: %v", err)
	}
	loginSnapshot, err := CaptureSnapshot(db)
	if err != nil {
		t.Fatalf("CaptureSnapshot after login change failed: %v", err)
	}
	if loginSnapshot.Hash == baseSnapshot.Hash {
		t.Error("Hash should change when login changes")
	}

	// Test 2: Change source
	_, err = db.Exec(`UPDATE email_resolution SET source = ? WHERE email = ?`, "manual", "user@example.com")
	if err != nil {
		t.Fatalf("Failed to update source: %v", err)
	}
	sourceSnapshot, err := CaptureSnapshot(db)
	if err != nil {
		t.Fatalf("CaptureSnapshot after source change failed: %v", err)
	}
	if sourceSnapshot.Hash == loginSnapshot.Hash {
		t.Error("Hash should change when source changes")
	}

	// Test 3: Change resolved_at
	newTime := testTime.Add(24 * time.Hour)
	_, err = db.Exec(`UPDATE email_resolution SET resolved_at = ? WHERE email = ?`, newTime, "user@example.com")
	if err != nil {
		t.Fatalf("Failed to update resolved_at: %v", err)
	}
	timeSnapshot, err := CaptureSnapshot(db)
	if err != nil {
		t.Fatalf("CaptureSnapshot after resolved_at change failed: %v", err)
	}
	if timeSnapshot.Hash == sourceSnapshot.Hash {
		t.Error("Hash should change when resolved_at changes")
	}

	// Test 4: Change email (primary key)
	_, err = db.Exec(`DELETE FROM email_resolution`)
	if err != nil {
		t.Fatalf("Failed to delete: %v", err)
	}
	insertTestRow(t, db, "newemail@example.com", "user", "manual", newTime)
	emailSnapshot, err := CaptureSnapshot(db)
	if err != nil {
		t.Fatalf("CaptureSnapshot after email change failed: %v", err)
	}
	if emailSnapshot.Hash == timeSnapshot.Hash {
		t.Error("Hash should change when email (primary key) changes")
	}

	t.Log("Hash sensitivity verified for all columns")
}

// TestCompareSnapshotsIdentical verifies comparison of identical snapshots.
func TestCompareSnapshotsIdentical(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	testTime := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	insertTestRow(t, db, "user@example.com", "user", "seed", testTime)

	snapshot1, err := CaptureSnapshot(db)
	if err != nil {
		t.Fatalf("CaptureSnapshot failed: %v", err)
	}

	snapshot2, err := CaptureSnapshot(db)
	if err != nil {
		t.Fatalf("CaptureSnapshot failed: %v", err)
	}

	identical, err := CompareSnapshots(snapshot1, snapshot2)
	if err != nil {
		t.Errorf("CompareSnapshots error for identical snapshots: %v", err)
	}
	if !identical {
		t.Error("Expected identical snapshots to compare as equal")
	}
}

// TestCompareSnapshotsDifferentRowCount verifies comparison detects row count differences.
func TestCompareSnapshotsDifferentRowCount(t *testing.T) {
	db1 := setupTestDB(t)
	defer db1.Close()

	db2 := setupTestDB(t)
	defer db2.Close()

	testTime := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)

	// db1 has 1 row
	insertTestRow(t, db1, "user@example.com", "user", "seed", testTime)

	// db2 has 2 rows
	insertTestRow(t, db2, "user@example.com", "user", "seed", testTime)
	insertTestRow(t, db2, "user2@example.com", "user2", "seed", testTime)

	snapshot1, _ := CaptureSnapshot(db1)
	snapshot2, _ := CaptureSnapshot(db2)

	identical, err := CompareSnapshots(snapshot1, snapshot2)
	if err == nil {
		t.Error("Expected error for different row counts")
	}
	if identical {
		t.Error("Expected snapshots with different row counts to not be identical")
	}
	if err != nil {
		t.Logf("Got expected error: %v", err)
	}
}

// TestCompareSnapshotsDifferentHash verifies comparison detects hash differences.
func TestCompareSnapshotsDifferentHash(t *testing.T) {
	db1 := setupTestDB(t)
	defer db1.Close()

	db2 := setupTestDB(t)
	defer db2.Close()

	testTime := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)

	// Same row count, different data
	insertTestRow(t, db1, "user@example.com", "user1", "seed", testTime)
	insertTestRow(t, db2, "user@example.com", "user2", "seed", testTime)

	snapshot1, _ := CaptureSnapshot(db1)
	snapshot2, _ := CaptureSnapshot(db2)

	identical, err := CompareSnapshots(snapshot1, snapshot2)
	if err == nil {
		t.Error("Expected error for different hashes")
	}
	if identical {
		t.Error("Expected snapshots with different hashes to not be identical")
	}
	if err != nil {
		t.Logf("Got expected error: %v", err)
	}
}

// TestCompareSnapshotsNilSnapshots verifies comparison handles nil snapshots.
func TestCompareSnapshotsNilSnapshots(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	testTime := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	insertTestRow(t, db, "user@example.com", "user", "seed", testTime)

	snapshot, _ := CaptureSnapshot(db)

	// Test both nil
	identical, err := CompareSnapshots(nil, nil)
	if err != nil {
		t.Errorf("Expected both nil snapshots to compare as equal: %v", err)
	}
	if !identical {
		t.Error("Expected both nil snapshots to be identical")
	}

	// Test first nil
	identical, err = CompareSnapshots(nil, snapshot)
	if err == nil {
		t.Error("Expected error when first snapshot is nil")
	}
	if identical {
		t.Error("Expected nil vs non-nil to not be identical")
	}

	// Test second nil
	identical, err = CompareSnapshots(snapshot, nil)
	if err == nil {
		t.Error("Expected error when second snapshot is nil")
	}
	if identical {
		t.Error("Expected non-nil vs nil to not be identical")
	}
}

// TestCaptureSnapshotInvalidSource verifies that invalid source values are rejected.
func TestCaptureSnapshotInvalidSource(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	testTime := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)

	// Insert row with invalid source
	_, err := db.Exec(`
		INSERT INTO email_resolution (email, login, source, resolved_at)
		VALUES (?, ?, ?, ?)
	`, "user@example.com", "user", "invalid_source", testTime)
	if err != nil {
		t.Fatalf("Failed to insert test row: %v", err)
	}

	// CaptureSnapshot should fail with invalid source
	_, err = CaptureSnapshot(db)
	if err == nil {
		t.Error("Expected error for invalid source value")
	}
	t.Logf("Got expected error for invalid source: %v", err)
}

// TestCompareSnapshotsSameReference verifies comparing snapshot to itself.
func TestCompareSnapshotsSameReference(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	testTime := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	insertTestRow(t, db, "user@example.com", "user", "seed", testTime)

	snapshot, err := CaptureSnapshot(db)
	if err != nil {
		t.Fatalf("CaptureSnapshot failed: %v", err)
	}

	// Compare snapshot to itself
	identical, err := CompareSnapshots(snapshot, snapshot)
	if err != nil {
		t.Errorf("CompareSnapshots error when comparing to itself: %v", err)
	}
	if !identical {
		t.Error("Expected snapshot to be identical to itself")
	}
}

// BenchmarkCaptureSnapshot benchmarks the snapshot capture performance.
func BenchmarkCaptureSnapshot(b *testing.B) {
	db := setupTestDB(&testing.T{})
	defer db.Close()

	// Insert test data - simulate a realistic workload
	testTime := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 1000; i++ {
		email := fmt.Sprintf("user%d@example.com", i)
		login := fmt.Sprintf("user%d", i)
		insertTestRow(&testing.T{}, db, email, login, "seed", testTime.Add(time.Duration(i)*time.Second))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CaptureSnapshot(db)
		if err != nil {
			b.Fatalf("CaptureSnapshot failed: %v", err)
		}
	}
}

// BenchmarkCaptureSnapshotWithFullRowData benchmarks snapshot capture with full row data.
func BenchmarkCaptureSnapshotWithFullRowData(b *testing.B) {
	db := setupTestDB(&testing.T{})
	defer db.Close()

	// Insert test data
	testTime := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 1000; i++ {
		email := fmt.Sprintf("user%d@example.com", i)
		login := fmt.Sprintf("user%d", i)
		insertTestRow(&testing.T{}, db, email, login, "seed", testTime.Add(time.Duration(i)*time.Second))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CaptureSnapshot(db, WithFullRowData())
		if err != nil {
			b.Fatalf("CaptureSnapshot with full row data failed: %v", err)
		}
	}
}
