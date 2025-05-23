package migrations

import (
	"database/sql"
	"fmt"
	"log"
)

// GetPendingMigrations returns a list of migration names that need to be applied
func GetPendingMigrations(db *sql.DB) ([]string, error) {
	// Ensure migration table exists
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS migrations (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get all migrations that should be applied
	allMigrations := []string{
		"add_transaction_notes_column",
		"fix_ynab_schema",
		"ensure_admin_user",
		"fix_ynab_api_urls",
		"fix_ynab_categories_schema",
		"add_ynab_categories_table",
		"fix_ynab_categories_column",
	}

	// Check which migrations have already been applied
	appliedMigrations := make(map[string]bool)
	rows, err := db.Query("SELECT name FROM migrations")
	if err != nil {
		return nil, fmt.Errorf("failed to query migrations: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("error scanning migration name: %w", err)
		}
		appliedMigrations[name] = true
	}

	// Determine which migrations are pending
	var pending []string
	for _, migration := range allMigrations {
		if !appliedMigrations[migration] {
			pending = append(pending, migration)
		}
	}

	return pending, nil
}

// RecordMigration records that a migration has been applied
func RecordMigration(db *sql.DB, name string) error {
	_, err := db.Exec(`
		INSERT INTO migrations (name) 
		VALUES ($1)
		ON CONFLICT (name) DO NOTHING
	`, name)

	if err != nil {
		return fmt.Errorf("failed to record migration '%s': %w", name, err)
	}

	log.Printf("Migration '%s' successfully recorded", name)
	return nil
}
