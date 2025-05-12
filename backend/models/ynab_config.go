package models

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
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
	APIToken           string    `json:"apiToken"`            // Used only for input/output
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

	// First check if the tables exist
	var tablesExist bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'ynab_config'
		)
	`).Scan(&tablesExist)

	if err != nil {
		log.Printf("Error checking if ynab_config table exists: %v", err)
	} else if !tablesExist {
		log.Printf("WARNING: ynab_config table does not exist!")

		// Try to create the tables
		ensureYNABConfigTables(db)
	} else {
		log.Printf("ynab_config table exists, checking columns...")

		// Check if columns exist
		var columnsExist bool
		err := db.QueryRow(`
			SELECT EXISTS (
				SELECT column_name 
				FROM information_schema.columns 
				WHERE table_schema = 'public' 
				AND table_name = 'ynab_config'
				AND column_name = 'encrypted_api_token'
			)
		`).Scan(&columnsExist)

		if err != nil {
			log.Printf("Error checking if encrypted_api_token column exists: %v", err)
		} else if !columnsExist {
			log.Printf("WARNING: encrypted_api_token column does not exist in ynab_config table!")

			// Try to create the column
			_, err := db.Exec(`
				ALTER TABLE ynab_config 
				ADD COLUMN IF NOT EXISTS encrypted_api_token TEXT,
				ADD COLUMN IF NOT EXISTS encrypted_budget_id TEXT,
				ADD COLUMN IF NOT EXISTS encrypted_account_id TEXT,
				ADD COLUMN IF NOT EXISTS last_sync_time TIMESTAMP,
				ADD COLUMN IF NOT EXISTS sync_frequency INTEGER DEFAULT 60
			`)

			if err != nil {
				log.Printf("Error adding missing columns to ynab_config table: %v", err)
			} else {
				log.Printf("Successfully added missing columns to ynab_config table")
			}
		}
	}

	var config YNABConfig

	// First check the new ynab_config table
	var lastSyncTime sql.NullTime
	query := `
		SELECT id, user_id, api_token, budget_id, account_id
		FROM ynab_config
		WHERE user_id = $1
	`
	log.Printf("Executing query: %s with userID: %s", query, userID)

	// We only have a subset of columns in the actual table
	err = db.QueryRow(query, userID).Scan(
		&config.ID,
		&config.UserID,
		&config.EncryptedAPIToken,
		&config.EncryptedBudgetID,
		&config.EncryptedAccountID,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("No YNAB config found for user %s in new table, checking legacy table", userID)
		} else {
			log.Printf("Error querying YNAB config table: %v", err)
			// Continue to check legacy table regardless
		}

		// Check the legacy user_ynab_settings table
		var token, budgetID, accountID string
		var lastSynced sql.NullTime

		err = db.QueryRow(`
			SELECT token, budget_id, account_id, last_synced 
			FROM user_ynab_settings 
			WHERE user_id = $1
		`, userID).Scan(&token, &budgetID, &accountID, &lastSynced)

		if err != nil {
			if err == sql.ErrNoRows {
				log.Printf("No YNAB settings found for user %s in legacy table", userID)
			} else {
				log.Printf("Error querying legacy YNAB settings table: %v", err)
			}

			// Return defaults instead of an error if tables are inconsistent
			return &YNABConfig{
				UserID:         userID,
				HasCredentials: false,
				SyncFrequency:  24, // Default 24 hours
			}, nil
		}

		log.Printf("Found YNAB settings in legacy table for user %s", userID)

		// Use legacy settings
		config.UserID = userID
		config.HasCredentials = token != ""
		config.BudgetID = budgetID
		config.AccountID = accountID
		if lastSynced.Valid {
			config.LastSyncTime = lastSynced.Time
		}

		// Special handling for stored token
		if token != "" {
			if strings.HasPrefix(token, "enc:") {
				// For local dev, token is prefixed in DB
				log.Printf("Token is already encrypted with 'enc:' prefix")
				config.EncryptedAPIToken = strings.TrimPrefix(token, "enc:")
			} else {
				log.Printf("Found unencrypted token in legacy table")
				config.APIToken = token
			}
		}

		return &config, nil
	}

	// If we get here, we found a record in the new table
	log.Printf("Found YNAB config in new table for user %s", userID)

	// Set the last sync time if it's valid
	if lastSyncTime.Valid {
		config.LastSyncTime = lastSyncTime.Time
	}

	// Set HasCredentials flag based on whether there's an API token
	config.HasCredentials = config.EncryptedAPIToken != ""

	// If API token exists, try to decrypt it for testing but don't return it
	if config.EncryptedAPIToken != "" {
		_, err = security.Decrypt(config.EncryptedAPIToken)
		if err != nil {
			log.Printf("Warning: Could not decrypt API token: %v", err)
			// Continue anyway as we don't return the actual token
		} else {
			log.Printf("Successfully decrypted API token for validation")
		}
	}

	// Also try to decrypt the budget and account IDs
	if config.EncryptedBudgetID != "" {
		decryptedBudgetID, err := security.Decrypt(config.EncryptedBudgetID)
		if err != nil {
			log.Printf("Warning: Could not decrypt budget ID: %v", err)
		} else {
			config.BudgetID = decryptedBudgetID
		}
	}

	if config.EncryptedAccountID != "" {
		decryptedAccountID, err := security.Decrypt(config.EncryptedAccountID)
		if err != nil {
			log.Printf("Warning: Could not decrypt account ID: %v", err)
		} else {
			config.AccountID = decryptedAccountID
		}
	}

	return &config, nil
}

// ensureYNABConfigTables creates the YNAB config tables if they don't exist
func ensureYNABConfigTables(db *sql.DB) error {
	log.Println("Creating YNAB config tables...")

	// Create the new ynab_config table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS ynab_config (
			id SERIAL PRIMARY KEY,
			user_id TEXT NOT NULL,
			encrypted_api_token TEXT,
			encrypted_budget_id TEXT,
			encrypted_account_id TEXT,
			api_token TEXT,
			budget_id TEXT,
			account_id TEXT,
			last_sync_time TIMESTAMP,
			sync_frequency INTEGER DEFAULT 60,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			has_credentials BOOLEAN
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating ynab_config table: %w", err)
	}

	// Create the legacy user_ynab_settings table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_ynab_settings (
			user_id TEXT PRIMARY KEY,
			token TEXT,
			budget_id TEXT,
			account_id TEXT,
			last_synced TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating user_ynab_settings table: %w", err)
	}

	return nil
}

// UpdateLastSyncTime updates the last sync time for a user
func UpdateLastSyncTime(db *sql.DB, userID string) error {
	now := time.Now()

	// Update in the new table
	_, err := db.Exec(`
		UPDATE ynab_config
		SET last_sync_time = $1,
			updated_at = $2
		WHERE user_id = $3
	`, now, now, userID)

	if err != nil {
		log.Printf("Error updating last sync time in ynab_config: %v", err)
	}

	// Also update in the legacy table
	_, err = db.Exec(`
		UPDATE user_ynab_settings
		SET last_synced = $1
		WHERE user_id = $2
	`, now, userID)

	if err != nil {
		log.Printf("Error updating last sync time in user_ynab_settings: %v", err)
		return fmt.Errorf("error updating last sync time: %w", err)
	}

	return nil
}

// FixYNABTableSchema checks and updates the ynab_config table schema if needed
func FixYNABTableSchema(db *sql.DB) error {
	log.Println("Checking and fixing YNAB config table schema...")

	// First, check if the table exists
	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'ynab_config'
		)
	`).Scan(&exists)

	if err != nil {
		return fmt.Errorf("error checking if ynab_config table exists: %w", err)
	}

	if !exists {
		log.Println("YNAB config table doesn't exist, nothing to fix")
		return nil
	}

	// Get the current column names
	rows, err := db.Query(`
		SELECT column_name 
		FROM information_schema.columns 
		WHERE table_schema = 'public' 
		AND table_name = 'ynab_config'
	`)
	if err != nil {
		return fmt.Errorf("error getting column names: %w", err)
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var colName string
		if err := rows.Scan(&colName); err != nil {
			return fmt.Errorf("error scanning column name: %w", err)
		}
		columns[colName] = true
	}

	log.Printf("Current columns in ynab_config: %v", columns)

	// If api_token exists but encrypted_api_token doesn't, we need to
	// rename and potentially convert data
	if columns["api_token"] && !columns["encrypted_api_token"] {
		log.Println("Found api_token column but not encrypted_api_token, fixing...")

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("error starting transaction: %w", err)
		}

		// Add encrypted columns
		_, err = tx.Exec(`
			ALTER TABLE ynab_config 
			ADD COLUMN encrypted_api_token TEXT,
			ADD COLUMN encrypted_budget_id TEXT,
			ADD COLUMN encrypted_account_id TEXT
		`)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("error adding encrypted columns: %w", err)
		}

		// Get all configs
		rows, err := tx.Query(`SELECT user_id, api_token, budget_id, account_id FROM ynab_config`)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("error querying configs: %w", err)
		}

		// Update each config with encrypted values
		for rows.Next() {
			var userID, apiToken, budgetID, accountID string
			if err := rows.Scan(&userID, &apiToken, &budgetID, &accountID); err != nil {
				rows.Close()
				tx.Rollback()
				return fmt.Errorf("error scanning config: %w", err)
			}

			encryptedToken, err := security.Encrypt(apiToken)
			if err != nil {
				rows.Close()
				tx.Rollback()
				return fmt.Errorf("error encrypting token: %w", err)
			}

			encryptedBudgetID, err := security.Encrypt(budgetID)
			if err != nil {
				rows.Close()
				tx.Rollback()
				return fmt.Errorf("error encrypting budget ID: %w", err)
			}

			encryptedAccountID, err := security.Encrypt(accountID)
			if err != nil {
				rows.Close()
				tx.Rollback()
				return fmt.Errorf("error encrypting account ID: %w", err)
			}

			_, err = tx.Exec(`
				UPDATE ynab_config
				SET encrypted_api_token = $1,
					encrypted_budget_id = $2,
					encrypted_account_id = $3
				WHERE user_id = $4
			`, encryptedToken, encryptedBudgetID, encryptedAccountID, userID)

			if err != nil {
				rows.Close()
				tx.Rollback()
				return fmt.Errorf("error updating encrypted values: %w", err)
			}
		}
		rows.Close()

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("error committing transaction: %w", err)
		}

		log.Println("Successfully updated ynab_config table schema")
	}

	// Ensure other columns exist
	if !columns["last_sync_time"] {
		_, err := db.Exec(`ALTER TABLE ynab_config ADD COLUMN last_sync_time TIMESTAMP`)
		if err != nil {
			log.Printf("Error adding last_sync_time column: %v", err)
		}
	}

	if !columns["sync_frequency"] {
		_, err := db.Exec(`ALTER TABLE ynab_config ADD COLUMN sync_frequency INTEGER DEFAULT 60`)
		if err != nil {
			log.Printf("Error adding sync_frequency column: %v", err)
		}
	}

	if !columns["created_at"] {
		_, err := db.Exec(`ALTER TABLE ynab_config ADD COLUMN created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP`)
		if err != nil {
			log.Printf("Error adding created_at column: %v", err)
		}
	}

	if !columns["updated_at"] {
		_, err := db.Exec(`ALTER TABLE ynab_config ADD COLUMN updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP`)
		if err != nil {
			log.Printf("Error adding updated_at column: %v", err)
		}
	}

	return nil
}

