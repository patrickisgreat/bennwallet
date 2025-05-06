package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"bennwallet/backend/middleware"
	"bennwallet/backend/services"

	"github.com/gorilla/mux"
)

// GetUserPermissions returns all permissions granted to a user
func GetUserPermissions(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	// Check if an admin is requesting permissions for another user
	targetUserID := r.URL.Query().Get("userId")
	if targetUserID != "" && targetUserID != userID {
		// Check if the requesting user is an admin
		isAdmin, err := services.IsAdmin(userID)
		if err != nil {
			http.Error(w, "Failed to check admin status: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if !isAdmin {
			http.Error(w, "Forbidden: Only admins can view other users' permissions", http.StatusForbidden)
			return
		}

		// Admin is requesting permissions for another user
		userID = targetUserID
	}

	// Get the user's permissions
	permissions, err := services.GetUserPermissions(userID)
	if err != nil {
		http.Error(w, "Failed to get user permissions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(permissions)
}

// GrantPermission grants a permission from one user to another
func GrantPermission(w http.ResponseWriter, r *http.Request) {
	// Get the requesting user's ID
	granterID := middleware.GetUserIDFromContext(r)
	if granterID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	// Parse the request body
	var request struct {
		GranteeID      string     `json:"granteeId"`
		ResourceType   string     `json:"resourceType"`
		PermissionType string     `json:"permissionType"`
		ExpiresAt      *time.Time `json:"expiresAt,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate the request
	if request.GranteeID == "" {
		http.Error(w, "granteeId is required", http.StatusBadRequest)
		return
	}

	if request.ResourceType == "" {
		http.Error(w, "resourceType is required", http.StatusBadRequest)
		return
	}

	if request.PermissionType == "" {
		http.Error(w, "permissionType is required", http.StatusBadRequest)
		return
	}

	// Grant the permission
	err := services.GrantPermission(granterID, request.GranteeID, request.ResourceType, request.PermissionType, request.ExpiresAt)
	if err != nil {
		http.Error(w, "Failed to grant permission: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// RevokePermission revokes a permission
func RevokePermission(w http.ResponseWriter, r *http.Request) {
	// Get the requesting user's ID
	revokerID := middleware.GetUserIDFromContext(r)
	if revokerID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	// Parse the request body
	var request struct {
		GranteeID      string `json:"granteeId"`
		OwnerID        string `json:"ownerId"`
		ResourceType   string `json:"resourceType"`
		PermissionType string `json:"permissionType"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate the request
	if request.GranteeID == "" {
		http.Error(w, "granteeId is required", http.StatusBadRequest)
		return
	}

	if request.OwnerID == "" {
		http.Error(w, "ownerId is required", http.StatusBadRequest)
		return
	}

	if request.ResourceType == "" {
		http.Error(w, "resourceType is required", http.StatusBadRequest)
		return
	}

	if request.PermissionType == "" {
		http.Error(w, "permissionType is required", http.StatusBadRequest)
		return
	}

	// Revoke the permission
	err := services.RevokePermission(revokerID, request.GranteeID, request.OwnerID, request.ResourceType, request.PermissionType)
	if err != nil {
		http.Error(w, "Failed to revoke permission: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// SetUserRole sets a user's role
func SetUserRole(w http.ResponseWriter, r *http.Request) {
	// Get the requesting user's ID
	actorID := middleware.GetUserIDFromContext(r)
	if actorID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	// Parse the request body
	var request struct {
		UserID  string `json:"userId"`
		NewRole string `json:"newRole"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate the request
	if request.UserID == "" {
		http.Error(w, "userId is required", http.StatusBadRequest)
		return
	}

	if request.NewRole == "" {
		http.Error(w, "newRole is required", http.StatusBadRequest)
		return
	}

	// Set the user role
	err := services.SetUserRole(actorID, request.UserID, request.NewRole)
	if err != nil {
		http.Error(w, "Failed to set user role: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetUserRole gets a user's role
func GetUserRole(w http.ResponseWriter, r *http.Request) {
	// Get the user ID from the URL parameter
	vars := mux.Vars(r)
	targetUserID := vars["userId"]

	if targetUserID == "" {
		http.Error(w, "userId is required", http.StatusBadRequest)
		return
	}

	// Get the requesting user's ID
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	// If the requesting user is not the target user, check if they're an admin
	if userID != targetUserID {
		isAdmin, err := services.IsAdmin(userID)
		if err != nil {
			http.Error(w, "Failed to check admin status: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if !isAdmin {
			http.Error(w, "Forbidden: Only admins can view other users' roles", http.StatusForbidden)
			return
		}
	}

	// Get the user's role
	role, err := services.GetUserRole(targetUserID)
	if err != nil {
		http.Error(w, "Failed to get user role: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return the role
	response := struct {
		UserID string `json:"userId"`
		Role   string `json:"role"`
	}{
		UserID: targetUserID,
		Role:   role,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
