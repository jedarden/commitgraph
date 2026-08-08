package identity

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// LoginRenameFixtures provides reusable test helpers for seeding login rename scenarios.
// These helpers support transactions that can be rolled back after tests.
//
// Usage:
//   tx, _ := db.BeginTx(ctx, nil)
//   defer tx.Rollback()
//   fixtures := &LoginRenameFixtures{Tx: tx}
//   userID := fixtures.SeedUser(ctx, t, "old-name")
//   fixtures.SeedEmailResolution(ctx, t, "user@example.com", "old-name")
//   fixtures.SeedRepoUserDailyTool(ctx, t, repoID, userID, "claude", 3)

// LoginRenameFixtures holds the transaction for seeding test data.
type LoginRenameFixtures struct {
	Tx *sql.Tx
}

// SeedUser creates a users row with the given login and returns the user_id.
// This helper is reusable and can be called with different logins for other tests.
func (f *LoginRenameFixtures) SeedUser(ctx context.Context, t interface{ Helper() }, login string, opts ...UserOption) int64 {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}

	var userID int64
	query := `INSERT INTO users (login`
	var values []interface{}
	var placeholders []string

	values = append(values, login)
	placeholders = append(placeholders, "$1")

	// Apply optional parameters
	for _, opt := range opts {
		if opt.ProfileURL != "" {
			query += `, profile_url`
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(values)+1))
			values = append(values, opt.ProfileURL)
		}
		if opt.AvatarURL != "" {
			query += `, avatar_url`
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(values)+1))
			values = append(values, opt.AvatarURL)
		}
	}

	query += `) VALUES (` + join(placeholders, ", ") + `) RETURNING user_id`

	err := f.Tx.QueryRowContext(ctx, query, values...).Scan(&userID)
	if err != nil {
		if h, ok := t.(interface{ Fatalf(string, ...interface{}) }); ok {
			h.Fatalf("Failed to insert user with login %q: %v", login, err)
		} else {
			panic(fmt.Sprintf("Failed to insert user with login %q: %v", login, err))
		}
	}

	return userID
}

// UserOption provides optional parameters for SeedUser.
type UserOption struct {
	ProfileURL string
	AvatarURL  string
}

// WithProfileURL sets the profile_url for a user.
func WithProfileURL(url string) UserOption {
	return UserOption{ProfileURL: url}
}

// WithAvatarURL sets the avatar_url for a user.
func WithAvatarURL(url string) UserOption {
	return UserOption{AvatarURL: url}
}

// SeedEmailResolution creates an email_resolution row with the email resolved to the given login.
// Source defaults to "seed" and resolved_at defaults to 24 hours ago if not specified.
func (f *LoginRenameFixtures) SeedEmailResolution(ctx context.Context, t interface{ Helper() }, email, login string, opts ...EmailResolutionOption) {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}

	config := EmailResolutionConfig{
		Source:     "seed",
		ResolvedAt: time.Now().Add(-24 * time.Hour),
	}

	for _, opt := range opts {
		opt(&config)
	}

	query := `
		INSERT INTO email_resolution (email, login, source, resolved_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (email) DO UPDATE
		  SET login = excluded.login,
		      source = excluded.source,
		      resolved_at = excluded.resolved_at
	`

	_, err := f.Tx.ExecContext(ctx, query, email, login, config.Source, config.ResolvedAt)
	if err != nil {
		if h, ok := t.(interface{ Fatalf(string, ...interface{}) }); ok {
			h.Fatalf("Failed to insert email_resolution for email %q -> login %q: %v", email, login, err)
		} else {
			panic(fmt.Sprintf("Failed to insert email_resolution for email %q -> login %q: %v", email, login, err))
		}
	}
}

// EmailResolutionConfig holds optional parameters for SeedEmailResolution.
type EmailResolutionConfig struct {
	Source     string
	ResolvedAt time.Time
}

// EmailResolutionOption provides optional configuration for SeedEmailResolution.
type EmailResolutionOption func(*EmailResolutionConfig)

// WithSource sets the source for email resolution.
func WithSource(source string) EmailResolutionOption {
	return func(c *EmailResolutionConfig) {
		c.Source = source
	}
}

// WithResolvedAt sets the resolved_at timestamp.
func WithResolvedAt(ts time.Time) EmailResolutionOption {
	return func(c *EmailResolutionConfig) {
		c.ResolvedAt = ts
	}
}

// SeedRepoUserDailyTool creates repo_user_daily_tool rows for a given user_id.
// The count parameter specifies how many historical days to generate (default 3).
// Each day will have a configurable number of commits (default 5).
func (f *LoginRenameFixtures) SeedRepoUserDailyTool(ctx context.Context, t interface{ Helper() }, repoID, userID int64, tool string, count int, opts ...RepoUserDailyToolOption) {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}

	if count <= 0 {
		count = 3
	}

	config := RepoUserDailyToolConfig{
		Commits:     5,
		StartDate:   time.Now().AddDate(0, 0, -count),
		InsertTime:  time.Now(),
	}

	for _, opt := range opts {
		opt(&config)
	}

	for i := 0; i < count; i++ {
		day := config.StartDate.AddDate(0, 0, i)
		query := `
			INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
			VALUES ($1, $2, $3, $4, $5, $6)
		`

		_, err := f.Tx.ExecContext(ctx, query, repoID, userID, tool, day, config.Commits, config.InsertTime)
		if err != nil {
			if h, ok := t.(interface{ Fatalf(string, ...interface{}) }); ok {
				h.Fatalf("Failed to insert repo_user_daily_tool row %d for user_id %d: %v", i+1, userID, err)
			} else {
				panic(fmt.Sprintf("Failed to insert repo_user_daily_tool row %d for user_id %d: %v", i+1, userID, err))
			}
		}
	}
}

