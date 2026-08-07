// Package pg provides PostgreSQL implementations for commitgraph.
package pg

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"
)

// BatchUsersUpsertQuery is a SQL query that batch upserts multiple user logins
// and returns the complete login -> user_id mapping.
//
// This query:
// - Accepts an array of logins via unnest($1::text[])
// - Inserts new users with those logins (profile_url and avatar_url are NULL initially)
// - On conflict (login already exists), performs a no-op update (preserves the
//   existing row's data, only login is reassigned to itself)
// - Returns all logins with their user_ids, both newly created and pre-existing
//
// The ON CONFLICT clause deliberately uses "DO UPDATE SET login = excluded.login"
// rather than "DO NOTHING": Postgres does not produce RETURNING rows for
// conflicts resolved by DO NOTHING, only for rows actually written by INSERT
// or UPDATE. Without the no-op update, a conflicting (pre-existing) login
// would be silently dropped from the result set, and the caller would need a
// second SELECT round trip to fill in the gaps. The no-op update keeps the
// whole batch upsert to exactly one round trip.
//
// The query is idempotent: re-running with the same login set returns consistent results.
//
// Usage:
//   rows, err := db.Query(ctx, BatchUsersUpsertQuery, pq.Array([]string{"alice", "bob"}))
//   for rows.Next() {
//     var login string
//     var userID int64
//     rows.Scan(&login, &userID)
//     // login -> userID mapping
//   }
//
// Example SQL test:
//   SELECT * FROM users;
//   -- Initial state: empty
//
//   SELECT * FROM unnest(ARRAY['alice', 'bob', 'charlie']) AS login;
//   -- Input: three logins
//
//   -- Execute the query:
//   INSERT INTO users (login)
//   SELECT unnest(ARRAY['alice', 'bob', 'charlie']::text[])
//   ON CONFLICT (login) DO UPDATE SET login = excluded.login
//   RETURNING login, user_id;
//   -- Returns: alice -> 1, bob -> 2, charlie -> 3
//
//   -- Re-run with same + new logins:
//   INSERT INTO users (login)
//   SELECT unnest(ARRAY['alice', 'bob', 'diana']::text[])
//   ON CONFLICT (login) DO UPDATE SET login = excluded.login
//   RETURNING login, user_id;
//   -- Returns: alice -> 1, bob -> 2, diana -> 4
//   -- (alice and bob reuse existing IDs, diana gets new ID)
const BatchUsersUpsertQuery = `
INSERT INTO users (login)
SELECT unnest($1::text[])
ON CONFLICT (login) DO UPDATE SET login = excluded.login
RETURNING login, user_id
`

// UsersSelectByLoginsQuery is a fallback query that retrieves user_ids for existing logins.
// This is useful when you need to get the mapping without attempting insertion.
//
// Use this when:
// - You know all logins already exist and want to avoid the overhead of INSERT attempts
// - You need to distinguish between "not found" and "newly inserted" cases
//
// Returns only logins that already exist in the users table.
const UsersSelectByLoginsQuery = `
SELECT login, user_id
FROM users
WHERE login = ANY($1::text[])
`

// BatchUpsertUsers executes BatchUsersUpsertQuery in exactly one SQL round
// trip and returns the complete login -> user_id map for every login in the
// input slice, whether it was just inserted or already existed.
//
// db is typically a *sql.Tx so the upsert participates in the caller's
// transaction alongside the rest of a scan-job write; a *sql.DB also
// satisfies the same QueryContext method and works for standalone callers.
//
// An empty logins slice returns an empty (non-nil) map without touching the
// database.
func BatchUpsertUsers(ctx context.Context, db *sql.Tx, logins []string) (map[string]int64, error) {
	result := make(map[string]int64, len(logins))
	if len(logins) == 0 {
		return result, nil
	}

	rows, err := db.QueryContext(ctx, BatchUsersUpsertQuery, pq.Array(logins))
	if err != nil {
		return nil, fmt.Errorf("batch upsert users failed for %d logins: %w", len(logins), err)
	}
	defer rows.Close()

	for rows.Next() {
		var login string
		var userID int64
		if err := rows.Scan(&login, &userID); err != nil {
			return nil, fmt.Errorf("scan batch upsert users row: %w", err)
		}
		result[login] = userID
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate batch upsert users rows: %w", err)
	}

	return result, nil
}
