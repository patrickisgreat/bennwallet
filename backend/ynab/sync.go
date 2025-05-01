package ynab

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"bennwallet/backend/models"
	"bennwallet/backend/security"
)

// YNABClient handles communication with the YNAB API
type YNABClient struct {
	client *http.Client
	db     *sql.DB
}

// NewYNABClient creates a new YNAB client
func NewYNABClient(db *sql.DB) *YNABClient {
	return &YNABClient{
		client: &http.Client{},
		db:     db,
	}
}

// InitYNABSync initializes the YNAB sync system
func InitYNABSync(db *sql.DB) error {
	// Create YNAB config table if it doesn't exist
	_, err := db.Exec(`
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
		return fmt.Errorf("failed to create YNAB config table: %w", err)
	}

	// Check if any users have YNAB configured with all required credentials
	var count int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM ynab_config 
		WHERE encrypted_api_token IS NOT NULL AND encrypted_api_token != ''
		AND encrypted_budget_id IS NOT NULL AND encrypted_budget_id != ''
		AND encrypted_account_id IS NOT NULL AND encrypted_account_id != ''
	`).Scan(&count)

	if err != nil {
		log.Printf("Error checking for configured users in ynab_config: %v", err)
		count = 0
	}

	if count == 0 {
		// Also check legacy table
		err = db.QueryRow(`
			SELECT COUNT(*) FROM user_ynab_settings
			WHERE token IS NOT NULL AND token != ''
			AND budget_id IS NOT NULL AND budget_id != ''
			AND account_id IS NOT NULL AND account_id != ''
			AND sync_enabled = 1
		`).Scan(&count)

		if err != nil {
			log.Printf("Error checking for configured users in user_ynab_settings: %v", err)
			count = 0
		}
	}

	if count == 0 {
		log.Println("No users with YNAB configured, skipping background sync")
		return nil
	}

	// Start background sync for all configured users
	log.Printf("Starting background sync for %d users with YNAB configured", count)
	go startBackgroundSync(db)

	return nil
}

// startBackgroundSync starts the background sync process for all configured users
func startBackgroundSync(db *sql.DB) {
	client := NewYNABClient(db)
	ticker := time.NewTicker(1 * time.Minute) // Check every minute for users to sync

	for range ticker.C {
		// Get all users with complete YNAB config
		rows, err := db.Query(`
			SELECT user_id, sync_frequency, last_sync_time
			FROM ynab_config
			WHERE encrypted_api_token IS NOT NULL AND encrypted_api_token != ''
			AND encrypted_budget_id IS NOT NULL AND encrypted_budget_id != ''
			AND encrypted_account_id IS NOT NULL AND encrypted_account_id != ''
		`)
		if err != nil {
			log.Printf("Error querying YNAB configs: %v", err)
			continue
		}

		users := make(map[string]bool)

		for rows.Next() {
			var userID string
			var syncFrequency int
			var lastSyncTime sql.NullTime

			if err := rows.Scan(&userID, &syncFrequency, &lastSyncTime); err != nil {
				log.Printf("Error scanning YNAB config: %v", err)
				continue
			}

			// Check if it's time to sync
			shouldSync := false
			if !lastSyncTime.Valid {
				// First sync
				shouldSync = true
			} else {
				// Check if enough time has passed since last sync
				nextSync := lastSyncTime.Time.Add(time.Duration(syncFrequency) * time.Minute)
				shouldSync = time.Now().After(nextSync)
			}

			if shouldSync {
				// Perform sync in a goroutine
				go func(userID string) {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
					defer cancel()

					if err := client.SyncCategories(ctx, userID); err != nil {
						log.Printf("Error syncing categories for user %s: %v", userID, err)
					}

					if err := client.SyncTransactions(ctx, userID); err != nil {
						log.Printf("Error syncing transactions for user %s: %v", userID, err)
					}
				}(userID)
			}

			// Keep track of which users we've seen
			users[userID] = true
		}
		rows.Close()

		// Check the legacy table for any users not already synced
		legacyRows, err := db.Query(`
			SELECT user_id, last_synced
			FROM user_ynab_settings
			WHERE token IS NOT NULL AND token != ''
			AND budget_id IS NOT NULL AND budget_id != ''
			AND account_id IS NOT NULL AND account_id != ''
			AND sync_enabled = 1
		`)
		if err != nil {
			log.Printf("Error querying legacy YNAB settings: %v", err)
			continue
		}

		for legacyRows.Next() {
			var userID string
			var lastSynced sql.NullTime

			if err := legacyRows.Scan(&userID, &lastSynced); err != nil {
				log.Printf("Error scanning legacy YNAB settings: %v", err)
				continue
			}

			// Skip users we've already processed
			if users[userID] {
				continue
			}

			// Check if it's time to sync (using default 60 minute frequency)
			shouldSync := false
			if !lastSynced.Valid {
				// First sync
				shouldSync = true
			} else {
				// Check if enough time has passed since last sync (default 60 minutes)
				nextSync := lastSynced.Time.Add(60 * time.Minute)
				shouldSync = time.Now().After(nextSync)
			}

			if shouldSync {
				// Perform sync in a goroutine
				go func(userID string) {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
					defer cancel()

					// Get the budget ID
					var budgetID string
					err := db.QueryRow("SELECT budget_id FROM user_ynab_settings WHERE user_id = ?", userID).Scan(&budgetID)
					if err != nil {
						log.Printf("Error getting budget ID for user %s: %v", userID, err)
						return
					}

					// Sync categories using the services package which works with legacy format
					if err := client.SyncCategories(ctx, userID); err != nil {
						log.Printf("Error syncing categories for legacy user %s: %v", userID, err)
					}
				}(userID)
			}
		}
		legacyRows.Close()
	}
}

