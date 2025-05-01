package migrations

import (
	"database/sql"
	"log"
)

// AddCategoriesUniqueConstraint adds a unique constraint to categories table
func AddCategoriesUniqueConstraint(db *sql.DB) error {
	log.Println("Adding unique constraint to categories table...")

	// First, create a temporary table with the desired structure
	_, err := db.Exec(`
		CREATE TABLE categories_temp (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT,
			user_id TEXT NOT NULL,
			color TEXT,
			last_updated DATETIME,
			UNIQUE(name, user_id)
		)
	`)
	if err != nil {
		log.Printf("Error creating temporary categories table: %v", err)
		return err
	}

	// Copy data from the original table
	_, err = db.Exec(`
		INSERT INTO categories_temp (id, name, description, user_id, color, last_updated)
		SELECT id, name, description, user_id, color, NULL FROM categories
	`)
	if err != nil {
		log.Printf("Error copying data to temporary categories table: %v", err)
		return err
	}

	// Drop the original table
	_, err = db.Exec(`DROP TABLE categories`)
	if err != nil {
		log.Printf("Error dropping original categories table: %v", err)
		return err
	}

	// Rename the temporary table to the original name
	_, err = db.Exec(`ALTER TABLE categories_temp RENAME TO categories`)
	if err != nil {
		log.Printf("Error renaming temporary table: %v", err)
		return err
	}

	log.Println("Successfully added unique constraint to categories table")
	return nil
}
