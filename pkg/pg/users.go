// Package pg provides PostgreSQL implementations for commitgraph.
package pg

// BatchUsersUpsertQuery is a SQL query that batch upserts multiple user logins
// and returns the complete login -> user_id mapping.
//
// This query:
// - Accepts an array of logins via unnest($1::text[])
// - Inserts new users with those logins (profile_url and avatar_url are NULL initially)
// - On conflict (login already exists), does nothing (preserves existing row)
// - Returns all logins with their user_ids, both newly created and pre-existing
//
// The query is idempotent: re-running with the same login set returns consistent results.
//
// Usage:
//   rows, err := db.Query(ctx, BatchUsersUpsertQuery, []string{"alice", "bob"})
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
//   ON CONFLICT (login) DO NOTHING
//   RETURNING login, user_id;
//   -- Returns: alice -> 1, bob -> 2, charlie -> 3
//
//   -- Re-run with same + new logins:
//   INSERT INTO users (login)
//   SELECT unnest(ARRAY['alice', 'bob', 'diana']::text[])
//   ON CONFLICT (login) DO NOTHING
//   RETURNING login, user_id;
//   -- Returns: alice -> 1, bob -> 2, diana -> 4
//   -- (alice and bob reuse existing IDs, diana gets new ID)
const BatchUsersUpsertQuery = `
INSERT INTO users (login)
SELECT unnest($1::text[])
ON CONFLICT (login) DO NOTHING
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
