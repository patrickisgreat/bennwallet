package migrations

import (
	"database/sql"
	"fmt"
	"log"
)

// AddYNABConfigTable creates the YNAB configuration table
func AddYNABConfigTable(db *sql.DB) error {
	log.Println("Running AddYNABConfigTable migration")

	// Create YNAB config table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS ynab_config (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			encrypted_api_token TEXT,
			encrypted_budget_id TEXT,
			encrypted_account_id TEXT,
			last_sync_time TIMESTAMP,
			sync_frequency INTEGER DEFAULT 60,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(user_id)
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create YNAB config table: %w", err)
	}

	// Ensure we have the YNAB category tables as well
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ynab_category_groups (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			user_id TEXT NOT NULL,
			last_updated TIMESTAMP NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);
	`)
	if err != nil {
		log.Printf("Error creating YNAB category groups table: %v", err)
		return fmt.Errorf("failed to create YNAB category groups table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ynab_categories (
			id TEXT PRIMARY KEY,
			group_id TEXT NOT NULL,
			name TEXT NOT NULL,
			user_id TEXT NOT NULL,
			last_updated TIMESTAMP NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (group_id) REFERENCES ynab_category_groups(id) ON DELETE CASCADE
		);
	`)
	if err != nil {
		log.Printf("Error creating YNAB categories table: %v", err)
		return fmt.Errorf("failed to create YNAB categories table: %w", err)
	}

	log.Println("AddYNABConfigTable migration completed")
	return nil
}
