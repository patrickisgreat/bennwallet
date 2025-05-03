package migrations

import (
	"database/sql"
	"fmt"
	"log"
)

// AddPermissionsTable adds the permissions table and updates the users table
func AddPermissionsTable(db *sql.DB) error {
	log.Println("Adding permissions table and updating users table...")

	// Start a transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Check if the role column exists in the users table
	var roleColumnExists bool
	err = tx.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM pragma_table_info('users') 
		WHERE name = 'role'
	`).Scan(&roleColumnExists)

	if err != nil {
		return fmt.Errorf("error checking for role column: %w", err)
	}

	// Check if the isAdmin column exists in the users table
	var isAdminColumnExists bool
	err = tx.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM pragma_table_info('users') 
		WHERE name = 'isAdmin'
	`).Scan(&isAdminColumnExists)

	if err != nil {
		return fmt.Errorf("error checking for isAdmin column: %w", err)
	}

	// Add the role column if it doesn't exist
	if !roleColumnExists {
		_, err = tx.Exec(`
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
		_, err = tx.Exec(`
			ALTER TABLE users
			ADD COLUMN isAdmin BOOLEAN DEFAULT 0
		`)
		if err != nil {
			return fmt.Errorf("error adding isAdmin column: %w", err)
		}
		log.Println("Added isAdmin column to users table")
	}

	// Set Sarah and Patrick as admins if they exist
	var sarahExists, patrickExists bool

	err = tx.QueryRow("SELECT COUNT(*) > 0 FROM users WHERE name = 'Sarah'").Scan(&sarahExists)
	if err != nil {
		return fmt.Errorf("error checking if Sarah exists: %w", err)
	}

	err = tx.QueryRow("SELECT COUNT(*) > 0 FROM users WHERE name = 'Patrick'").Scan(&patrickExists)
	if err != nil {
		return fmt.Errorf("error checking if Patrick exists: %w", err)
	}

	if sarahExists {
		_, err = tx.Exec(`
			UPDATE users
			SET role = 'admin', isAdmin = 1
			WHERE name = 'Sarah'
		`)
		if err != nil {
			return fmt.Errorf("error setting Sarah as admin: %w", err)
		}
		log.Println("Set Sarah as admin")
	}

	if patrickExists {
		_, err = tx.Exec(`
			UPDATE users
			SET role = 'admin', isAdmin = 1
			WHERE name = 'Patrick'
		`)
		if err != nil {
			return fmt.Errorf("error setting Patrick as admin: %w", err)
		}
		log.Println("Set Patrick as admin")
	}

	// Check if permissions table already exists
	var permissionsTableExists bool
	err = tx.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM sqlite_master 
		WHERE type='table' AND name='permissions'
	`).Scan(&permissionsTableExists)

	if err != nil {
		return fmt.Errorf("error checking if permissions table exists: %w", err)
	}

	// Create the permissions table if it doesn't exist
	if !permissionsTableExists {
		_, err = tx.Exec(`
			CREATE TABLE permissions (
				id TEXT PRIMARY KEY,
				owner_user_id TEXT NOT NULL,
				granted_user_id TEXT NOT NULL,
				permission_type TEXT NOT NULL CHECK(permission_type IN ('read', 'write')),
				resource_type TEXT NOT NULL,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				expires_at DATETIME,
				FOREIGN KEY (owner_user_id) REFERENCES users(id) ON DELETE CASCADE,
				FOREIGN KEY (granted_user_id) REFERENCES users(id) ON DELETE CASCADE,
				UNIQUE(owner_user_id, granted_user_id, resource_type)
			)
		`)
		if err != nil {
			return fmt.Errorf("error creating permissions table: %w", err)
		}
		log.Println("Created permissions table")

		// Create an index on the permissions table
		_, err = tx.Exec(`
			CREATE INDEX idx_permissions_granted_user_id
			ON permissions(granted_user_id)
		`)
		if err != nil {
			return fmt.Errorf("error creating permissions index: %w", err)
		}
		log.Println("Created permissions index")
	} else {
		log.Println("Permissions table already exists, skipping creation")
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}

	log.Println("Successfully added permissions table and updated users table")
	return nil
}
