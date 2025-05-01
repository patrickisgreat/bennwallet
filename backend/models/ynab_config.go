package models

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"bennwallet/backend/security"
)

// YNABConfig represents a user's YNAB configuration
type YNABConfig struct {
	ID                 int       `json:"id,omitempty"`
	UserID             string    `json:"userId"`
	EncryptedAPIToken  string    `json:"-"`                   // Not returned in API responses
	EncryptedBudgetID  string    `json:"-"`                   // Not returned in API responses
	EncryptedAccountID string    `json:"-"`                   // Not returned in API responses
	APIToken           string    `json:"apiToken,omitempty"`  // Used only for input/output
	BudgetID           string    `json:"budgetId,omitempty"`  // Used only for input/output
	AccountID          string    `json:"accountId,omitempty"` // Used only for input/output
	LastSyncTime       time.Time `json:"lastSyncTime,omitempty"`
	SyncFrequency      int       `json:"syncFrequency"`
	CreatedAt          time.Time `json:"createdAt,omitempty"`
	UpdatedAt          time.Time `json:"updatedAt,omitempty"`
	HasCredentials     bool      `json:"hasCredentials"`
}

// YNABConfigUpdateRequest represents a request to update YNAB configuration
type YNABConfigUpdateRequest struct {
	APIToken      string `json:"apiToken"`
	BudgetID      string `json:"budgetId"`
	AccountID     string `json:"accountId"`
	SyncFrequency int    `json:"syncFrequency,omitempty"`
}

// GetYNABConfig retrieves a user's YNAB configuration
func GetYNABConfig(db *sql.DB, userID string) (*YNABConfig, error) {
	log.Printf("Getting YNAB config for user %s", userID)

	var config YNABConfig

	// First check the new ynab_config table
	var lastSyncTime sql.NullTime
	query := `
		SELECT id, user_id, encrypted_api_token, encrypted_budget_id, encrypted_account_id, 
		       last_sync_time, sync_frequency, created_at, updated_at
		FROM ynab_config
		WHERE user_id = ?
	`
	log.Printf("Executing query: %s with userID: %s", query, userID)

	err := db.QueryRow(query, userID).Scan(
		&config.ID, &config.UserID, &config.EncryptedAPIToken, &config.EncryptedBudgetID,
		&config.EncryptedAccountID, &lastSyncTime, &config.SyncFrequency,
		&config.CreatedAt, &config.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		log.Printf("No configuration found in ynab_config table for user %s, checking legacy table", userID)
		// Check the legacy user_ynab_settings table
		var token, budgetID, accountID string
		var lastSynced sql.NullTime

		err = db.QueryRow(`
			SELECT token, budget_id, account_id, last_synced
			FROM user_ynab_settings
			WHERE user_id = ?
		`, userID).Scan(&token, &budgetID, &accountID, &lastSynced)

		if err == sql.ErrNoRows {
			// No config found in either table
			log.Printf("No configuration found in legacy table either for user %s, returning default", userID)
			config.UserID = userID
			config.SyncFrequency = 60 // Default to 60 minutes
			config.HasCredentials = false
			return &config, nil
		} else if err != nil {
			log.Printf("Error querying legacy YNAB settings: %v", err)
			return nil, fmt.Errorf("error querying legacy YNAB settings: %w", err)
		}

		// Config found in legacy table
		log.Printf("Found configuration in legacy table for user %s", userID)
		config.UserID = userID

		// Convert legacy format to new format
		if token != "" {
			config.HasCredentials = true
		}

		// In legacy format, we need to handle the token format
		if token != "" && token != "[stored in environment variables]" && len(token) > 4 && token[:4] == "enc:" {
			// Token is stored with "enc:" prefix in local dev
			config.APIToken = token[4:] // Remove "enc:" prefix
		}

		config.BudgetID = budgetID
		config.AccountID = accountID

		if lastSynced.Valid {
			config.LastSyncTime = lastSynced.Time
		}

		config.SyncFrequency = 60 // Default

		return &config, nil
	} else if err != nil {
		log.Printf("Error querying YNAB config: %v", err)
		return nil, fmt.Errorf("error querying YNAB config: %w", err)
	}

	// Found config in new table
	log.Printf("Found configuration in ynab_config table for user %s", userID)
	if lastSyncTime.Valid {
		config.LastSyncTime = lastSyncTime.Time
	}

	config.HasCredentials = config.EncryptedAPIToken != "" &&
		config.EncryptedBudgetID != "" &&
		config.EncryptedAccountID != ""

	log.Printf("User %s has credentials: %v", userID, config.HasCredentials)

	// Decrypt the credentials for display in the API response if they exist
	if config.HasCredentials {
		// Don't include the API token for security (done in handler)
		// But include the budget and account IDs
		if config.EncryptedBudgetID != "" {
			budgetID, err := security.Decrypt(config.EncryptedBudgetID)
			if err != nil {
				log.Printf("Error decrypting budget ID: %v", err)
			} else {
				config.BudgetID = budgetID
				log.Printf("Successfully set BudgetID to: %s", config.BudgetID)
			}
		} else {
			log.Printf("EncryptedBudgetID is empty")
		}

		if config.EncryptedAccountID != "" {
			accountID, err := security.Decrypt(config.EncryptedAccountID)
			if err != nil {
				log.Printf("Error decrypting account ID: %v", err)
			} else {
				config.AccountID = accountID
				log.Printf("Successfully set AccountID to: %s", config.AccountID)
			}
		} else {
			log.Printf("EncryptedAccountID is empty")
		}
	} else {
		log.Printf("User %s doesn't have credentials, not attempting to decrypt", userID)
	}

	return &config, nil
}

