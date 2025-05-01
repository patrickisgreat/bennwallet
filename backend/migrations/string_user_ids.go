package migrations

import (
	"database/sql"
	"log"
)

// StringUserIDs changes the users table to use TEXT for ID column to support Firebase auth IDs
func StringUserIDs(db *sql.DB) error {
	log.Println("Updating users table to use TEXT IDs...")

	// SQLite doesn't support ALTER COLUMN, so we need to:
	// 1. Rename the old table
	// 2. Create a new table with the correct schema
	// 3. Copy the data
	// 4. Drop the old table

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	// Check if we've already done this migration
	var count int
	err = tx.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='users_old'").Scan(&count)
	if err != nil {
		tx.Rollback()
		return err
	}

	if count > 0 {
		// The old table exists, which means migration was interrupted
		log.Println("Found incomplete previous migration attempt, cleaning up...")
		_, err = tx.Exec("DROP TABLE users_old")
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	// Step 1: Rename the existing table
	_, err = tx.Exec("ALTER TABLE users RENAME TO users_old")
	if err != nil {
		tx.Rollback()
		return err
	}

	// Step 2: Create a new table with TEXT id
	_, err = tx.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			name TEXT NOT NULL,
			status TEXT DEFAULT 'approved',
			isAdmin BOOLEAN DEFAULT 0
		)
	`)
	if err != nil {
		tx.Rollback()
		return err
	}

	// Step 3: Copy the data, converting id to TEXT
	_, err = tx.Exec(`
		INSERT INTO users (id, username, name, status, isAdmin)
		SELECT CAST(id AS TEXT), username, name, status, isAdmin
		FROM users_old
	`)
	if err != nil {
		tx.Rollback()
		return err
	}

	// Step 4: Drop the old table
	_, err = tx.Exec("DROP TABLE users_old")
	if err != nil {
		tx.Rollback()
		return err
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return err
	}

	log.Println("Successfully updated users table to use TEXT IDs")
	return nil
}
