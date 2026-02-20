package testutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	postgrescontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// PostgresContainer holds references to a test Postgres container
type PostgresContainer struct {
	ConnString string
	container  testcontainers.Container
}

// Cleanup terminates the container and cleans up resources
func (pc *PostgresContainer) Cleanup(t *testing.T) {
	if err := testcontainers.TerminateContainer(pc.container); err != nil {
		t.Logf("failed to terminate container: %v", err)
	}
}

// SetupTestPostgres creates a Postgres testcontainer and applies migrations.
// It returns a PostgresContainer with connection string and a cleanup function.
// The caller is responsible for creating their own Store/client using the connection string.
//
// This function uses the actual migration files from the migrations/ directory,
// ensuring that tests use the exact same schema as production, including:
//   - All tables and columns
//   - Indexes for query performance
//   - Triggers (e.g., updated_at timestamp)
//   - Foreign key constraints
//   - Enums and custom types
//
// Example usage:
//
//	pgContainer := testutil.SetupTestPostgres(t)
//	defer pgContainer.Cleanup(t)
//	store, err := postgres.New(pgContainer.ConnString)
//	require.NoError(t, err)
//	defer store.Close()
func SetupTestPostgres(t *testing.T) *PostgresContainer {
	t.Helper()
	ctx := context.Background()

	// Start Postgres container
	container, err := postgrescontainer.Run(ctx,
		"postgres:16-alpine",
		postgrescontainer.WithDatabase("testdb"),
		postgrescontainer.WithUsername("testuser"),
		postgrescontainer.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	require.NoError(t, err, "failed to start postgres container")

	// Get connection string
	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "failed to get connection string")

	// Run migrations using golang-migrate
	// Find migrations directory - it should be at project root
	// Try common paths relative to where tests run
	migrationsPath := findMigrationsPath(t)
	m, err := migrate.New(migrationsPath, connStr)
	require.NoError(t, err, "failed to create migrate instance for path: %s", migrationsPath)

	err = m.Up()
	require.NoError(t, err, "failed to run migrations")

	// Close migrate instance
	sourceErr, dbErr := m.Close()
	require.NoError(t, sourceErr, "failed to close migration source")
	require.NoError(t, dbErr, "failed to close migration database")

	return &PostgresContainer{
		ConnString: connStr,
		container:  container,
	}
}

// findMigrationsPath locates the migrations directory relative to the current working directory.
// It tries several common paths to support running tests from different locations.
func findMigrationsPath(t *testing.T) string {
	t.Helper()

	// Get current working directory
	wd, err := os.Getwd()
	require.NoError(t, err, "failed to get working directory")

	// Try common paths relative to current directory
	candidates := []string{
		"migrations",             // Running from project root
		"../migrations",          // Running from internal/
		"../../migrations",       // Running from internal/testutil/
		"../../../migrations",    // Running from internal/storage/postgres/
		"../../../../migrations", // Running from deeper nesting
	}

	for _, candidate := range candidates {
		absPath := filepath.Join(wd, candidate)
		if _, err := os.Stat(absPath); err == nil {
			// Found it! Return as file:// URL
			return fmt.Sprintf("file://%s", absPath)
		}
	}

	// If we get here, we couldn't find migrations
	require.FailNow(t, "migrations directory not found",
		"tried looking for migrations from working directory: %s\nCandidates: %v",
		wd, candidates)
	return ""
}
