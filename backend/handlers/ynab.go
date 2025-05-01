package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"bennwallet/backend/database"
	"bennwallet/backend/models"
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

	// Check if ynab_category_groups table exists
	var tableCount int
	err := database.DB.QueryRowContext(ctx, `
		SELECT count(*) FROM sqlite_master 
		WHERE type='table' AND name='ynab_category_groups'
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
		// To this:
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		_, err := database.DB.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS ynab_category_groups (
				id TEXT NOT NULL,
				name TEXT NOT NULL,
				user_id TEXT NOT NULL,
				last_updated DATETIME NOT NULL,
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
				last_updated DATETIME NOT NULL,
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
				sync_enabled BOOLEAN NOT NULL DEFAULT 0,
				last_synced DATETIME
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
		SELECT sync_enabled FROM user_ynab_settings WHERE user_id = ?
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
		WHERE user_id = ? 
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
			WHERE user_id = ? AND group_id = ?
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
