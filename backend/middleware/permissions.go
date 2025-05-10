package middleware

import (
	"fmt"
	"log"
	"net/http"

	"bennwallet/backend/database"
	"bennwallet/backend/models"
	"bennwallet/backend/services"
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

// GetUsersWithAccessToResource gets all users who have access to a resource
func GetUsersWithAccessToResource(resourceOwnerID, resourceType string) ([]string, error) {
	// This is now a wrapper around the service function
	return services.GetAccessibleResources(resourceOwnerID, resourceType, models.PermissionRead)
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

// RequirePermission is a middleware that ensures the user has permission to access a resource
func RequirePermission(resourceType, permissionType string, getResourceOwnerID func(r *http.Request) (string, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user ID from context
			userID := GetUserIDFromContext(r)
			if userID == "" {
				http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
				return
			}

			// Get resource owner ID
			resourceOwnerID, err := getResourceOwnerID(r)
			if err != nil {
				http.Error(w, "Failed to determine resource owner: "+err.Error(), http.StatusInternalServerError)
				return
			}

			// Check permission
			if !CheckUserPermission(userID, resourceOwnerID, resourceType, permissionType) {
				http.Error(w, "Forbidden: Insufficient permissions", http.StatusForbidden)
				return
			}

			// Permission check passed, continue
			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole is a middleware that ensures the user has at least the specified role
func RequireRole(requiredRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user ID from context
			userID := GetUserIDFromContext(r)
			if userID == "" {
				http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
				return
			}

			// Get user's role
			userRole, err := services.GetUserRole(userID)
			if err != nil {
				http.Error(w, "Failed to get user role: "+err.Error(), http.StatusInternalServerError)
				return
			}

			// Check if user's role is sufficient
			if !services.IsRoleAtLeast(userRole, requiredRole) {
				http.Error(w, "Forbidden: Insufficient role privileges", http.StatusForbidden)
				return
			}

			// Role check passed, continue
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAdmin is a middleware that ensures the user is an admin
func RequireAdmin() func(http.Handler) http.Handler {
	return RequireRole(models.RoleAdmin)
}

// RequireSuperAdmin is a middleware that ensures the user is a super admin
func RequireSuperAdmin() func(http.Handler) http.Handler {
	return RequireRole(models.RoleSuperAdmin)
}
