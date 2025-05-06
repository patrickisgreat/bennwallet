package middleware

import (
	"log"
	"net/http"

	"bennwallet/backend/models"
	"bennwallet/backend/services"
)

// CheckUserPermission checks if a user has permission to access a resource
// Returns true if:
// 1. The user is the owner of the resource
// 2. The user is an admin
// 3. The user has been granted explicit permission to access the resource
func CheckUserPermission(userID, resourceOwnerID, resourceType, permissionType string) bool {
	hasPermission, err := services.CheckPermission(userID, resourceOwnerID, resourceType, permissionType)
	if err != nil {
		log.Printf("Error checking permission for user %s on resource %s: %v", userID, resourceType, err)
		return false
	}
	return hasPermission
}

// GetUsersWithAccessToResource gets all users who have access to a resource
func GetUsersWithAccessToResource(resourceOwnerID, resourceType string) ([]string, error) {
	// This is now a wrapper around the service function
	return services.GetAccessibleResources(resourceOwnerID, resourceType, models.PermissionRead)
}

// GetUserAccessibleResources gets all resources that a user has access to
func GetUserAccessibleResources(userID, resourceType, permissionType string) ([]string, error) {
	// This is now a wrapper around the service function
	return services.GetAccessibleResources(userID, resourceType, permissionType)
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