// SyncCategories syncs categories from YNAB
func (c *YNABClient) SyncCategories(ctx context.Context, userID string) error {
	config, err := models.GetYNABConfig(c.db, userID)
	if err != nil {
		return fmt.Errorf("failed to get YNAB config: %w", err)
	}
	if config == nil {
		return fmt.Errorf("no YNAB configuration found for user")
	}

	// Get API token and budget ID, either from encrypted fields or legacy format
	var apiToken, budgetID string

	if config.EncryptedAPIToken != "" {
		// Get from encrypted fields
		apiToken, err = security.Decrypt(config.EncryptedAPIToken)
		if err != nil {
			return fmt.Errorf("failed to decrypt API token: %w", err)
		}

		budgetID, err = security.Decrypt(config.EncryptedBudgetID)
		if err != nil {
			return fmt.Errorf("failed to decrypt budget ID: %w", err)
		}
	} else {
		// Try legacy format
		var token string
		err := c.db.QueryRow("SELECT token, budget_id FROM user_ynab_settings WHERE user_id = ?", userID).Scan(&token, &budgetID)
		if err != nil {
			return fmt.Errorf("failed to get YNAB settings from legacy table: %w", err)
		}

		// Handle legacy token format
		if token != "" && token != "[stored in environment variables]" && len(token) > 4 && token[:4] == "enc:" {
			apiToken = token[4:] // Remove "enc:" prefix
		} else {
			return fmt.Errorf("unsupported token format in legacy table")
		}
	}

	// Make API request to YNAB
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("https://api.ynab.com/v1/budgets/%s/categories", budgetID), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("YNAB API returned status %d", resp.StatusCode)
	}

	// Parse response and update categories in database
	// ... (rest of the sync logic)

	// Update last sync time
	if err := models.UpdateLastSyncTime(c.db, userID); err != nil {
		log.Printf("Failed to update last sync time: %v", err)
	}

	return nil
}

// SyncTransactions syncs transactions from YNAB
func (c *YNABClient) SyncTransactions(ctx context.Context, userID string) error {
	config, err := models.GetYNABConfig(c.db, userID)
	if err != nil {
		return fmt.Errorf("failed to get YNAB config: %w", err)
	}
	if config == nil {
		return fmt.Errorf("no YNAB configuration found for user")
	}

	// Get API credentials, either from encrypted fields or legacy format
	var apiToken, budgetID, accountID string

	if config.EncryptedAPIToken != "" {
		// Get from encrypted fields
		apiToken, err = security.Decrypt(config.EncryptedAPIToken)
		if err != nil {
			return fmt.Errorf("failed to decrypt API token: %w", err)
		}

		budgetID, err = security.Decrypt(config.EncryptedBudgetID)
		if err != nil {
			return fmt.Errorf("failed to decrypt budget ID: %w", err)
		}

		accountID, err = security.Decrypt(config.EncryptedAccountID)
		if err != nil {
			return fmt.Errorf("failed to decrypt account ID: %w", err)
		}
	} else {
		// Try legacy format
		var token string
		err := c.db.QueryRow("SELECT token, budget_id, account_id FROM user_ynab_settings WHERE user_id = ?", userID).Scan(&token, &budgetID, &accountID)
		if err != nil {
			return fmt.Errorf("failed to get YNAB settings from legacy table: %w", err)
		}

		// Handle legacy token format
		if token != "" && token != "[stored in environment variables]" && len(token) > 4 && token[:4] == "enc:" {
			apiToken = token[4:] // Remove "enc:" prefix
		} else {
			return fmt.Errorf("unsupported token format in legacy table")
		}
	}

	// Make API request to YNAB
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("https://api.ynab.com/v1/budgets/%s/accounts/%s/transactions", budgetID, accountID), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("YNAB API returned status %d", resp.StatusCode)
	}

	// Parse response and update transactions in database
	// ... (rest of the sync logic)

	// Update last sync time
	if err := models.UpdateLastSyncTime(c.db, userID); err != nil {
		log.Printf("Failed to update last sync time: %v", err)
	}

	return nil
}
