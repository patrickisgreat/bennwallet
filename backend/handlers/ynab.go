package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"bennwallet/backend/models"
	"bennwallet/backend/ynab"
)

// GetYNABConfig returns the current YNAB configuration
func GetYNABConfig(w http.ResponseWriter, r *http.Request) {
	config, err := ynab.GetYNABConfig()
	if err != nil {
		log.Printf("Error getting YNAB config: %v", err)
		http.Error(w, "Failed to get YNAB configuration", http.StatusInternalServerError)
		return
	}

	// Don't expose the full API token
	if config.ApiToken != "" {
		// Mask the API token for security
		config.ApiToken = "************" + config.ApiToken[len(config.ApiToken)-4:]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// UpdateYNABConfig updates the YNAB configuration
func UpdateYNABConfig(w http.ResponseWriter, r *http.Request) {
	var config models.YNABConfig
	err := json.NewDecoder(r.Body).Decode(&config)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get current config
	currentConfig, err := ynab.GetYNABConfig()
	if err != nil {
		log.Printf("Error getting current YNAB config: %v", err)
		http.Error(w, "Failed to get current YNAB configuration", http.StatusInternalServerError)
		return
	}

	// Only update API token if a new one is provided
	// and it's not the masked version
	if config.ApiToken != "" && !isTokenMasked(config.ApiToken) {
		currentConfig.ApiToken = config.ApiToken
	}

	// Update other fields
	if config.BudgetID != "" {
		currentConfig.BudgetID = config.BudgetID
	}

	if config.SyncFrequency > 0 {
		currentConfig.SyncFrequency = config.SyncFrequency
	}

	// Save the updated config
	err = ynab.SaveYNABConfig(currentConfig)
	if err != nil {
		log.Printf("Error saving YNAB config: %v", err)
		http.Error(w, "Failed to save YNAB configuration", http.StatusInternalServerError)
		return
	}

	// Restart background sync with new settings
	ynab.StartBackgroundSync()

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "Configuration updated"})
}

// SyncYNABCategories manually triggers a sync of YNAB categories
func SyncYNABCategories(w http.ResponseWriter, r *http.Request) {
	err := ynab.SyncCategories()
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
