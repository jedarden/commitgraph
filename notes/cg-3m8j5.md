# BatchUpsertUsers Implementation Verification

## Task (cg-3m8j5)
Implement Go function to execute batched users upsert and return login->user_id map.

## Implementation Location
`pkg/pg/users.go:100-137` - Function `BatchUpsertUsers`

## Acceptance Criteria Verification

All acceptance criteria have been met:

### 1. Function accepts []string of logins ✅
- **Line 100**: `func BatchUpsertUsers(ctx context.Context, db Executor, logins []string) (map[string]int64, error)`
- Uses `Executor` interface (more flexible than `*sql.Tx`, accepts both `*sql.DB` and `*sql.Tx`)

### 2. Issues exactly one SQL query ✅
- **Line 107**: `rows, err := db.QueryContext(ctx, BatchUsersUpsertQuery, logins)`
- Single `QueryContext` call - no per-login loops, exactly one SQL round trip
- Uses CTE query defined at `pkg/pg/users.go:56-64`

### 3. Returns complete login->user_id map for all inputs ✅
- **Lines 114, 117-124**: Creates `map[string]int64` with capacity, scans all rows into map
- Returns `result` containing all input logins with their user_ids

### 4. Returns empty map (not error) for empty login slice ✅
- **Lines 102-104**: `if len(logins) == 0 { return make(map[string]int64), nil }`
- Gracefully handles empty input without error

### 5. On database error, returns error with useful context ✅
- **Line 108-109**: `"batch upsert users query failed: %w"`
- **Line 121-122**: `"scan batch upsert row failed: %w"`
- **Line 127-129**: `"batch upsert rows iteration error: %w"`
- **Line 132-134**: `"close batch upsert rows failed: %w"`
- All errors wrapped with descriptive context using `fmt.Errorf` with `%w`

## SQL Query
The query (`BatchUsersUpsertQuery`) at lines 56-64:
- Uses CTE (`WITH inserted AS`) to INSERT new logins with `ON CONFLICT DO NOTHING`
- Returns all logins (both newly created and pre-existing) with their user_ids
- Single `unnest($1::text[])` parameter accepts the entire login array
- Uses `ANY($1::text[])` to filter and return all input logins

## Implementation Notes
- Function uses `Executor` interface for flexibility (accepts `*sql.DB`, `*sql.Tx`, or mock)
- Properly handles `rows.Close()` with explicit error checking
- Pre-allocates map capacity for efficiency
- Error handling follows Go best practices with `%w` wrapping

## Status
✅ Implementation complete and verified - meets all acceptance criteria
