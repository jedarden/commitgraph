# Database Files Found in queue-api Pod (cg-219bo)

## Search Summary
Searched the queue-api pod filesystem for SQLite database files on 2026-08-06.

## Pod Information
- **Cluster**: ord-devimprint
- **Namespace**: commitgraph
- **Pod**: queue-api-c5894c469-p9rhr
- **Status**: Running (2/2 containers ready)

## Database Files Found

### Primary Database
- **`/data/queue.db`** - 810,295,296 bytes (~773 MB)
  - Main SQLite database file
  - Owned by queueapi user

### WAL (Write-Ahead Log) Files
- **`/data/queue.db-wal`** - 20,534,112 bytes (~19.6 MB)
  - SQLite WAL file for concurrent access
- **`/data/queue.db-shm`** - 65,536 bytes (64 KB)
  - SQLite shared memory file for WAL

### Backup
- **`/data/queue.db.backup-20260718-132909`** - 24,576 bytes (24 KB)
  - Backup created on 2026-07-18 13:29:09

### Litestream Replication
- **`/data/.queue.db-litestream/`** - Directory containing Litestream replication files
  - Contains `.ltx` files in subdirectory structure
  - These are Litestream WAL segment files, not standalone databases

## Directories Searched
- ✅ `/data` - Primary database location found here
- ✅ `/app` - No database files found
- ✅ `/var/lib` - No database files found
- ✅ `/home` - Directory exists but empty (no /home/app)
- ✅ Full filesystem search completed (excluding /sys, /proc, /dev)

## Summary
The queue-api application stores its data in a single SQLite database at `/data/queue.db` with WAL enabled for concurrent access. The database is approximately 773 MB in size. A Litestream replication directory is also present for backup/replication purposes.
