package ynab

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"bennwallet/backend/database"
	"bennwallet/backend/models"
)

const (
	ynabBaseURL  = "https://api.ynab.com/v1"
	configTable  = "ynab_config"
	syncInterval = 60 // 60 minutes default
)

var (
	syncMutex  sync.Mutex
	syncTicker *time.Ticker
	stopChan   chan struct{}
)

// InitYNABSync initializes YNAB sync and starts background sync
func InitYNABSync() error {
	// Create config table if it doesn't exist
	_, err := database.DB.Exec(`
		CREATE TABLE IF NOT EXISTS ynab_config (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			api_token TEXT,
			budget_id TEXT,
			last_sync_time TIMESTAMP,
			sync_frequency INTEGER DEFAULT 60
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create YNAB config table: %w", err)
	}

	// Initialize the config with a single row if it doesn't exist
	var count int
	err = database.DB.QueryRow("SELECT COUNT(*) FROM ynab_config").Scan(&count)
	if err != nil {
		return fmt.Errorf("error checking config table: %w", err)
	}

	if count == 0 {
		// Get API token from environment variable with fallback to empty string
		apiToken := os.Getenv("YNAB_API_TOKEN")

		_, err = database.DB.Exec(`
			INSERT INTO ynab_config (id, api_token, budget_id, sync_frequency) 
			VALUES (1, ?, '', ?)`,
			apiToken, syncInterval)
		if err != nil {
			return fmt.Errorf("failed to initialize YNAB config: %w", err)
		}
	}

	// Start background sync if we have a token
	config, err := GetYNABConfig()
	if err != nil {
		return fmt.Errorf("failed to get YNAB config: %w", err)
	}

	if config.ApiToken != "" {
		StartBackgroundSync()
	}

	return nil
}

// GetYNABConfig retrieves the YNAB configuration
func GetYNABConfig() (*models.YNABConfig, error) {
	var config models.YNABConfig
	var lastSyncTime sql.NullTime

	err := database.DB.QueryRow(`
		SELECT api_token, budget_id, last_sync_time, sync_frequency
		FROM ynab_config WHERE id = 1
	`).Scan(&config.ApiToken, &config.BudgetID, &lastSyncTime, &config.SyncFrequency)

	if err != nil {
		return nil, fmt.Errorf("error retrieving YNAB config: %w", err)
	}

	if lastSyncTime.Valid {
		config.LastSyncTime = lastSyncTime.Time
	}

	return &config, nil
}

// SaveYNABConfig saves the YNAB configuration
func SaveYNABConfig(config *models.YNABConfig) error {
	_, err := database.DB.Exec(`
		UPDATE ynab_config
		SET api_token = ?, budget_id = ?, last_sync_time = ?, sync_frequency = ?
		WHERE id = 1
	`, config.ApiToken, config.BudgetID, config.LastSyncTime, config.SyncFrequency)

	if err != nil {
		return fmt.Errorf("error saving YNAB config: %w", err)
	}

	return nil
}

// StartBackgroundSync begins the background category syncing process
func StartBackgroundSync() {
	syncMutex.Lock()
	defer syncMutex.Unlock()

	// Stop existing sync if running
	if syncTicker != nil {
		stopChan <- struct{}{}
		syncTicker.Stop()
	}

	config, err := GetYNABConfig()
	if err != nil {
		log.Printf("Error getting YNAB config for sync: %v", err)
		return
	}

	if config.ApiToken == "" || config.BudgetID == "" {
		log.Println("YNAB sync not configured (missing API token or budget ID)")
		return
	}

	// Use configured frequency or default
	frequency := config.SyncFrequency
	if frequency <= 0 {
		frequency = syncInterval
	}

	syncTicker = time.NewTicker(time.Duration(frequency) * time.Minute)
	stopChan = make(chan struct{})

	go func() {
		// Perform initial sync immediately
		if err := SyncCategories(); err != nil {
			log.Printf("Initial category sync failed: %v", err)
		}

		for {
			select {
			case <-syncTicker.C:
				if err := SyncCategories(); err != nil {
					log.Printf("Category sync failed: %v", err)
				}
			case <-stopChan:
				return
			}
		}
	}()

	log.Printf("Background YNAB sync started with %d minute interval", frequency)
}

// StopBackgroundSync stops the background sync
func StopBackgroundSync() {
	syncMutex.Lock()
	defer syncMutex.Unlock()

	if syncTicker != nil {
		stopChan <- struct{}{}
		syncTicker.Stop()
		syncTicker = nil
		log.Println("Background YNAB sync stopped")
	}
}

// SyncCategories syncs categories from YNAB to the local database
func SyncCategories() error {
	config, err := GetYNABConfig()
	if err != nil {
		return fmt.Errorf("error getting YNAB config: %w", err)
	}

	if config.ApiToken == "" {
		return fmt.Errorf("YNAB API token not configured")
	}

	if config.BudgetID == "" {
		return fmt.Errorf("YNAB budget ID not configured")
	}

	// Build request to YNAB API
	url := fmt.Sprintf("%s/budgets/%s/categories", ynabBaseURL, config.BudgetID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+config.ApiToken)

	// Make the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error making request to YNAB API: %w", err)
	}
	defer resp.Body.Close()

	// Check for authentication errors
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("YNAB API unauthorized (401) - check your API token")
	}

	// Check for other errors
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("YNAB API error: %s - %s", resp.Status, body)
	}

	// Read and parse the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}

	// Parse the YNAB response
	var response struct {
		Data struct {
			CategoryGroups []struct {
				ID         string `json:"id"`
				Name       string `json:"name"`
				Hidden     bool   `json:"hidden"`
				Deleted    bool   `json:"deleted"`
				Categories []struct {
					ID      string `json:"id"`
					Name    string `json:"name"`
					Hidden  bool   `json:"hidden"`
					Deleted bool   `json:"deleted"`
				} `json:"categories"`
			} `json:"category_groups"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("error parsing YNAB response: %w", err)
	}

	// Begin transaction
	tx, err := database.DB.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Process each category group and its categories
	for _, group := range response.Data.CategoryGroups {
		// Skip hidden and deleted groups
		if group.Hidden || group.Deleted {
			continue
		}

		for _, cat := range group.Categories {
			// Skip hidden and deleted categories
			if cat.Hidden || cat.Deleted {
				continue
			}

			// Check if category exists first
			var exists bool
			err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM categories WHERE name = ? AND user_id = ?)",
				cat.Name, "ynab").Scan(&exists)
			if err != nil {
				return fmt.Errorf("error checking if category exists: %w", err)
			}

			if !exists {
				// Insert new category
				_, err = tx.Exec(`
					INSERT INTO categories (name, description, user_id, color)
					VALUES (?, ?, ?, ?)
				`, cat.Name, "Imported from YNAB: "+group.Name, "ynab", generateRandomColor())

				if err != nil {
					return fmt.Errorf("error inserting YNAB category: %w", err)
				}
			}
		}
	}

	// Update last sync time
	now := time.Now()
	config.LastSyncTime = now

	_, err = tx.Exec("UPDATE ynab_config SET last_sync_time = ? WHERE id = 1", now)
	if err != nil {
		return fmt.Errorf("error updating last sync time: %w", err)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}

	log.Println("YNAB categories synced successfully")
	return nil
}

// Helper function to generate a random color for categories
func generateRandomColor() string {
	colors := []string{
		"#FF6B6B", "#4ECDC4", "#45B7D1", "#96CEB4", "#FFEEAD",
		"#D4A5A5", "#9B59B6", "#3498DB", "#1ABC9C", "#F1C40F",
	}
	return colors[time.Now().UnixNano()%int64(len(colors))]
}
