package migrations

import (
	"database/sql"
	"log"
)

// FixYNABAPIURLs is a documentation migration for the fixed YNAB API URLs
func FixYNABAPIURLs(db *sql.DB) error {
	log.Println("Running migration: FixYNABAPIURLs")

	// This is just a documentation migration - the actual fix was made in the code
	// We corrected URLs from https://api.youneedabudget.com.com to https://api.youneedabudget.com
	// in the following files:
	// - backend/services/ynab.go
	// - backend/ynab/sync.go
	// - backend/services/ynab_sync.go
	// - backend/models/ynab_sync.go

	// Record this migration
	_, err := db.Exec(`
		INSERT INTO migrations (name)
		VALUES ('fix_ynab_api_urls')
		ON CONFLICT (name) DO NOTHING
	`)

	if err != nil {
		return err
	}

	log.Println("Migration FixYNABAPIURLs completed successfully")
	return nil
}
