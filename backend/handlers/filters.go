package handlers

import (
	"encoding/json"
	"net/http"

	"bennwallet/backend/middleware"
	"bennwallet/backend/services"

	"github.com/gorilla/mux"
)

// GetSavedFilters returns all saved filters for the current user
func GetSavedFilters(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	// Get resource type from query parameter
	resourceType := r.URL.Query().Get("resourceType")
	if resourceType == "" {
		http.Error(w, "resourceType query parameter is required", http.StatusBadRequest)
		return
	}

	// Get the user's saved filters for this resource type
	filters, err := services.GetSavedFilters(userID, resourceType)
	if err != nil {
		http.Error(w, "Failed to get saved filters: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filters)
}

// GetSavedFilter returns a specific saved filter
func GetSavedFilter(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	// Get filter ID from URL parameter
	vars := mux.Vars(r)
	filterID := vars["id"]
	if filterID == "" {
		http.Error(w, "Filter ID is required", http.StatusBadRequest)
		return
	}

	// Get the filter
	filter, err := services.GetSavedFilterByID(filterID)
	if err != nil {
		http.Error(w, "Failed to get saved filter: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Check if the user owns the filter or is an admin
	if filter.UserID != userID {
		isAdmin, err := services.IsAdmin(userID)
		if err != nil {
			http.Error(w, "Failed to check admin status: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if !isAdmin {
			http.Error(w, "Forbidden: You do not have permission to access this filter", http.StatusForbidden)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filter)
}

// CreateSavedFilter creates a new saved filter
func CreateSavedFilter(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	// Parse the request body
	var request struct {
		Name         string `json:"name"`
		ResourceType string `json:"resourceType"`
		FilterConfig string `json:"filterConfig"`
		IsDefault    bool   `json:"isDefault"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate the request
	if request.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	if request.ResourceType == "" {
		http.Error(w, "resourceType is required", http.StatusBadRequest)
		return
	}

	if request.FilterConfig == "" {
		http.Error(w, "filterConfig is required", http.StatusBadRequest)
		return
	}

	// Create the saved filter
	filter, err := services.CreateSavedFilter(userID, request.Name, request.ResourceType, request.FilterConfig, request.IsDefault)
	if err != nil {
		http.Error(w, "Failed to create saved filter: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(filter)
}

// UpdateSavedFilter updates an existing saved filter
func UpdateSavedFilter(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	// Get filter ID from URL parameter
	vars := mux.Vars(r)
	filterID := vars["id"]
	if filterID == "" {
		http.Error(w, "Filter ID is required", http.StatusBadRequest)
		return
	}

	// Check if the user owns the filter
	filter, err := services.GetSavedFilterByID(filterID)
	if err != nil {
		http.Error(w, "Failed to get saved filter: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if filter.UserID != userID {
		isAdmin, err := services.IsAdmin(userID)
		if err != nil {
			http.Error(w, "Failed to check admin status: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if !isAdmin {
			http.Error(w, "Forbidden: You do not have permission to update this filter", http.StatusForbidden)
			return
		}
	}

	// Parse the request body
	var request struct {
		Name         string `json:"name"`
		FilterConfig string `json:"filterConfig"`
		IsDefault    bool   `json:"isDefault"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate the request
	if request.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	if request.FilterConfig == "" {
		http.Error(w, "filterConfig is required", http.StatusBadRequest)
		return
	}

	// Update the saved filter
	updatedFilter, err := services.UpdateSavedFilter(filterID, request.Name, request.FilterConfig, request.IsDefault)
	if err != nil {
		http.Error(w, "Failed to update saved filter: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedFilter)
}

// DeleteSavedFilter deletes a saved filter
func DeleteSavedFilter(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	// Get filter ID from URL parameter
	vars := mux.Vars(r)
	filterID := vars["id"]
	if filterID == "" {
		http.Error(w, "Filter ID is required", http.StatusBadRequest)
		return
	}

	// Check if the user owns the filter
	filter, err := services.GetSavedFilterByID(filterID)
	if err != nil {
		http.Error(w, "Failed to get saved filter: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if filter.UserID != userID {
		isAdmin, err := services.IsAdmin(userID)
		if err != nil {
			http.Error(w, "Failed to check admin status: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if !isAdmin {
			http.Error(w, "Forbidden: You do not have permission to delete this filter", http.StatusForbidden)
			return
		}
	}

	// Delete the saved filter
	err = services.DeleteSavedFilter(filterID)
	if err != nil {
		http.Error(w, "Failed to delete saved filter: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
