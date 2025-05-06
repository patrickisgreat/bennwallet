package migrations

import (
	"database/sql"
	"fmt"
	"log"
)

// AddPermissionsTable adds the permissions table to the database
func AddPermissionsTable(db *sql.DB) error {
	log.Println("Adding permissions table...")

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS permissions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			granted_user_id TEXT NOT NULL,
			owner_user_id TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			permission_type TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP,
			UNIQUE(granted_user_id, owner_user_id, resource_type, permission_type)
		);
	`)

	if err != nil {
		return fmt.Errorf("failed to create permissions table: %w", err)
	}

	// Create an index for faster permission lookup
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_permissions_lookup ON permissions (
			granted_user_id, owner_user_id, resource_type, permission_type
		);
	`)

	if err != nil {
		return fmt.Errorf("failed to create permissions index: %w", err)
	}

	log.Println("Permissions table created successfully")
	return nil
}
