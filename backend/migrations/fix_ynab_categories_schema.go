package migrations

import (
	"database/sql"
	"fmt"
	"log"
)

// FixYNABCategoriesSchema ensures the YNAB categories tables have the correct schema
func FixYNABCategoriesSchema(db *sql.DB) error {
	log.Println("Running migration: FixYNABCategoriesSchema")

	// Check if the ynab_category_groups table exists
	var tableExists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'ynab_category_groups'
		)
	`).Scan(&tableExists)

	if err != nil {
		return fmt.Errorf("error checking if ynab_category_groups table exists: %w", err)
	}

	if !tableExists {
		log.Println("Creating ynab_category_groups table...")
		_, err = db.Exec(`
			CREATE TABLE ynab_category_groups (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				category_group_id TEXT NOT NULL,
				user_id TEXT NOT NULL REFERENCES users(id),
				created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
			)
		`)
		if err != nil {
			return fmt.Errorf("error creating ynab_category_groups table: %w", err)
		}
	} else {
		// Check if the last_updated column exists
		var lastUpdatedExists bool
		err = db.QueryRow(`
			SELECT EXISTS (
				SELECT column_name 
				FROM information_schema.columns 
				WHERE table_schema = 'public' 
				AND table_name = 'ynab_category_groups'
				AND column_name = 'last_updated'
			)
		`).Scan(&lastUpdatedExists)

		if err != nil {
			return fmt.Errorf("error checking if last_updated column exists: %w", err)
		}

		// Add the last_updated column if it doesn't exist
		if !lastUpdatedExists {
			log.Println("Adding last_updated column to ynab_category_groups table...")
			_, err = db.Exec(`ALTER TABLE ynab_category_groups ADD COLUMN last_updated TIMESTAMP WITH TIME ZONE`)
			if err != nil {
				return fmt.Errorf("error adding last_updated column: %w", err)
			}
		}
	}

	// Check if the ynab_categories table exists
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'ynab_categories'
		)
	`).Scan(&tableExists)

	if err != nil {
		return fmt.Errorf("error checking if ynab_categories table exists: %w", err)
	}

	if !tableExists {
		log.Println("Creating ynab_categories table...")
		_, err = db.Exec(`
			CREATE TABLE ynab_categories (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				group_id TEXT NOT NULL REFERENCES ynab_category_groups(id),
				user_id TEXT NOT NULL REFERENCES users(id),
				created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
			)
		`)
		if err != nil {
			return fmt.Errorf("error creating ynab_categories table: %w", err)
		}
	} else {
		// Check if the last_updated column exists
		var lastUpdatedExists bool
		err = db.QueryRow(`
			SELECT EXISTS (
				SELECT column_name 
				FROM information_schema.columns 
				WHERE table_schema = 'public' 
				AND table_name = 'ynab_categories'
				AND column_name = 'last_updated'
			)
		`).Scan(&lastUpdatedExists)

		if err != nil {
			return fmt.Errorf("error checking if last_updated column exists: %w", err)
		}

		// Add the last_updated column if it doesn't exist
		if !lastUpdatedExists {
			log.Println("Adding last_updated column to ynab_categories table...")
			_, err = db.Exec(`ALTER TABLE ynab_categories ADD COLUMN last_updated TIMESTAMP WITH TIME ZONE`)
			if err != nil {
				return fmt.Errorf("error adding last_updated column: %w", err)
			}
		}
	}

	// Record this migration
	_, err = db.Exec(`
		INSERT INTO migrations (name)
		VALUES ('fix_ynab_categories_schema')
		ON CONFLICT (name) DO NOTHING
	`)

	if err != nil {
		return err
	}

	log.Println("Migration FixYNABCategoriesSchema completed successfully")
	return nil
}
