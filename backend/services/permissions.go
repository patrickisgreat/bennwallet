package services

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"bennwallet/backend/database"
	"bennwallet/backend/models"
)

// RoleHierarchy defines the hierarchy of roles in the system
// Higher numbers have more permissions
var RoleHierarchy = map[string]int{
	"user":       1,
	"admin":      2,
	"superadmin": 3,
}

// IsRoleAtLeast checks if a role is at least at the specified level
func IsRoleAtLeast(userRole, requiredRole string) bool {
	userLevel, userExists := RoleHierarchy[userRole]
	requiredLevel, requiredExists := RoleHierarchy[requiredRole]

	// If the role doesn't exist in our hierarchy, default behavior
	if !userExists || !requiredExists {
		return userRole == requiredRole
	}

	return userLevel >= requiredLevel
}

// GetUserRole gets the role of a user
func GetUserRole(userID string) (string, error) {
	var role sql.NullString
	err := database.DB.QueryRow("SELECT role FROM users WHERE id = ?", userID).Scan(&role)
	if err != nil {
		return "", err
	}

	if !role.Valid || role.String == "" {
		return "user", nil // Default role
	}

	return role.String, nil
}

// IsSuperAdmin checks if a user is a super admin
func IsSuperAdmin(userID string) (bool, error) {
	role, err := GetUserRole(userID)
	if err != nil {
		return false, err
	}

	return role == "superadmin", nil
}

// IsAdmin checks if a user is an admin or super admin
func IsAdmin(userID string) (bool, error) {
	role, err := GetUserRole(userID)
	if err != nil {
		return false, err
	}

	return IsRoleAtLeast(role, "admin"), nil
}

// SetUserRole sets the role of a user
// Only superadmins can set other users to superadmin
// Only admins or higher can set other users' roles
func SetUserRole(actorID, targetUserID, newRole string) error {
	// Validate role
	if _, exists := RoleHierarchy[newRole]; !exists {
		return fmt.Errorf("invalid role: %s", newRole)
	}

	// Get actor's role
	actorRole, err := GetUserRole(actorID)
	if err != nil {
		return fmt.Errorf("failed to get actor role: %w", err)
	}

	// Get target user's current role
	targetRole, err := GetUserRole(targetUserID)
	if err != nil {
		return fmt.Errorf("failed to get target user role: %w", err)
	}

	// Rules for role changes:
	// 1. Users can't change roles (except their own in special cases)
	// 2. Only superadmins can create other superadmins
	// 3. Can't demote yourself
	// 4. Admins can't change roles of other admins or superadmins

	if actorRole == "user" && (actorID != targetUserID || newRole != "user") {
		return fmt.Errorf("insufficient permissions to change roles")
	}

	if newRole == "superadmin" && actorRole != "superadmin" {
		return fmt.Errorf("only superadmins can create other superadmins")
	}

	if actorID == targetUserID && RoleHierarchy[newRole] < RoleHierarchy[actorRole] {
		return fmt.Errorf("cannot demote yourself")
	}

	if actorRole == "admin" && (targetRole == "admin" || targetRole == "superadmin") {
		return fmt.Errorf("admins cannot change roles of other admins or superadmins")
	}

	// All checks passed, update the role
	_, err = database.DB.Exec("UPDATE users SET role = ? WHERE id = ?", newRole, targetUserID)
	if err != nil {
		return fmt.Errorf("failed to update user role: %w", err)
	}

	// Update isAdmin for backward compatibility
	isAdmin := newRole == "admin" || newRole == "superadmin"
	_, err = database.DB.Exec("UPDATE users SET isAdmin = ? WHERE id = ?", isAdmin, targetUserID)
	if err != nil {
		return fmt.Errorf("failed to update isAdmin flag: %w", err)
	}

	return nil
}

// GrantPermission grants a permission from one user to another
func GrantPermission(granterID, granteeID, resourceType, permissionType string, expiresAt *time.Time) error {
	// Validate permission type
	if permissionType != models.PermissionRead && permissionType != models.PermissionWrite {
		return fmt.Errorf("invalid permission type: %s", permissionType)
	}

	// Check if granter has admin rights or is the resource owner
	isGranterAdmin, err := IsAdmin(granterID)
	if err != nil {
		return fmt.Errorf("failed to check granter admin status: %w", err)
	}

	if !isGranterAdmin && granterID != granteeID {
		// Regular users can only grant permissions to themselves
		return fmt.Errorf("insufficient permissions to grant access")
	}

	// Prepare SQL for inserting or replacing permission
	query := `
		INSERT INTO permissions 
		(granted_user_id, owner_user_id, resource_type, permission_type, created_at, expires_at) 
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(granted_user_id, owner_user_id, resource_type, permission_type) 
		DO UPDATE SET expires_at = excluded.expires_at
	`

	_, err = database.DB.Exec(
		query,
		granteeID,
		granterID,
		resourceType,
		permissionType,
		time.Now(),
		expiresAt,
	)

	if err != nil {
		return fmt.Errorf("failed to grant permission: %w", err)
	}

	return nil
}

