# cg-tdytq: Extract NULL Login Samples

## Task
Extract 5-50 pairs from author_login_cache that have NULL logins.

## What was done
1. Located author_login_cache data at `/home/coding/commitgraph/cmd/seed-author-login-cache/testdata/author_login_cache_sample.csv`
2. Extracted 19 samples with NULL github_login values
3. Preserved original timestamp format (ISO 8601 with microseconds)
4. Saved to `notes/cg-tdytq-null-login-samples.csv`

## Sample data
The extracted file contains 19 entries where github_login is NULL:
- unknown.user1@example.com,NULL,2026-08-06T10:00:00.000000+00:00
- unknown.user2@example.com,NULL,2026-08-06T10:05:00.000000+00:00
- (and 17 more)

All NULL values are represented as the literal string "NULL" as in the original data.
