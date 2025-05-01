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

	log.Println("AddYNABConfigTable migration completed")
	return nil
}