// RevokePermission revokes a permission
func RevokePermission(revokerID, granteeID, ownerID, resourceType, permissionType string) error {
	// Check if revoker has admin rights or is the resource owner
	isRevokerAdmin, err := IsAdmin(revokerID)
	if err != nil {
		return fmt.Errorf("failed to check revoker admin status: %w", err)
	}

	if !isRevokerAdmin && revokerID != ownerID {
		// Regular users can only revoke permissions they granted
		return fmt.Errorf("insufficient permissions to revoke access")
	}

	// Delete the permission
	_, err = database.DB.Exec(
		"DELETE FROM permissions WHERE granted_user_id = ? AND owner_user_id = ? AND resource_type = ? AND permission_type = ?",
		granteeID,
		ownerID,
		resourceType,
		permissionType,
	)

	if err != nil {
		return fmt.Errorf("failed to revoke permission: %w", err)
	}

	return nil
}

// GetUserPermissions gets all permissions granted to a user
func GetUserPermissions(userID string) ([]models.Permission, error) {
	rows, err := database.DB.Query(`
		SELECT id, granted_user_id, owner_user_id, resource_type, permission_type, created_at, expires_at 
		FROM permissions 
		WHERE granted_user_id = ?
	`, userID)

	if err != nil {
		return nil, fmt.Errorf("failed to get user permissions: %w", err)
	}
	defer rows.Close()

	var permissions []models.Permission
	for rows.Next() {
		var p models.Permission
		var expiresAt sql.NullTime

		err := rows.Scan(&p.ID, &p.GrantedUserID, &p.OwnerUserID, &p.ResourceType, &p.PermissionType, &p.CreatedAt, &expiresAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan permission: %w", err)
		}

		if expiresAt.Valid {
			p.ExpiresAt = expiresAt.Time
		}

		permissions = append(permissions, p)
	}

	return permissions, nil
}

// CheckPermission checks if a user has permission to access a resource
func CheckPermission(userID, resourceOwnerID, resourceType, permissionType string) (bool, error) {
	// Always allow users to access their own resources
	if userID == resourceOwnerID {
		return true, nil
	}

	// Check if user is an admin or superadmin
	isUserAdmin, err := IsAdmin(userID)
	if err != nil {
		return false, fmt.Errorf("failed to check admin status: %w", err)
	}

	if isUserAdmin {
		return true, nil
	}

	// Check for explicit permission
	var exists bool
	now := time.Now()

	err = database.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM permissions 
			WHERE granted_user_id = ? 
			AND owner_user_id = ? 
			AND resource_type IN (?, 'all')
			AND permission_type IN (?, 'write', 'all')
			AND (expires_at IS NULL OR expires_at > ?)
		)
	`, userID, resourceOwnerID, resourceType, permissionType, now).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("failed to check permission: %w", err)
	}

	return exists, nil
}

// GetAccessibleResources gets all resource owners that a user has access to for a given resource type
func GetAccessibleResources(userID, resourceType, permissionType string) ([]string, error) {
	// Get user's admin status
	isUserAdmin, err := IsAdmin(userID)
	if err != nil {
		log.Printf("Error checking if user %s is admin: %v", userID, err)
		isUserAdmin = false
	}

	var rows *sql.Rows
	now := time.Now()

	if isUserAdmin {
		// Admins can access all resources
		rows, err = database.DB.Query(`
			SELECT id FROM users
		`)
	} else {
		// Regular users can only access resources they own or have been granted access to
		rows, err = database.DB.Query(`
			SELECT DISTINCT owner_user_id 
			FROM permissions 
			WHERE granted_user_id = ? 
			AND resource_type IN (?, 'all')
			AND permission_type IN (?, 'write', 'all')
			AND (expires_at IS NULL OR expires_at > ?)
			UNION
			SELECT ? as owner_user_id
		`, userID, resourceType, permissionType, now, userID)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query accessible resources: %w", err)
	}
	defer rows.Close()

	var ownerIDs []string
	for rows.Next() {
		var ownerID string
		if err := rows.Scan(&ownerID); err != nil {
			return nil, fmt.Errorf("failed to scan owner ID: %w", err)
		}
		ownerIDs = append(ownerIDs, ownerID)
	}

	return ownerIDs, nil
}
