package migrations

import "database/sql"

// AddYNABCategoriesTable creates or updates the YNAB categories table
func AddYNABCategoriesTable(db *sql.DB) error {
	// Check if table exists
	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'ynab_categories'
		)
	`).Scan(&exists)
	if err != nil {
		return err
	}

	if exists {
		// Table exists, check if group_id column exists
		var hasGroupId bool
		err := db.QueryRow(`
			SELECT EXISTS (
				SELECT FROM information_schema.columns 
				WHERE table_schema = 'public' 
				AND table_name = 'ynab_categories' 
				AND column_name = 'group_id'
			)
		`).Scan(&hasGroupId)
		if err != nil {
			return err
		}

		if !hasGroupId {
			// Add missing group_id column
			_, err := db.Exec(`ALTER TABLE ynab_categories ADD COLUMN group_id TEXT;`)
			if err != nil {
				return err
			}
		}
	} else {
		// Create the full table
		_, err := db.Exec(`
			CREATE TABLE ynab_categories (
				id SERIAL PRIMARY KEY,
				user_id TEXT NOT NULL,
				category_id TEXT NOT NULL,
				name TEXT NOT NULL,
				group_id TEXT,
				group_name TEXT,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			);
		`)
		if err != nil {
			return err
		}
	}

	return nil
}
