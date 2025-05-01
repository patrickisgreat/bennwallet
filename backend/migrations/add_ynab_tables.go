package migrations

import (
	"database/sql"
	"log"
)

// AddYNABTables adds tables for YNAB integration
func AddYNABTables(db *sql.DB) error {
	log.Println("Adding YNAB tables...")

	// Create YNAB category groups table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS ynab_category_groups (
			id TEXT NOT NULL,
			name TEXT NOT NULL,
			user_id TEXT NOT NULL,
			last_updated DATETIME NOT NULL,
			PRIMARY KEY (id, user_id)
		)
	`)
	if err != nil {
		log.Printf("Error creating ynab_category_groups table: %v", err)
		return err
	}

	// Create YNAB categories table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ynab_categories (
			id TEXT NOT NULL,
			group_id TEXT NOT NULL,
			name TEXT NOT NULL,
			user_id TEXT NOT NULL,
			last_updated DATETIME NOT NULL,
			PRIMARY KEY (id, user_id),
			FOREIGN KEY (group_id, user_id) REFERENCES ynab_category_groups(id, user_id)
		)
	`)
	if err != nil {
		log.Printf("Error creating ynab_categories table: %v", err)
		return err
	}

	// Create user YNAB settings table
	_, err = db.Exec(`
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

	// Add status and isAdmin columns to users table if they don't exist
	_, err = db.Exec(`
		PRAGMA table_info(users)
	`)
	if err != nil {
		log.Printf("Error checking users table schema: %v", err)
		return err
	}

	// Add status column if it doesn't exist
	_, err = db.Exec(`
		ALTER TABLE users ADD COLUMN status TEXT DEFAULT 'approved'
	`)
	if err != nil {
		log.Printf("Error adding status column or column already exists: %v", err)
	}

	// Add isAdmin column if it doesn't exist
	_, err = db.Exec(`
		ALTER TABLE users ADD COLUMN isAdmin BOOLEAN DEFAULT 0
	`)
	if err != nil {
		log.Printf("Error adding isAdmin column or column already exists: %v", err)
	}

	log.Println("YNAB tables added successfully")
	return nil
}
