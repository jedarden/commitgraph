# Commit Operation Analysis - Bead cg-5xpxo

## Task
Analyze files identified in grep search to find functions that handle commit INSERT/UPSERT operations.

## Files Analyzed

Based on grep results from bead cg-5p7dz, the following candidate files were analyzed:

1. **pkg/pg/users.go** - Contains `BatchUsersUpsertQuery` constant
2. **pkg/pg/user_aliases.go** - Contains `UpsertAliases` function
3. **pkg/pg/identity.go** - Contains `IngestEmailResolution` function
4. **pkg/pg/repo.go** - Contains `RepoExcluder` with exclusion operations
5. **pkg/pg/invariant_4_integration_test.go** - Test file with commits column references

## Key Finding

**NO COMMIT-SPECIFIC INSERT/UPSERT OPERATIONS EXIST**

The grep search from cg-5p7dz returned **false positives**. The matched files do not contain functions that operate on a "commits" table because:

### No "commits" Table Exists

From the database schema (`migrations/00001_initial_schema.sql`), the commitgraph database contains these tables:

- `repos` - repository identity with exclusion tracking
- `users` - developer identity (no counter columns)
- `email_resolution` - email→login resolution results
- `user_aliases` - login→login alias mapping
- `repo_user_daily_tool` - main rollup (AI-tool-tagged commits) - **contains a `commits` column but it's an INT counter, not a table**
- `corpus_stats` - global scalar totals

### Analysis of False Positives

| File | What grep matched | Reality |
|------|-------------------|---------|
| `pkg/pg/users.go` | Package comment "PostgreSQL implementations for commitgraph" | Contains `BatchUsersUpsertQuery` for **users** table, not commits |
| `pkg/pg/user_aliases.go` | Package comment "PostgreSQL operations for commitgraph user_aliases" | Contains `UpsertAliases` for **user_aliases** table, not commits |
| `pkg/pg/identity.go` | Package comment "PostgreSQL implementations for commitgraph" | Contains `IngestEmailResolution` for **email_resolution** table, not commits |
| `pkg/pg/repo.go` | Package comment "PostgreSQL operations for commitgraph repos" | Contains `RepoExcluder` for **repos** table, not commits |
| `pkg/pg/invariant_4_integration_test.go` | Test data with `commits INT NOT NULL` | References the `commits` **column** in `repo_user_daily_tool` table, not a commits table |

## What Operations DO Exist

The files found contain INSERT/UPSERT operations for these tables:

1. **users table** (`pkg/pg/users.go`)
   - Function: `BatchUsersUpsertQuery` (line 46-50)
   - Operation: `INSERT INTO users (login) ... ON CONFLICT (login) DO NOTHING`
   - Purpose: Batch upsert user login records

2. **user_aliases table** (`pkg/pg/user_aliases.go`)
   - Function: `UpsertAliases` (line 46-88)
   - Operation: `INSERT INTO user_aliases ... ON CONFLICT (source_login) DO UPDATE`
   - Purpose: Bulk upsert alias rows

3. **email_resolution table** (`pkg/pg/identity.go`)
   - Function: `IngestEmailResolution` (line 94-234)
   - Operation: `INSERT INTO email_resolution ... ON CONFLICT (email) DO UPDATE`
   - Purpose: Bulk upsert email resolution rows with conflict resolution logic

4. **repos table** (`pkg/pg/repo.go`)
   - Function: `ApplyExclusion` (line 45-89)
   - Operation: `UPDATE repos SET excluded_at/excluded_reason`
   - Purpose: Apply or clear repo exclusions

## The "commits" Column

The only reference to "commits" in the database schema is the `commits` column in the `repo_user_daily_tool` table (line 52 of schema):

```sql
commits     INT    NOT NULL,
```

This is an **integer counter** column that stores the count of AI-tool-tagged commits per repo/user/tool/day combination, not a table of individual commit records.

## Conclusion

**NO DEDICATED COMMIT TABLE OR OPERATIONS FOUND**

The commitgraph v2 database architecture stores commits only as aggregated counts in the `repo_user_daily_tool.rollup` table. There is:
- No `commits` table
- No functions that INSERT individual commit records
- No functions that UPSERT individual commit records
- Only aggregate commit counts stored in rollup tables

The grep search matched files that contained the word "commit" in package-level comments describing the system as providing "PostgreSQL implementations for commitgraph", not actual commit table operations.

## Next Steps

If the goal was to find where individual commit records are stored:
1. Check if commits are stored in a different system (e.g., queue-api, external storage)
2. Review the plan.md to understand the intended commit storage architecture
3. Determine if commits are meant to be processed and discarded, only storing aggregate counts
