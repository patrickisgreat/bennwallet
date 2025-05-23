package migrations

import (
	"database/sql"
	"log"
)

// FixYNABCategoriesColumn ensures both group_id and category_group_id columns exist
// and copies values between them to fix category grouping issues
func FixYNABCategoriesColumn(db *sql.DB) error {
	log.Println("Running migration to fix YNAB categories column names")

	// Check if ynab_categories table exists
	var categoriesTableExists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'ynab_categories'
		)
	`).Scan(&categoriesTableExists)
	if err != nil {
		return err
	}

	if !categoriesTableExists {
		log.Println("ynab_categories table doesn't exist, nothing to migrate")
		return nil
	}

	// Check if group_id column exists
	var hasGroupId bool
	err = db.QueryRow(`
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

	// Check if category_group_id column exists
	var hasCategoryGroupId bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT FROM information_schema.columns 
			WHERE table_schema = 'public' 
			AND table_name = 'ynab_categories' 
			AND column_name = 'category_group_id'
		)
	`).Scan(&hasCategoryGroupId)
	if err != nil {
		return err
	}

	// Add any missing columns
	if !hasGroupId {
		log.Println("Adding missing group_id column")
		_, err := db.Exec(`ALTER TABLE ynab_categories ADD COLUMN group_id TEXT;`)
		if err != nil {
			return err
		}
	}

	if !hasCategoryGroupId {
		log.Println("Adding missing category_group_id column")
		_, err := db.Exec(`ALTER TABLE ynab_categories ADD COLUMN category_group_id TEXT;`)
		if err != nil {
			return err
		}
	}

	// Now copy data between columns if they exist but one is empty
	if hasGroupId && hasCategoryGroupId {
		log.Println("Copying data between group_id and category_group_id columns")

		// First, copy from group_id to category_group_id where category_group_id is null
		_, err = db.Exec(`
			UPDATE ynab_categories
			SET category_group_id = group_id
			WHERE category_group_id IS NULL AND group_id IS NOT NULL
		`)
		if err != nil {
			return err
		}

		// Then, copy from category_group_id to group_id where group_id is null
		_, err = db.Exec(`
			UPDATE ynab_categories
			SET group_id = category_group_id
			WHERE group_id IS NULL AND category_group_id IS NOT NULL
		`)
		if err != nil {
			return err
		}
	}

	log.Println("YNAB categories column migration completed successfully")
	return nil
}
