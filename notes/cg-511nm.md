# Extract Sample with Valid Logins (cg-511nm)

## Task
Extract 5-50 pairs from author_login_cache that have valid email logins.

## Work Completed

### Data Source
- Source file: `cmd/seed-author-login-cache/testdata/author_login_cache_sample.csv`
- Original file contains 80 entries (lines 2-81, after header)
- Entries 2-61: Valid github_logins (non-NULL)
- Entries 62-80: NULL github_logins (unresolved)

### Extraction Results
- Extracted: **20 pairs** with non-NULL logins
- All email addresses validated using RFC 5322-compatible regex
- All github_login fields are non-NULL
- Original timestamp format preserved (ISO 8601 with microseconds)

### Output File
- Location: `notes/cg-511nm-extracted-valid-logins.csv`
- Format: CSV with columns `author_email,github_login,resolved_at`
- Count: 20 entries (within required 5-50 range)

### Validation
✓ All 20 email addresses passed validation
✓ All 20 github_login values are non-NULL
✓ All 20 timestamps preserved in original format

### Sample Entries
1. kinngut7@gmail.com → dogTK
2. noahsolomon2003@gmail.com → noahgsolomon
3. arthur.x.du@gmail.com → ka1kqi
4. broscko@tenstorrent.com → brosckoTT
5. zach@bayouoffice.com → ZachBOM
... (20 total)

All acceptance criteria met:
- [x] Extract 5-50 pairs with non-NULL logins (20 pairs extracted)
- [x] Preserve original timestamp format (ISO 8601 preserved)
- [x] Save extracted pairs to temporary file (saved to notes/)
- [x] Verify all extracted logins are valid email addresses (all 20 validated)
