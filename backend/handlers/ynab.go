package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"bennwallet/backend/database"
	"bennwallet/backend/middleware"
	"bennwallet/backend/models"
	"bennwallet/backend/services"
)

// GetYNABCategories returns YNAB categories for a user in a hierarchical structure
func GetYNABCategories(w http.ResponseWriter, r *http.Request) {
	userId := r.URL.Query().Get("userId")
	if userId == "" {
		http.Error(w, "userId is required", http.StatusBadRequest)
		return
	}

	// First, verify if YNAB tables exist and create them if they don't
	log.Printf("Verifying YNAB tables exist for user %s", userId)

	// Set a context with timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if ynab_category_groups table exists using PostgreSQL information_schema
	var tableCount int
	err := database.DB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM information_schema.tables 
		WHERE table_name = 'ynab_category_groups' 
		AND table_schema = 'public'
	`).Scan(&tableCount)

	// If we hit a timeout or other error, just proceed anyway - worst case tables don't exist
	// and we'll get an empty result
	if err != nil {
		log.Printf("Error checking if YNAB tables exist: %v", err)
		// Return empty array to avoid UI issues
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]struct{}{})
		return
	}

	if tableCount == 0 {
		log.Printf("YNAB tables missing, creating them now")

		// Create YNAB category groups table with timeout context
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		_, err := database.DB.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS ynab_category_groups (
				id TEXT NOT NULL,
				name TEXT NOT NULL,
				user_id TEXT NOT NULL,
				last_updated TIMESTAMP WITH TIME ZONE NOT NULL,
				PRIMARY KEY (id, user_id)
			)
		`)
		if err != nil {
			log.Printf("Error creating ynab_category_groups table: %v", err)
			// Return empty array to avoid UI issues
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]struct{}{})
			return
		}

		// Create YNAB categories table
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err = database.DB.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS ynab_categories (
				id TEXT NOT NULL,
				group_id TEXT NOT NULL,
				name TEXT NOT NULL,
				user_id TEXT NOT NULL,
				last_updated TIMESTAMP WITH TIME ZONE NOT NULL,
				PRIMARY KEY (id, user_id),
				FOREIGN KEY (group_id, user_id) REFERENCES ynab_category_groups(id, user_id)
			)
		`)
		if err != nil {
			log.Printf("Error creating ynab_categories table: %v", err)
			// Return empty array to avoid UI issues
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]struct{}{})
			return
		}

		// Create user YNAB settings table
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err = database.DB.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS user_ynab_settings (
				user_id TEXT PRIMARY KEY,
				token TEXT NOT NULL,
				budget_id TEXT NOT NULL,
				account_id TEXT NOT NULL,
				sync_enabled BOOLEAN NOT NULL DEFAULT FALSE,
				last_synced TIMESTAMP WITH TIME ZONE
			)
		`)
		if err != nil {
			log.Printf("Error creating user_ynab_settings table: %v", err)
			// Return empty array to avoid UI issues
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]struct{}{})
			return
		}

		log.Printf("YNAB tables created successfully")
	}

	// Now check if user has YNAB configured with timeout
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var syncEnabled bool
	err = database.DB.QueryRowContext(ctx, `
		SELECT sync_enabled FROM user_ynab_settings WHERE user_id = $1
	`, userId).Scan(&syncEnabled)

	if err != nil || !syncEnabled {
		// If no YNAB settings or sync disabled, return an empty result
		log.Printf("User %s has no YNAB configuration or sync disabled: %v", userId, err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]struct{}{})
		return
	}

	// Get category groups first with timeout
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	groupRows, err := database.DB.QueryContext(ctx, `
		SELECT id, name 
		FROM ynab_category_groups 
		WHERE user_id = $1 
		ORDER BY name
	`, userId)
	if err != nil {
		log.Printf("Error querying YNAB category groups: %v", err)
		// Return empty array to avoid UI issues
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]struct{}{})
		return
	}
	defer groupRows.Close()

	type CategoryGroup struct {
		ID         string                `json:"id"`
		Name       string                `json:"name"`
		Categories []models.YNABCategory `json:"categories"`
	}

	var groups []CategoryGroup
	for groupRows.Next() {
		var group CategoryGroup
		err := groupRows.Scan(&group.ID, &group.Name)
		if err != nil {
			log.Printf("Error scanning group: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		groups = append(groups, group)
	}

	// Get categories for each group
	for i, group := range groups {
		catRows, err := database.DB.QueryContext(ctx, `
			SELECT id, name
			FROM ynab_categories
			WHERE user_id = $1 AND group_id = $2
			ORDER BY name
		`, userId, group.ID)
		if err != nil {
			log.Printf("Error querying categories for group %s: %v", group.ID, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var categories []models.YNABCategory
		for catRows.Next() {
			var cat models.YNABCategory
			err := catRows.Scan(&cat.ID, &cat.Name)
			if err != nil {
				catRows.Close()
				log.Printf("Error scanning category: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			cat.CategoryGroupID = group.ID
			cat.CategoryGroupName = group.Name
			categories = append(categories, cat)
		}
		catRows.Close()

		groups[i].Categories = categories
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(groups)
}

// SyncYNABTransaction creates a transaction in YNAB based on split data
func SyncYNABTransaction(w http.ResponseWriter, r *http.Request) {
	var request models.YNABSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate request
	if request.UserID == "" {
		http.Error(w, "userId is required", http.StatusBadRequest)
		return
	}

	if len(request.Categories) == 0 {
		http.Error(w, "at least one category split is required", http.StatusBadRequest)
		return
	}

	if request.Date == "" {
		http.Error(w, "date is required", http.StatusBadRequest)
		return
	}

	// Create YNAB transaction via service
	err := models.CreateYNABTransaction(request)
	if err != nil {
		log.Printf("Error creating YNAB transaction: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return success
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Transaction successfully synced to YNAB",
	})
}

// GetYNABConfig handles GET requests for YNAB configuration
func GetYNABConfig(w http.ResponseWriter, r *http.Request) {
	// Get user ID from authentication context
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	log.Printf("Getting YNAB config for user %s", userID)

	// Note: YNAB config table is ensured to exist in ynab_handler.go

	config, err := models.GetYNABConfig(database.DB, userID)
	if err != nil {
		log.Printf("Error retrieving YNAB config: %v", err)
		http.Error(w, "Error retrieving YNAB configuration", http.StatusInternalServerError)
		return
	}

	// Don't return actual token value for security reasons
	// But set placeholder if it exists to indicate to the UI that a token is saved
	if config.HasCredentials {
		config.APIToken = "********" // Placeholder to indicate token exists
	} else {
		config.APIToken = ""
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// UpdateYNABConfig handles PUT requests to update YNAB configuration
func UpdateYNABConfig(w http.ResponseWriter, r *http.Request) {
	// Get user ID from authentication context
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	var request models.YNABConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if request.APIToken == "" || request.BudgetID == "" || request.AccountID == "" {
		http.Error(w, "API token, budget ID, and account ID are required", http.StatusBadRequest)
		return
	}

	// Note: YNAB config table is ensured to exist in ynab_handler.go

	// Update the config
	err := models.UpsertYNABConfig(database.DB, &request, userID)
	if err != nil {
		log.Printf("Error updating YNAB config: %v", err)
		http.Error(w, "Error updating YNAB configuration", http.StatusInternalServerError)
		return
	}

	// Immediately trigger a sync of the YNAB categories
	go func() {
		log.Printf("Triggering initial YNAB category sync for user %s", userID)
		if err := services.SyncYNABCategoriesNew(userID, request.BudgetID); err != nil {
			log.Printf("Error during initial YNAB category sync: %v", err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "YNAB configuration updated successfully. Categories will be synced in the background.",
	})
}

// SyncYNABCategories handles POST requests to sync YNAB categories
func SyncYNABCategories(w http.ResponseWriter, r *http.Request) {
	// Get user ID from authentication context
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	// Get the user's YNAB config
	config, err := models.GetYNABConfig(database.DB, userID)
	if err != nil {
		log.Printf("Error retrieving YNAB config: %v", err)
		http.Error(w, "Error retrieving YNAB configuration", http.StatusInternalServerError)
		return
	}

	if !config.HasCredentials {
		http.Error(w, "YNAB not configured for this user", http.StatusBadRequest)
		return
	}

	// Use the budget ID from the config
	if config.BudgetID == "" {
		http.Error(w, "YNAB budget ID not found", http.StatusBadRequest)
		return
	}

	// Trigger the sync in the background
	go func() {
		if err := services.SyncYNABCategoriesNew(userID, config.BudgetID); err != nil {
			log.Printf("Error syncing YNAB categories: %v", err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "YNAB category sync initiated",
	})
}
