package migrations

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
)

// EnsureAdminUser ensures that specific users have admin privileges
func EnsureAdminUser(db *sql.DB) error {
	log.Println("Ensuring admin users exist with proper privileges...")

	// List of admin user IDs - can be expanded as needed
	// First check environment variables for admin users
	adminUsers := []string{}

	// Check for ADMIN_USER_IDS environment variable
	if adminUserIDs := os.Getenv("ADMIN_USER_IDS"); adminUserIDs != "" {
		for _, id := range strings.Split(adminUserIDs, ",") {
			adminUsers = append(adminUsers, strings.TrimSpace(id))
		}
		log.Printf("Found %d admin users from ADMIN_USER_IDS env var", len(adminUsers))
	}

	// Add hardcoded admins - these should be application maintainers/owners
	// Including the Firebase UID for the user
	adminUsers = append(adminUsers,
		"UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", // Patrick Bennett's Firebase UID
	)

	log.Printf("Ensuring %d admin users exist with proper privileges", len(adminUsers))

	// Create transaction for all updates
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}

	for _, userID := range adminUsers {
		// First ensure user exists
		var exists bool
		err := tx.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", userID).Scan(&exists)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("error checking if user exists: %w", err)
		}

		if !exists {
			// Create user if it doesn't exist
			log.Printf("Creating admin user %s", userID)
			_, err := tx.Exec(`
				INSERT INTO users (id, username, name, role, is_admin)
				VALUES ($1, $2, $3, 'admin', TRUE)
			`, userID, fmt.Sprintf("admin_%s", userID), fmt.Sprintf("Admin User %s", userID))

			if err != nil {
				tx.Rollback()
				return fmt.Errorf("error creating admin user: %w", err)
			}
		} else {
			// Update existing user to ensure admin role
			log.Printf("Updating user %s to admin role", userID)
			_, err := tx.Exec(`
				UPDATE users 
				SET role = 'admin', is_admin = TRUE, 
				    name = CASE WHEN name LIKE 'User %' THEN $1 ELSE name END
				WHERE id = $2
			`, fmt.Sprintf("Admin User %s", userID), userID)

			if err != nil {
				tx.Rollback()
				return fmt.Errorf("error updating user to admin: %w", err)
			}
		}

		// Also ensure this user has necessary permissions for all features
		tables := []string{"categories", "transactions", "ynab_categories", "ynab_category_groups"}
		permissions := []string{"read", "write", "admin"}

		for _, table := range tables {
			for _, permission := range permissions {
				_, err := tx.Exec(`
					INSERT INTO permissions 
					(granted_user_id, owner_user_id, resource_type, permission_type)
					VALUES ($1, $1, $2, $3)
					ON CONFLICT (granted_user_id, owner_user_id, resource_type, permission_type) DO NOTHING
				`, userID, table, permission)

				if err != nil {
					log.Printf("Warning: Error setting permission %s.%s for user %s: %v",
						table, permission, userID, err)
					// Continue anyway - permissions table might be different in some environments
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}

	log.Println("Successfully ensured admin users exist with proper privileges")
	return nil
}
