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

	// Create migrations table if it doesn't exist
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS migrations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
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
		{"add_transaction_date", AddTransactionDateColumn},
		{"add_ynab_tables", AddYNABTables},
		{"string_user_ids", StringUserIDs},
		{"add_categories_unique_constraint", AddCategoriesUniqueConstraint},
		{"add_optional_field", AddOptionalField},
		// Add future migrations here
	}

	// Apply migrations that haven't been run yet
	for _, migration := range migrations {
		// Check if migration has already been applied
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM migrations WHERE name = ?", migration.name).Scan(&count)
		if err != nil {
			return fmt.Errorf("error checking migration status: %w", err)
		}

		if count == 0 {
			log.Printf("Running migration: %s", migration.name)

			// Run the migration
			if err := migration.fn(db); err != nil {
				return fmt.Errorf("migration '%s' failed: %w", migration.name, err)
			}

			// Record that the migration was applied
			_, err = db.Exec("INSERT INTO migrations (name) VALUES (?)", migration.name)
			if err != nil {
				return fmt.Errorf("failed to record migration '%s': %w", migration.name, err)
			}

			log.Printf("Migration '%s' completed successfully", migration.name)
		} else {
			log.Printf("Migration '%s' already applied, skipping", migration.name)
		}
	}

	log.Println("All migrations completed")

	// After all migrations are done, seed test data for dev/PR environments
	// This is not part of the core migrations, so we don't track it in the migrations table
	// Check environment variables to see if we should reset/seed data
	if os.Getenv("RESET_DB") == "true" ||
		os.Getenv("APP_ENV") == "development" ||
		os.Getenv("PR_DEPLOYMENT") == "true" {

		// Only run if not in production
		if os.Getenv("APP_ENV") != "production" && os.Getenv("NODE_ENV") != "production" {
			log.Println("Non-production environment detected. Running test data seeding...")
			if err := SeedTestData(db); err != nil {
				return fmt.Errorf("failed to seed test data: %w", err)
			}
		}
	}

	return nil
}
