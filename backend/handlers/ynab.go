package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"bennwallet/backend/models"
	"bennwallet/backend/security"
)

// YNABHandler handles YNAB-related operations
type YNABHandler struct {
	db *sql.DB
}

// NewYNABHandler creates a new YNAB handler
func NewYNABHandler(db *sql.DB) *YNABHandler {
	return &YNABHandler{
		db: db,
	}
}

// GetYNABConfig returns the current YNAB configuration
func (h *YNABHandler) GetYNABConfig(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(string)

	config, err := models.GetYNABConfig(h.db, userID)
	if err != nil {
		log.Printf("Error getting YNAB config: %v", err)
		http.Error(w, "Failed to get YNAB configuration", http.StatusInternalServerError)
		return
	}

	if config == nil {
		// No configuration found, return empty response
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Return safe version without sensitive data
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config.ToResponse())
}

// UpdateYNABConfig updates the YNAB configuration
func (h *YNABHandler) UpdateYNABConfig(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(string)

	var req struct {
		APIToken      string `json:"api_token"`
		BudgetID      string `json:"budget_id"`
		AccountID     string `json:"account_id"`
		SyncFrequency int    `json:"sync_frequency"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get current config if it exists
	currentConfig, err := models.GetYNABConfig(h.db, userID)
	if err != nil {
		log.Printf("Error getting current YNAB config: %v", err)
		http.Error(w, "Failed to get current YNAB configuration", http.StatusInternalServerError)
		return
	}

	// Create new config if none exists
	if currentConfig == nil {
		currentConfig = &models.YNABConfig{
			UserID: userID,
		}
	}

	// Only update fields that were provided
	if req.APIToken != "" && !isTokenMasked(req.APIToken) {
		encryptedToken, err := security.Encrypt(req.APIToken)
		if err != nil {
			http.Error(w, "Failed to encrypt API token", http.StatusInternalServerError)
			return
		}
		currentConfig.EncryptedAPIToken = encryptedToken
	}

	if req.BudgetID != "" {
		encryptedBudgetID, err := security.Encrypt(req.BudgetID)
		if err != nil {
			http.Error(w, "Failed to encrypt budget ID", http.StatusInternalServerError)
			return
		}
		currentConfig.EncryptedBudgetID = encryptedBudgetID
	}

	if req.AccountID != "" {
		encryptedAccountID, err := security.Encrypt(req.AccountID)
		if err != nil {
			http.Error(w, "Failed to encrypt account ID", http.StatusInternalServerError)
			return
		}
		currentConfig.EncryptedAccountID = encryptedAccountID
	}

	if req.SyncFrequency > 0 {
		currentConfig.SyncFrequency = req.SyncFrequency
	}

	// Save the updated config
	err = models.SaveYNABConfig(h.db, currentConfig)
	if err != nil {
		log.Printf("Error saving YNAB config: %v", err)
		http.Error(w, "Failed to save YNAB configuration", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "Configuration updated"})
}

// SyncYNABCategories manually triggers a sync of YNAB categories
func (h *YNABHandler) SyncYNABCategories(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(string)

	client := NewYNABClient(h.db)
	err := client.SyncCategories(r.Context(), userID)
	if err != nil {
		log.Printf("Error syncing YNAB categories: %v", err)
		http.Error(w, "Failed to sync YNAB categories: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "Categories synced successfully"})
}

// Helper function to check if a token is masked
func isTokenMasked(token string) bool {
	// Check if token starts with asterisks (masked)
	for i := 0; i < len(token) && i < 12; i++ {
		if token[i] != '*' {
			return false
		}
	}
	return true
}

// NewYNABClient creates a new YNAB client
func NewYNABClient(db *sql.DB) *YNABClient {
	return &YNABClient{
		db: db,
	}
}

// YNABClient is a client for interacting with YNAB
type YNABClient struct {
	db *sql.DB
}

// SyncCategories syncs categories from YNAB
func (c *YNABClient) SyncCategories(ctx context.Context, userID string) error {
	// This is a stub - implement the actual sync logic or call into ynab package
	return nil
}
