package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"bennwallet/backend/middleware"
	"bennwallet/backend/models"
	"bennwallet/backend/services"
)

// YNABHandler handles YNAB API requests
type YNABHandler struct {
	db *sql.DB
}

// NewYNABHandler creates a new YNAB handler
func NewYNABHandler(db *sql.DB) *YNABHandler {
	return &YNABHandler{db: db}
}

// GetYNABConfig handles GET /ynab/config
func (h *YNABHandler) GetYNABConfig(w http.ResponseWriter, r *http.Request) {
	userID := getUserIDFromContext(r)
	if userID == "" {
		log.Printf("ERROR: User ID not found in context for request: %s %s", r.Method, r.URL.Path)
		http.Error(w, "User ID not found in context", http.StatusUnauthorized)
		return
	}

	log.Printf("Getting YNAB config for user %s from path: %s", userID, r.URL.Path)

	// Ensure YNAB config table exists
	ensureYNABConfigTable(h.db)

	config, err := models.GetYNABConfig(h.db, userID)
	if err != nil {
		log.Printf("Error retrieving YNAB config: %v", err)
		http.Error(w, "Error retrieving YNAB configuration", http.StatusInternalServerError)
		return
	}

	// Don't return actual token value for security reasons
	// But set placeholder if it exists to indicate to the UI that a token is saved
	if config.HasCredentials {
		config.APIToken = "********" // Placeholder to indicate token exists
		log.Printf("Setting API token placeholder for user %s", userID)
	} else {
		config.APIToken = ""
	}

	log.Printf("Retrieved YNAB config for user %s: HasCredentials=%v, LastSyncTime=%v, BudgetID=%v, AccountID=%v, APIToken=%v",
		userID, config.HasCredentials, config.LastSyncTime, config.BudgetID, config.AccountID, config.APIToken)

	// Marshal to JSON and then log it to debug potential issues
	jsonBytes, err := json.Marshal(config)
	if err != nil {
		log.Printf("Error marshaling config to JSON: %v", err)
	} else {
		log.Printf("Full JSON response: %s", string(jsonBytes))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// UpdateYNABConfig handles POST/PUT /ynab/config
func (h *YNABHandler) UpdateYNABConfig(w http.ResponseWriter, r *http.Request) {
	userID := getUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "User ID not found in context", http.StatusUnauthorized)
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

	// Ensure YNAB config table exists
	ensureYNABConfigTable(h.db)

	// Update the config
	err := models.UpsertYNABConfig(h.db, &request, userID)
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

// SyncYNABCategories handles POST /ynab/sync-categories
func (h *YNABHandler) SyncYNABCategories(w http.ResponseWriter, r *http.Request) {
	userID := getUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "User ID not found in context", http.StatusUnauthorized)
		return
	}

	// Get the user's YNAB config
	config, err := models.GetYNABConfig(h.db, userID)
	if err != nil {
		log.Printf("Error retrieving YNAB config: %v", err)
		http.Error(w, "Error retrieving YNAB configuration", http.StatusInternalServerError)
		return
	}

	if !config.HasCredentials {
		http.Error(w, "YNAB not configured for this user", http.StatusBadRequest)
		return
	}

	// Get the budget ID from the config or legacy table
	var budgetID string
	if config.BudgetID != "" {
		budgetID = config.BudgetID
	} else {
		// Try to get from legacy table
		err := h.db.QueryRow("SELECT budget_id FROM user_ynab_settings WHERE user_id = ?", userID).Scan(&budgetID)
		if err != nil {
			log.Printf("Error retrieving budget ID: %v", err)
			http.Error(w, "YNAB budget ID not found", http.StatusBadRequest)
			return
		}
	}

	// Trigger the sync in the background
	go func() {
		if err := services.SyncYNABCategories(userID, budgetID); err != nil {
			log.Printf("Error syncing YNAB categories: %v", err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "YNAB category sync initiated",
	})
}

// getUserIDFromContext extracts the user ID from the request context
func getUserIDFromContext(r *http.Request) string {
	return middleware.GetUserIDFromContext(r)
}

// ensureYNABConfigTable ensures the YNAB config table exists
func ensureYNABConfigTable(db *sql.DB) error {
	// Create YNAB config table if it doesn't exist
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS ynab_config (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			encrypted_api_token TEXT,
			encrypted_budget_id TEXT,
			encrypted_account_id TEXT,
			last_sync_time TIMESTAMP,
			sync_frequency INTEGER DEFAULT 60,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(user_id)
		);
	`)
	if err != nil {
		log.Printf("Error creating YNAB config table: %v", err)
		return fmt.Errorf("failed to create YNAB config table: %w", err)
	}

	return nil
}
