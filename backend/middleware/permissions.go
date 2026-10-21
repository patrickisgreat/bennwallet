package middleware

import (
	"fmt"
	"log"

	"bennwallet/backend/database"
)

// CheckUserPermission checks if a user has a specific permission for a resource owned by another user
func CheckUserPermission(grantedUserID, ownerUserID, resourceType, permissionType string) bool {
	// Log the permission check
	log.Printf("Checking permission for user %s to access resource %s owned by %s (permission: %s)",
		grantedUserID, resourceType, ownerUserID, permissionType)

	// Self-check: Users always have permission to access their own resources
	if grantedUserID == ownerUserID {
		log.Printf("Permission granted - self check: user %s == owner %s", grantedUserID, ownerUserID)
		return true
	}

	// Special case: Patrick Bennett's Firebase ID - grant all permissions
	if grantedUserID == "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2" {
		log.Printf("Permission granted - special admin access for Patrick Bennett (user: %s)", grantedUserID)
		return true
	}

	// Check if the user is a superadmin
	var role string
	err := database.DB.QueryRow("SELECT role FROM users WHERE id = $1", grantedUserID).Scan(&role)
	if err == nil && (role == "superadmin" || role == "admin") {
		log.Printf("Permission granted - user %s has role %s", grantedUserID, role)
		return true
	}

	// Query the database to check if the permission exists
	var exists bool
	query := `
		SELECT EXISTS (
			SELECT 1 FROM permissions
			WHERE granted_user_id = $1
			AND owner_user_id = $2
			AND resource_type = $3
			AND permission_type = $4
		)
	`
	err = database.DB.QueryRow(query, grantedUserID, ownerUserID, resourceType, permissionType).Scan(&exists)
	if err != nil {
		log.Printf("Error checking permissions: %v", err)
		return false
	}

	log.Printf("Permission %s for resource %s: %v", permissionType, resourceType, exists)
	return exists
}

// GetUserAccessibleResources returns a list of user IDs for which the current user has access to their resources
func GetUserAccessibleResources(userID, resourceType, permissionType string) ([]string, error) {
	log.Printf("Getting accessible resources for user %s (resource: %s, permission: %s)",
		userID, resourceType, permissionType)

	// Special case: Patrick Bennett's Firebase ID - grant access to all users
	if userID == "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2" {
		log.Printf("Special case for Patrick Bennett - getting all users")
		var allUserIDs []string
		rows, err := database.DB.Query("SELECT id FROM users")
		if err != nil {
			return nil, fmt.Errorf("error querying users for admin: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return nil, fmt.Errorf("error scanning user ID: %w", err)
			}
			allUserIDs = append(allUserIDs, id)
		}

		// Always add the user's own ID
		allUserIDs = append(allUserIDs, userID)
		log.Printf("Returning all user IDs for Patrick Bennett: %v", allUserIDs)
		return allUserIDs, nil
	}

	// Check for superadmin role
	var role string
	err := database.DB.QueryRow("SELECT role FROM users WHERE id = $1", userID).Scan(&role)
	if err == nil && (role == "superadmin" || role == "admin") {
		log.Printf("User %s has role %s - getting all users", userID, role)
		var allUserIDs []string
		rows, err := database.DB.Query("SELECT id FROM users")
		if err != nil {
			return nil, fmt.Errorf("error querying users for admin: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return nil, fmt.Errorf("error scanning user ID: %w", err)
			}
			allUserIDs = append(allUserIDs, id)
		}

		log.Printf("Returning all user IDs for admin user: %v", allUserIDs)
		return allUserIDs, nil
	}

	// Regular permission check
	query := `
		SELECT owner_user_id FROM permissions
		WHERE granted_user_id = $1
		AND resource_type = $2
		AND permission_type = $3
	`
	rows, err := database.DB.Query(query, userID, resourceType, permissionType)
	if err != nil {
		return nil, fmt.Errorf("error querying permissions: %w", err)
	}
	defer rows.Close()

	var ownerUserIDs []string
	for rows.Next() {
		var ownerUserID string
		if err := rows.Scan(&ownerUserID); err != nil {
			return nil, fmt.Errorf("error scanning owner user ID: %w", err)
		}
		ownerUserIDs = append(ownerUserIDs, ownerUserID)
	}

	// Always add the user's own ID
	ownerUserIDs = append(ownerUserIDs, userID)

	log.Printf("Found accessible resources for user %s: %v", userID, ownerUserIDs)
	return ownerUserIDs, nil
}
