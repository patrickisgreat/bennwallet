package middleware

import (
	"database/sql"
	"log"
	"time"

	"bennwallet/backend/database"
	"bennwallet/backend/models"
)

// CheckUserPermission checks if a user has permission to access a resource
// Returns true if:
// 1. The user is the owner of the resource
// 2. The user is an admin
// 3. The user has been granted explicit permission to access the resource
func CheckUserPermission(userID, resourceOwnerID, resourceType, permissionType string) bool {
	// Always allow users to access their own resources
	if userID == resourceOwnerID {
		return true
	}

	// Check if user is an admin
	var isAdmin bool
	err := database.DB.QueryRow("SELECT isAdmin FROM users WHERE id = ?", userID).Scan(&isAdmin)
	if err != nil {
		log.Printf("Error checking if user %s is admin: %v", userID, err)
	} else if isAdmin {
		// Admins have access to everything
		return true
	}

	// Check if user has been granted explicit permission
	var permissionExists bool
	now := time.Now()

	err = database.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM permissions 
			WHERE granted_user_id = ? 
			AND owner_user_id = ? 
			AND resource_type IN (?, 'all')
			AND permission_type = ?
			AND (expires_at IS NULL OR expires_at > ?)
		)
	`, userID, resourceOwnerID, resourceType, permissionType, now).Scan(&permissionExists)

	if err != nil {
		log.Printf("Error checking permission for user %s on resource %s: %v", userID, resourceType, err)
		return false
	}

	// Check for write permission if read permission was requested
	// (write permission implies read permission)
	if !permissionExists && permissionType == models.PermissionRead {
		err = database.DB.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM permissions 
				WHERE granted_user_id = ? 
				AND owner_user_id = ? 
				AND resource_type IN (?, 'all')
				AND permission_type = 'write'
				AND (expires_at IS NULL OR expires_at > ?)
			)
		`, userID, resourceOwnerID, resourceType, now).Scan(&permissionExists)

		if err != nil {
			log.Printf("Error checking write permission for user %s on resource %s: %v", userID, resourceType, err)
			return false
		}
	}

	return permissionExists
}

// GetUsersWithAccessToResource gets all users who have access to a resource
func GetUsersWithAccessToResource(resourceOwnerID, resourceType string) ([]string, error) {
	rows, err := database.DB.Query(`
		SELECT DISTINCT granted_user_id FROM permissions 
		WHERE owner_user_id = ? 
		AND resource_type IN (?, 'all')
		AND (expires_at IS NULL OR expires_at > ?)
	`, resourceOwnerID, resourceType, time.Now())

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, userID)
	}

	return userIDs, nil
}

// GetUserAccessibleResources gets all resources that a user has access to
func GetUserAccessibleResources(userID, resourceType, permissionType string) ([]string, error) {
	// Get user's admin status
	var isAdmin bool
	err := database.DB.QueryRow("SELECT isAdmin FROM users WHERE id = ?", userID).Scan(&isAdmin)
	if err != nil {
		log.Printf("Error checking if user %s is admin: %v", userID, err)
	}

	var rows *sql.Rows
	if isAdmin {
		// Admins can access all resources
		rows, err = database.DB.Query(`
			SELECT DISTINCT owner_user_id 
			FROM permissions 
			WHERE resource_type = ?
			UNION
			SELECT id FROM users
		`, resourceType)
	} else {
		// Regular users can only access resources they own or have been granted access to
		rows, err = database.DB.Query(`
			SELECT DISTINCT owner_user_id 
			FROM permissions 
			WHERE granted_user_id = ? 
			AND resource_type IN (?, 'all')
			AND permission_type IN (?, 'write')
			AND (expires_at IS NULL OR expires_at > ?)
			UNION
			SELECT ? as owner_user_id
		`, userID, resourceType, permissionType, time.Now(), userID)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ownerIDs []string
	for rows.Next() {
		var ownerID string
		if err := rows.Scan(&ownerID); err != nil {
			return nil, err
		}
		ownerIDs = append(ownerIDs, ownerID)
	}

	return ownerIDs, nil
}
