package migrations

import (
	"database/sql"
	"fmt"
	"log"
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
		// Add all migrations here in order
		{"add_transaction_date", AddTransactionDateColumn},
		{"add_ynab_tables", AddYNABTables},
		{"string_user_ids", StringUserIDs},
		{"add_categories_unique_constraint", AddCategoriesUniqueConstraint},
		{"add_optional_field", AddOptionalField},
		{"add_permissions_table", AddPermissionsTable},
		{"update_users_for_permissions", UpdateUsersForPermissions},
		// For development and PR environments, also seed test data
		{"seed_test_data", SeedTestData},
	}

	// Run each migration if it hasn't been applied yet
	for _, migration := range migrations {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM migrations WHERE name = ?", migration.name).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check migration status: %w", err)
		}

		if count == 0 {
			log.Printf("Applying migration: %s", migration.name)
			err := migration.fn(db)
			if err != nil {
				return fmt.Errorf("failed to apply migration %s: %w", migration.name, err)
			}

			_, err = db.Exec("INSERT INTO migrations (name) VALUES (?)", migration.name)
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
