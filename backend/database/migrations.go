package database

import (
	"log"

	"bennwallet/backend/migrations"
)

// RunMigrations runs all database migrations
func RunMigrations() error {
	log.Println("Running database migrations...")

	// Run YNAB config table migration
	if err := migrations.AddYNABConfigTable(DB); err != nil {
		log.Printf("Error running YNAB config table migration: %v", err)
		return err
	}

	// Legacy: ensure user_ynab_settings table exists
	_, err := DB.Exec(`
		CREATE TABLE IF NOT EXISTS user_ynab_settings (
			user_id TEXT PRIMARY KEY,
			token TEXT NOT NULL,
			budget_id TEXT NOT NULL,
			account_id TEXT NOT NULL,
			sync_enabled BOOLEAN NOT NULL DEFAULT 0,
			last_synced DATETIME
		)
	`)
	if err != nil {
		log.Printf("Error creating user_ynab_settings table: %v", err)
		return err
	}

	log.Println("Database migrations completed successfully")
	return nil
}
