# cg-2prjd: Email Resolution Dump - Durable Storage Upload

## Upload Details

**Date:** 2026-08-06
**Task:** Store email_resolution dump durably

## Source File

- **Path:** `/home/coding/commitgraph/email_resolution_dump.sql`
- **Size:** 139 MB (138.730 MiB)
- **SHA256 Checksum:** `799de2d571a97f24af8b4c426bf3d22babde1a32a05c27209a108d44614a6ddb`
- **Rows:** 897,079
- **Format:** SQLite .dump format
- **Source:** queue-api pod on ord-devimprint cluster

## Durable Storage Location

**B2 Bucket:** `commitgraph-ops`
**Path:** `commitgraph-ops/email_resolution/email_resolution_dump.sql`
**Rclone remote:** `b2commitgraph:commitgraph-ops/email_resolution/`

## Upload Verification

✅ **Upload completed successfully**
- **Transferred:** 138.730 MiB (100%)
- **Transfer time:** 18.1 seconds
- **Transfer rate:** ~7.7 MiB/s

✅ **Integrity verified** via `rclone check`
- **Differences:** 0
- **Matching files:** 1
- **Checksums match:** Yes

## Commands Used

```bash
# Upload to B2
rclone copy /home/coding/commitgraph/email_resolution_dump.sql \
  b2commitgraph:commitgraph-ops/email_resolution/ --progress

# Verify upload integrity
rclone check /home/coding/commitgraph/email_resolution_dump.sql \
  b2commitgraph:commitgraph-ops/email_resolution/ --one-way
```

## Access Information

**Bucket ID:** commitgraph-ops
**Endpoint:** https://s3.us-west-002.backblazeb2.com
**Rclone config name:** b2commitgraph

## Local Backup

✅ Local copy retained at `/home/coding/commitgraph/email_resolution_dump.sql`
- Not deleted after upload (kept as backup)
- File remains in workspace root

## Acceptance Criteria Met

- ✅ Dump uploaded to durable storage (B2 bucket `commitgraph-ops`)
- ✅ Upload verified (checksum matches local file via rclone check)
- ✅ Storage location recorded in bead comments and documentation
- ✅ File size and checksum recorded for downstream verification
- ✅ Local copy retained on ex44 as backup (not deleted)

## Related Work

- **Extraction bead:** cg-519b5 (email_resolution table extraction from queue-api)
- **Source dump created:** 2026-08-06T15:58:00 UTC
- **This upload:** 2026-08-06T16:07:28 UTC
