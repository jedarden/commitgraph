# Task cg-5pqwf: Remove Deprecated SQLite Resolution Write Code

## Investigation Summary

This task investigated whether there were any deprecated SQLite resolution write methods to remove after the ingest endpoint migration.

## Findings

**No deprecated SQLite resolution write code exists to remove.** The migration was completed in a prior bead (commit `02bc3b4`).

### Current Resolution Write Path

All resolution writes now use the ingest endpoint pattern:

1. **`pkg/identity/ingest.go`** - Defines the `Ingester` interface and `ResolutionRow` struct
2. **`pkg/pg/identity.go`** - PostgreSQL implementation using bulk upsert with ON CONFLICT
3. **`pkg/client/queueapi/client.go`** - HTTP client for queue-api ingest endpoint

### Login Revalidation Worker

The `containers/login-revalidation-worker/main.go` calls the queue-api client:

```go
func updateEmailResolution(ctx context.Context, cfg *Config, email, newLogin string) error {
    // Use the queue-api client to post the resolution
    return cfg.QueueAPIClient.PostResolution(ctx, email, newLogin)
}
```

### SQLite Imports (Not Deprecated)

The only remaining `github.com/mattn/go-sqlite3` imports are in:
- `cmd/seed-email-resolution/main.go` - READS from claude-leaderboard database
- `cmd/seed-author-login-cache/main.go` - READS from claude-leaderboard database

These imports are **ACTIVE and CORRECT** - they read data from the source SQLite database and then use the **NEW** ingest endpoint to write to PostgreSQL.

### No Direct SQLite Writes to email_resolution

All writes to `email_resolution` table go through:
- PostgreSQL bulk upsert (`pkg/pg/identity.go`)
- Queue-api HTTP client (`pkg/client/queueapi/client.go`)

There are no direct SQLite writes to the `email_resolution` table.

## Acceptance Criteria Status

✅ **Old SQLite resolution write code is removed or marked deprecated**
   - No old SQLite write code exists to remove

✅ **Unused SQLite imports are cleaned up**
   - All SQLite imports are in active use for reading source data

✅ **No remaining code paths call the old write method**
   - All code uses the ingest endpoint pattern

✅ **Code compiles without errors**
   - Verified: no compilation errors related to resolution writes

## Conclusion

The task is complete because there is no deprecated code to remove. The migration to the ingest endpoint was completed in a prior bead, and all code now follows the new pattern.
