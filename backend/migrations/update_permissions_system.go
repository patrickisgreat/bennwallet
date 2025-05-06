package migrations

import (
	"database/sql"
	"fmt"
	"log"
)

// UpdatePermissionsSystem enhances the permissions system with the latest schema changes
func UpdatePermissionsSystem(db *sql.DB) error {
	log.Println("Updating permissions system...")

	// Step 1: Update Users table - add superadmin role
	err := updateUsersTableForSuperAdmin(db)
	if err != nil {
		return fmt.Errorf("failed to update users table: %w", err)
	}

	// Step 2: Update Permissions table - add 'all' permission type
	err = updatePermissionsTable(db)
	if err != nil {
		return fmt.Errorf("failed to update permissions table: %w", err)
	}

	// Step 3: Create saved filters table
	err = createSavedFiltersTable(db)
	if err != nil {
		return fmt.Errorf("failed to create saved filters table: %w", err)
	}

	// Step 4: Create custom reports table
	err = createCustomReportsTable(db)
	if err != nil {
		return fmt.Errorf("failed to create custom reports table: %w", err)
	}

	// Step 5: Set initial superadmin (if applicable)
	err = setInitialSuperAdmin(db)
	if err != nil {
		return fmt.Errorf("failed to set initial superadmin: %w", err)
	}

	log.Println("Permissions system updated successfully")
	return nil
}

func updateUsersTableForSuperAdmin(db *sql.DB) error {
	// Update existing admin users to have proper role string
	_, err := db.Exec(`
		UPDATE users 
		SET role = 
			CASE 
				WHEN isAdmin = 1 THEN 'admin'
				ELSE 'user'
			END
		WHERE role IS NULL OR role = ''
	`)
	if err != nil {
		return fmt.Errorf("failed to update admin roles: %w", err)
	}

	// Set hardcoded admins to have 'admin' role
	_, err = db.Exec(`
		UPDATE users
		SET role = 'admin', isAdmin = 1
		WHERE name IN ('Sarah', 'Patrick') AND (role IS NULL OR role != 'superadmin')
	`)
	if err != nil {
		return fmt.Errorf("failed to set hardcoded admins: %w", err)
	}

	return nil
}

func updatePermissionsTable(db *sql.DB) error {
	// No schema changes needed for the permissions table
	// since 'all' permission type can be used without schema change

	// Add indexes for better performance
	_, err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_permissions_granted_user_id ON permissions (granted_user_id);
		CREATE INDEX IF NOT EXISTS idx_permissions_owner_user_id ON permissions (owner_user_id);
	`)
	if err != nil {
		return fmt.Errorf("failed to create permission indexes: %w", err)
	}

	return nil
}

func createSavedFiltersTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS saved_filters (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			user_id TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			filter_config TEXT NOT NULL,
			is_default BOOLEAN DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);
		
		CREATE INDEX IF NOT EXISTS idx_saved_filters_user_id ON saved_filters (user_id);
	`)
	if err != nil {
		return fmt.Errorf("failed to create saved filters table: %w", err)
	}

	return nil
}

func createCustomReportsTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS custom_reports (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			user_id TEXT NOT NULL,
			description TEXT,
			report_config TEXT NOT NULL,
			is_public BOOLEAN DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);
		
		CREATE INDEX IF NOT EXISTS idx_custom_reports_user_id ON custom_reports (user_id);
		CREATE INDEX IF NOT EXISTS idx_custom_reports_public ON custom_reports (is_public);
	`)
	if err != nil {
		return fmt.Errorf("failed to create custom reports table: %w", err)
	}

	return nil
}

func setInitialSuperAdmin(db *sql.DB) error {
	// Check for existing superadmin
	var hasSuperAdmin bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE role = 'superadmin')").Scan(&hasSuperAdmin)
	if err != nil {
		return fmt.Errorf("failed to check for existing superadmin: %w", err)
	}

	// If no superadmin exists, set Patrick or Sarah as superadmin
	if !hasSuperAdmin {
		// Try to find Patrick first
		var patrickId string
		err := db.QueryRow("SELECT id FROM users WHERE name = 'Patrick' LIMIT 1").Scan(&patrickId)
		if err == nil && patrickId != "" {
			_, err = db.Exec("UPDATE users SET role = 'superadmin' WHERE id = ?", patrickId)
			if err != nil {
				return fmt.Errorf("failed to set Patrick as superadmin: %w", err)
			}
			log.Println("Patrick set as initial superadmin")
			return nil
		}

		// Try Sarah next
		var sarahId string
		err = db.QueryRow("SELECT id FROM users WHERE name = 'Sarah' LIMIT 1").Scan(&sarahId)
		if err == nil && sarahId != "" {
			_, err = db.Exec("UPDATE users SET role = 'superadmin' WHERE id = ?", sarahId)
			if err != nil {
				return fmt.Errorf("failed to set Sarah as superadmin: %w", err)
			}
			log.Println("Sarah set as initial superadmin")
			return nil
		}

		// No eligible users found
		log.Println("No eligible users found for superadmin role. Please set manually.")
	}

	return nil
}
