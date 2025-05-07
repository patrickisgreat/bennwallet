package migrations

import (
	"database/sql"
	"fmt"
	"log"
	"os"
)

// RunMigrations executes all migrations in the correct order
func RunMigrations(db *sql.DB) error {
	log.Println("Running migrations...")

	// If RESET_DB is true, handle it first before any other operations
	if os.Getenv("RESET_DB") == "true" {
		log.Println("RESET_DB is true - resetting database before migrations...")

		// Drop all tables
		if err := DropAllTables(db); err != nil {
			return fmt.Errorf("failed to drop tables: %w", err)
		}

		// Create base tables
		log.Println("Creating base tables...")
		if err := CreateBaseSchema(db); err != nil {
			return fmt.Errorf("failed to create base schema: %w", err)
		}
	}

	// Create migrations table if it doesn't exist
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS migrations (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Define migrations
	migrations := []struct {
		name string
		fn   func(*sql.DB) error
	}{
		// Test data seeding is ONLY for development and testing
		{"seed_test_data", SeedTestData},
	}

	// Check if we're in production
	inProduction := os.Getenv("APP_ENV") == "production" ||
		os.Getenv("NODE_ENV") == "production" ||
		os.Getenv("ENVIRONMENT") == "production" ||
		os.Getenv("ENV") == "production"

	if inProduction {
		log.Println("Running in PRODUCTION mode - test data seeding will be skipped")
	} else {
		log.Println("Running in DEVELOPMENT/TEST mode - test data may be seeded if needed")
	}

	// Run each migration if it hasn't been applied yet
	for _, migration := range migrations {
		// Skip test data seeding in production
		if inProduction && migration.name == "seed_test_data" {
			log.Printf("Skipping test data seeding in production environment")
			continue
		}

		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM migrations WHERE name = $1", migration.name).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check migration status: %w", err)
		}

		if count == 0 {
			log.Printf("Applying migration: %s", migration.name)
			err := migration.fn(db)
			if err != nil {
				return fmt.Errorf("failed to apply migration %s: %w", migration.name, err)
			}

			_, err = db.Exec("INSERT INTO migrations (name) VALUES ($1)", migration.name)
			if err != nil {
				return fmt.Errorf("failed to record migration: %w", err)
			}
		} else {
			log.Printf("Skipping already applied migration: %s", migration.name)
		}
	}

	log.Println("All migrations completed successfully")
	return nil
}