// RepoUserDailyToolConfig holds optional parameters for SeedRepoUserDailyTool.
type RepoUserDailyToolConfig struct {
	Commits    int
	StartDate  time.Time
	InsertTime time.Time
}

// RepoUserDailyToolOption provides optional configuration for SeedRepoUserDailyTool.
type RepoUserDailyToolOption func(*RepoUserDailyToolConfig)

// WithCommits sets the number of commits per day.
func WithCommits(count int) RepoUserDailyToolOption {
	return func(c *RepoUserDailyToolConfig) {
		c.Commits = count
	}
}

// WithStartDate sets the start date for historical data.
func WithStartDate(date time.Time) RepoUserDailyToolOption {
	return func(c *RepoUserDailyToolConfig) {
		c.StartDate = date
	}
}

// WithInsertTime sets the insert_time timestamp.
func WithInsertTime(ts time.Time) RepoUserDailyToolOption {
	return func(c *RepoUserDailyToolConfig) {
		c.InsertTime = ts
	}
}

// SeedRepo creates a repos row and returns the repo_id.
// This is useful for setting up foreign key dependencies for repo_user_daily_tool.
func (f *LoginRenameFixtures) SeedRepo(ctx context.Context, t interface{ Helper() }, provider, repoFullName string) int64 {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}

	var repoID int64
	query := `INSERT INTO repos (provider, repo_full_name) VALUES ($1, $2) RETURNING repo_id`

	err := f.Tx.QueryRowContext(ctx, query, provider, repoFullName).Scan(&repoID)
	if err != nil {
		if h, ok := t.(interface{ Fatalf(string, ...interface{}) }); ok {
			h.Fatalf("Failed to insert repo %s/%s: %v", provider, repoFullName, err)
		} else {
			panic(fmt.Sprintf("Failed to insert repo %s/%s: %v", provider, repoFullName, err))
		}
	}

	return repoID
}

// SeedLoginRenameScenario seeds a complete login rename scenario with all dependencies.
// This is a convenience helper that seeds:
// - A users row with login="old-name" and returns the user_id
// - A repos row and returns the repo_id
// - Historical repo_user_daily_tool activity under that user_id
// - An email_resolution row with email resolved to login="old-name"
//
// Returns: (userID, repoID)
func (f *LoginRenameFixtures) SeedLoginRenameScenario(ctx context.Context, t interface{ Helper() }, email, oldLogin string, opts ...LoginRenameScenarioOption) (userID, repoID int64) {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}

	config := LoginRenameScenarioConfig{
		Provider:          "github",
		RepoFullName:      "test/repo",
		Tool:              "claude",
		HistoricalDays:    3,
		CommitsPerDay:     5,
		EmailSource:       "seed",
		EmailResolvedAt:   time.Now().Add(-24 * time.Hour),
		ProfileURL:        "",
		AvatarURL:         "",
	}

	for _, opt := range opts {
		opt(&config)
	}

	// Seed user
	userID = f.SeedUser(ctx, t, oldLogin,
		WithProfileURL(config.ProfileURL),
		WithAvatarURL(config.AvatarURL),
	)

	// Seed repo
	repoID = f.SeedRepo(ctx, t, config.Provider, config.RepoFullName)

	// Seed historical activity
	f.SeedRepoUserDailyTool(ctx, t, repoID, userID, config.Tool, config.HistoricalDays,
		WithCommits(config.CommitsPerDay),
	)

	// Seed email resolution
	f.SeedEmailResolution(ctx, t, email, oldLogin,
		WithSource(config.EmailSource),
		WithResolvedAt(config.EmailResolvedAt),
	)

	return userID, repoID
}

// LoginRenameScenarioConfig holds optional parameters for SeedLoginRenameScenario.
type LoginRenameScenarioConfig struct {
	Provider        string
	RepoFullName    string
	Tool            string
	HistoricalDays  int
	CommitsPerDay   int
	EmailSource     string
	EmailResolvedAt time.Time
	ProfileURL      string
	AvatarURL       string
}

// LoginRenameScenarioOption provides optional configuration for SeedLoginRenameScenario.
type LoginRenameScenarioOption func(*LoginRenameScenarioConfig)

// WithProvider sets the provider for the repo.
func WithProvider(provider string) LoginRenameScenarioOption {
	return func(c *LoginRenameScenarioConfig) {
		c.Provider = provider
	}
}

// WithRepoFullName sets the repo full name.
func WithRepoFullName(repo string) LoginRenameScenarioOption {
	return func(c *LoginRenameScenarioConfig) {
		c.RepoFullName = repo
	}
}

// WithTool sets the tool name for historical activity.
func WithTool(tool string) LoginRenameScenarioOption {
	return func(c *LoginRenameScenarioConfig) {
		c.Tool = tool
	}
}

// WithHistoricalDays sets the number of historical days to generate.
func WithHistoricalDays(days int) LoginRenameScenarioOption {
	return func(c *LoginRenameScenarioConfig) {
		c.HistoricalDays = days
	}
}

// WithCommitsPerDay sets the number of commits per historical day.
func WithCommitsPerDay(commits int) LoginRenameScenarioOption {
	return func(c *LoginRenameScenarioConfig) {
		c.CommitsPerDay = commits
	}
}

// join is a helper function to join string slices.
func join(slice []string, sep string) string {
	if len(slice) == 0 {
		return ""
	}
	result := slice[0]
	for i := 1; i < len(slice); i++ {
		result += sep + slice[i]
	}
	return result
}
