# Task cg-t3oc2: Save extracted sample to test file

## Completion Summary

Successfully ran the `extract-sample-cache-data` script and verified the sample data is properly saved to the test file location.

## Verification Results

### File Location
- **Temporary file:** `/tmp/author_login_cache_test.csv`
- **Test data file:** `cmd/seed-author-login-cache/testdata/author_login_cache_test_sample.csv`
- **Status:** Files are identical (no differences)

### Acceptance Criteria Verification

✅ **Sample data saved to temporary test file**
- Created `/tmp/author_login_cache_test.csv` via extraction script

✅ **File format matches expected input format for tests**
- Header: `author_email,github_login,resolved_at`
- CSV format matches expected structure

✅ **File contains exactly 10-100 pairs**
- Total lines: 51 (1 header + 50 data rows)
- Data pairs: 50 (within required 10-100 range)

✅ **Both valid and NULL logins present in file**
- NULL logins: 5 entries (10%)
- Non-NULL logins: 45 entries (90%)
- Mix provides good test coverage for both scenarios

✅ **Timestamps preserved correctly**
- ISO 8601 format with microsecond precision
- Examples:
  - `2026-08-06T10:00:00.000000+00:00` (synthetic NULL entries)
  - `2026-06-29T20:14:17.787578Z` (real database entries)

✅ **File is readable and parseable**
- Successfully read and verified with standard tools
- CSV structure is valid and parseable

## Execution Details

**Command run:**
```bash
./extract-sample-cache-data -output /tmp/author_login_cache_test.csv -count 50
```

**Script output:**
- Database statistics: 349,425 total rows, 0 NULL logins in source
- Extracted 50 pairs (45 non-NULL + 5 synthetic NULL for test coverage)
- File successfully written to `/tmp/author_login_cache_test.csv`

## Notes

The extraction script automatically handles the case where the source database has no NULL logins by creating synthetic NULL entries (lines 2, 10, 18, 26, 34 in the output). This ensures test data covers both NULL and non-NULL scenarios even when the source database only has valid logins.

The test sample file was already created in a previous run (cg-5l4fq) and verified to be identical to the current output, confirming the extraction script produces consistent, repeatable results.

## Files Modified
- No files modified (test sample file already exists from cg-5l4fq)
- Created `notes/cg-t3oc2.md` to document completion
