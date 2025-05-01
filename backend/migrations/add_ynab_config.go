package migrations

import (
	"database/sql"
	"fmt"
	"log"
	"os"
)

// AddYNABConfigTable creates the YNAB configuration table
func AddYNABConfigTable(db *sql.DB) error {
	log.Println("Running AddYNABConfigTable migration")

	// Create YNAB config table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS ynab_config (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			api_token TEXT,
			budget_id TEXT,
			last_sync_time TIMESTAMP,
			sync_frequency INTEGER DEFAULT 60
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create YNAB config table: %w", err)
	}

	// Initialize config with a single row if the table is empty
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM ynab_config").Scan(&count)
	if err != nil {
		return fmt.Errorf("error checking YNAB config table: %w", err)
	}

	if count == 0 {
		// Get API token from environment variable with fallback to empty string
		apiToken := os.Getenv("YNAB_API_TOKEN")

		_, err = db.Exec(`
			INSERT INTO ynab_config (id, api_token, budget_id, sync_frequency) 
			VALUES (1, ?, '', 60)`,
			apiToken)
		if err != nil {
			return fmt.Errorf("failed to initialize YNAB config: %w", err)
		}
	}

	log.Println("AddYNABConfigTable migration completed")
	return nil
}
