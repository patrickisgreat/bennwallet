package migrations

import (
	"database/sql"
	"fmt"
	"log"
)

// AddOptionalField adds the optional boolean field to the transactions table
func AddOptionalField(db *sql.DB) error {
	log.Println("Adding optional field to transactions table...")

	// First check if the column already exists
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) 
		FROM pragma_table_info('transactions') 
		WHERE name = 'optional'
	`).Scan(&count)

	if err != nil {
		return fmt.Errorf("error checking for optional column: %w", err)
	}

	if count > 0 {
		log.Println("Optional column already exists in transactions table")
		return nil
	}

	// Add the column
	_, err = db.Exec(`
		ALTER TABLE transactions
		ADD COLUMN optional BOOLEAN NOT NULL DEFAULT 0
	`)
	if err != nil {
		return fmt.Errorf("error adding optional column: %w", err)
	}

	log.Println("Successfully added optional field to transactions table")
	return nil
}
