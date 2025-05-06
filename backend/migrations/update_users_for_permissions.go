package migrations

import (
	"database/sql"
	"fmt"
	"log"
)

// UpdateUsersForPermissions updates the users table to add roles and admin flags
func UpdateUsersForPermissions(db *sql.DB) error {
	log.Println("Updating users table for permissions system...")

	// Check if the role column exists in the users table
	var roleColumnExists bool
	err := db.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM pragma_table_info('users') 
		WHERE name = 'role'
	`).Scan(&roleColumnExists)

	if err != nil {
		return fmt.Errorf("error checking for role column: %w", err)
	}

	// Check if the isAdmin column exists in the users table
	var isAdminColumnExists bool
	err = db.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM pragma_table_info('users') 
		WHERE name = 'isAdmin'
	`).Scan(&isAdminColumnExists)

	if err != nil {
		return fmt.Errorf("error checking for isAdmin column: %w", err)
	}

	// Add the role column if it doesn't exist
	if !roleColumnExists {
		_, err = db.Exec(`
			ALTER TABLE users
			ADD COLUMN role TEXT DEFAULT 'user'
		`)
		if err != nil {
			return fmt.Errorf("error adding role column: %w", err)
		}
		log.Println("Added role column to users table")
	}

	// Add the isAdmin column if it doesn't exist
	if !isAdminColumnExists {
		_, err = db.Exec(`
			ALTER TABLE users
			ADD COLUMN isAdmin BOOLEAN DEFAULT 0
		`)
		if err != nil {
			return fmt.Errorf("error adding isAdmin column: %w", err)
		}
		log.Println("Added isAdmin column to users table")
	}

	// Check if the userId column exists in the transactions table
	var hasUserIdColumn bool
	err = db.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM pragma_table_info('transactions') 
		WHERE name = 'userId'
	`).Scan(&hasUserIdColumn)

	if err != nil {
		log.Printf("Error checking for userId column in transactions: %v", err)
		hasUserIdColumn = false
	}

	// Only try to update users based on transaction volume if the userId column exists
	if hasUserIdColumn {
		_, err = db.Exec(`
			UPDATE users
			SET isAdmin = 1, role = 'admin'
			WHERE id IN (
				SELECT DISTINCT userId FROM transactions 
				GROUP BY userId 
				HAVING COUNT(*) > 10
			)
		`)

		if err != nil {
			return fmt.Errorf("error updating admin users based on transaction volume: %w", err)
		}
		log.Println("Updated admin users based on transaction volume")
	}

	// Also use DefaultAdmins from models to set specific users as admins
	_, err = db.Exec(`
		UPDATE users
		SET isAdmin = 1, role = 'admin'
		WHERE name IN ('Sarah', 'Patrick')
	`)

	if err != nil {
		return fmt.Errorf("error updating admin users based on default list: %w", err)
	}
	log.Println("Updated admin users based on default list")

	log.Println("Users table updated successfully")
	return nil
}
