package migrations

import (
	"database/sql"
	"fmt"
	"log"

	"bennwallet/backend/models"
)

// FixYNABSchema ensures the YNAB config table has the correct schema
// This is particularly important for fixing schema issues in production
func FixYNABSchema(db *sql.DB) error {
	log.Println("Running YNAB schema fix migration...")

	err := models.FixYNABTableSchema(db)
	if err != nil {
		return fmt.Errorf("failed to fix YNAB schema: %w", err)
	}

	log.Println("Successfully completed YNAB schema fix migration")
	return nil
}