// UpsertYNABConfig creates or updates a user's YNAB configuration
func UpsertYNABConfig(db *sql.DB, config *YNABConfigUpdateRequest, userID string) error {
	log.Printf("Upserting YNAB config for user %s", userID)

	// First ensure the user exists in the users table
	var userExists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", userID).Scan(&userExists)
	if err != nil {
		log.Printf("Error checking if user exists: %v", err)
		// Continue anyway - the constraint will catch this if needed
	} else if !userExists {
		log.Printf("User %s does not exist in users table, creating dummy user", userID)

		// Create a dummy user to satisfy the foreign key constraint
		_, err = db.Exec(`
			INSERT INTO users (id, username, name, role) 
			VALUES ($1, $2, $3, 'user')
			ON CONFLICT (id) DO NOTHING
		`, userID, fmt.Sprintf("user_%s", userID), fmt.Sprintf("User %s", userID))

		if err != nil {
			log.Printf("Error creating dummy user: %v", err)
			return fmt.Errorf("error creating user entry: %w", err)
		}
	}

	// Ensure the tables exist first
	var tablesExist bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'ynab_config'
		)
	`).Scan(&tablesExist)

	if err != nil {
		log.Printf("Error checking if ynab_config table exists: %v", err)
	} else if !tablesExist {
		log.Printf("ynab_config table doesn't exist, creating it now")
		if err := ensureYNABConfigTables(db); err != nil {
			return fmt.Errorf("error creating YNAB config tables: %w", err)
		}
	}

	// Check if we already have a config for this user
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM ynab_config WHERE user_id = $1", userID).Scan(&count)
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

	// Check if the table has the needed columns
	var columnsMap map[string]bool
	columnsMap, err = getTableColumns(db, "ynab_config")
	if err != nil {
		log.Printf("Error getting ynab_config columns: %v", err)
		columnsMap = map[string]bool{"api_token": true, "budget_id": true, "account_id": true} // Default assumption
	}

	log.Printf("Columns in ynab_config table: %v", columnsMap)

	if count > 0 {
		// Update existing config - adjust the query based on available columns
		var query string
		var args []interface{}

		// Always update both column sets to ensure compatibility
		query = `
			UPDATE ynab_config
			SET encrypted_api_token = $1,
				encrypted_budget_id = $2,
				encrypted_account_id = $3,
				api_token = $4,
				budget_id = $5,
				account_id = $6
			WHERE user_id = $7
		`
		args = []interface{}{
			encryptedToken, encryptedBudgetID, encryptedAccountID, // Encrypted columns
			encryptedToken, encryptedBudgetID, encryptedAccountID, // Regular columns (using encrypted values)
			userID,
		}

		_, err = db.Exec(query, args...)
		if err != nil {
			return fmt.Errorf("error updating YNAB config: %w", err)
		}
	} else {
		// Insert new config - adjust the query based on available columns
		var query string
		var args []interface{}

		// Always insert to both column sets to ensure compatibility
		query = `
			INSERT INTO ynab_config
			(user_id, encrypted_api_token, encrypted_budget_id, encrypted_account_id, 
			 api_token, budget_id, account_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`
		args = []interface{}{
			userID,
			encryptedToken, encryptedBudgetID, encryptedAccountID, // Encrypted columns
			encryptedToken, encryptedBudgetID, encryptedAccountID, // Regular columns (using encrypted values)
		}

		_, err = db.Exec(query, args...)
		if err != nil {
			return fmt.Errorf("error inserting YNAB config: %w", err)
		}
	}

	return nil
}

// getTableColumns gets a map of column names in a table
func getTableColumns(db *sql.DB, tableName string) (map[string]bool, error) {
	columns := make(map[string]bool)

	rows, err := db.Query(`
		SELECT column_name 
		FROM information_schema.columns 
		WHERE table_schema = 'public' 
		AND table_name = $1
	`, tableName)

	if err != nil {
		return columns, err
	}
	defer rows.Close()

	for rows.Next() {
		var colName string
		if err := rows.Scan(&colName); err != nil {
			return columns, err
		}
		columns[colName] = true
	}

	return columns, nil
}
