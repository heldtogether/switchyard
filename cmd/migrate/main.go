package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

const (
	exitSuccess = 0
	exitError   = 1
)

func main() {
	// Parse command-line flags
	var (
		dir      string
		action   string
		database string
	)

	flag.StringVar(&dir, "dir", "./migrations", "Path to migrations directory")
	flag.StringVar(&action, "action", "", "Migration action: up, down")
	flag.StringVar(&database, "database", "", "Database URL (defaults to DATABASE_URL env var)")
	flag.Parse()

	// Validate required flags
	if action == "" {
		log.Fatal("Error: -action flag is required (up or down)")
	}

	if action != "up" && action != "down" {
		log.Fatalf("Error: invalid action '%s'. Must be 'up' or 'down'", action)
	}

	// Get database URL from flag or environment
	dbURL := database
	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
	}

	if dbURL == "" {
		log.Fatal("Error: database URL not provided. Set DATABASE_URL env var or use -database flag")
	}

	// Run migrations
	if err := runMigrations(dbURL, dir, action); err != nil {
		log.Printf("Migration failed: %v", err)
		os.Exit(exitError)
	}

	log.Printf("Migration '%s' completed successfully", action)
	os.Exit(exitSuccess)
}

// runMigrations executes the migration operation
func runMigrations(dbURL, migrationsDir, action string) error {
	// Construct file source URL
	sourceURL := fmt.Sprintf("file://%s", migrationsDir)

	log.Printf("Initializing migrations...")
	log.Printf("  Source: %s", sourceURL)
	log.Printf("  Action: %s", action)

	// Create migration instance
	m, err := migrate.New(sourceURL, dbURL)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}
	defer m.Close()

	// Get current version before migration
	version, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	if errors.Is(err, migrate.ErrNilVersion) {
		log.Printf("Current state: No migrations applied yet")
	} else {
		log.Printf("Current version: %d (dirty: %v)", version, dirty)
	}

	// Execute the migration
	log.Printf("Running migration '%s'...", action)

	switch action {
	case "up":
		err = m.Up()
		if err != nil && errors.Is(err, migrate.ErrNoChange) {
			log.Println("No new migrations to apply")
			return nil
		}
		if err != nil {
			return fmt.Errorf("migration up failed: %w", err)
		}
		log.Println("Applied migrations successfully")

	case "down":
		err = m.Steps(-1)
		if err != nil && errors.Is(err, migrate.ErrNoChange) {
			log.Println("No migrations to roll back")
			return nil
		}
		if err != nil {
			return fmt.Errorf("migration down failed: %w", err)
		}
		log.Println("Rolled back one migration successfully")
	}

	// Get version after migration
	version, dirty, err = m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("failed to get final version: %w", err)
	}

	if errors.Is(err, migrate.ErrNilVersion) {
		log.Printf("Final state: No migrations applied")
	} else {
		log.Printf("Final version: %d (dirty: %v)", version, dirty)
	}

	return nil
}