// UpsertYNABConfig creates or updates a user's YNAB configuration
func UpsertYNABConfig(db *sql.DB, config *YNABConfigUpdateRequest, userID string) error {
	log.Printf("Upserting YNAB config for user %s", userID)

	// Check if we already have a config for this user
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM ynab_config WHERE user_id = ?", userID).Scan(&count)
	if err != nil {
		return fmt.Errorf("error checking for existing YNAB config: %w", err)
	}

	// Encrypt the credentials
	encryptedToken, err := security.Encrypt(config.APIToken)
	if err != nil {
		return fmt.Errorf("error encrypting API token: %w", err)
	}

	encryptedBudgetID, err := security.Encrypt(config.BudgetID)
	if err != nil {
		return fmt.Errorf("error encrypting budget ID: %w", err)
	}

	encryptedAccountID, err := security.Encrypt(config.AccountID)
	if err != nil {
		return fmt.Errorf("error encrypting account ID: %w", err)
	}

	// Default sync frequency to 60 minutes if not specified
	syncFrequency := config.SyncFrequency
	if syncFrequency <= 0 {
		syncFrequency = 60
	}

	now := time.Now()

	if count > 0 {
		// Update existing config
		_, err = db.Exec(`
			UPDATE ynab_config
			SET encrypted_api_token = ?,
				encrypted_budget_id = ?,
				encrypted_account_id = ?,
				sync_frequency = ?,
				updated_at = ?
			WHERE user_id = ?
		`, encryptedToken, encryptedBudgetID, encryptedAccountID, syncFrequency, now, userID)

		if err != nil {
			return fmt.Errorf("error updating YNAB config: %w", err)
		}
	} else {
		// Insert new config
		_, err = db.Exec(`
			INSERT INTO ynab_config
			(user_id, encrypted_api_token, encrypted_budget_id, encrypted_account_id, 
			 sync_frequency, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, userID, encryptedToken, encryptedBudgetID, encryptedAccountID,
			syncFrequency, now, now)

		if err != nil {
			return fmt.Errorf("error inserting YNAB config: %w", err)
		}
	}

	// Also update the legacy table for backward compatibility
	_, err = db.Exec(`
		INSERT INTO user_ynab_settings
		(user_id, token, budget_id, account_id, sync_enabled)
		VALUES (?, ?, ?, ?, 1)
		ON CONFLICT(user_id) DO UPDATE
		SET token = excluded.token,
			budget_id = excluded.budget_id,
			account_id = excluded.account_id,
			sync_enabled = 1
	`, userID, "enc:"+config.APIToken, config.BudgetID, config.AccountID)

	if err != nil {
		log.Printf("Error updating legacy YNAB settings: %v", err)
		// Don't fail the whole operation if this fails
	}

	return nil
}

// UpdateLastSyncTime updates the last sync time for a user
func UpdateLastSyncTime(db *sql.DB, userID string) error {
	now := time.Now()

	// Update in the new table
	_, err := db.Exec(`
		UPDATE ynab_config
		SET last_sync_time = ?,
			updated_at = ?
		WHERE user_id = ?
	`, now, now, userID)

	if err != nil {
		log.Printf("Error updating last sync time in ynab_config: %v", err)
	}

	// Also update in the legacy table
	_, err = db.Exec(`
		UPDATE user_ynab_settings
		SET last_synced = ?
		WHERE user_id = ?
	`, now, userID)

	if err != nil {
		log.Printf("Error updating last sync time in user_ynab_settings: %v", err)
		return fmt.Errorf("error updating last sync time: %w", err)
	}

	return nil
}
