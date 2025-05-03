package services

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"bennwallet/backend/database"
	"bennwallet/backend/models"
	"bennwallet/backend/security"
)

// SetupYNABFromEnv loads YNAB settings from environment variables and stores them in the database
// This is useful for initial setup and ensuring credentials are updated on app restarts
func SetupYNABFromEnv() {
	log.Println("Setting up YNAB configurations from environment variables...")

	// Get all environment variables that match our YNAB pattern
	setupYNABFromEnvironment()

	// Get all users
	rows, err := database.DB.Query("SELECT id FROM users")
	if err != nil {
		log.Printf("Error fetching users: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var userID string
		err := rows.Scan(&userID)
		if err != nil {
			log.Printf("Error scanning user ID: %v", err)
			continue
		}

		// Check for user-specific YNAB credentials in environment
		setupYNABFromEnvForUser(userID)
	}
}

// setupYNABFromEnvironment looks for YNAB credentials in environment variables for all possible users
func setupYNABFromEnvironment() {
	// Find all YNAB token variables for any user
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "YNAB_TOKEN_USER_") {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) != 2 {
				continue
			}

			// Extract user ID from the environment variable name
			// Format is YNAB_TOKEN_USER_{userID}
			userIDParts := strings.SplitN(parts[0], "YNAB_TOKEN_USER_", 2)
			if len(userIDParts) != 2 {
				continue
			}

			userID := userIDParts[1]
			if userID == "" {
				continue
			}

			// Now check if we have the corresponding budget and account IDs
			setupYNABFromEnvForUser(userID)
		}
	}
}

// setupYNABFromEnvForUser sets up YNAB settings for a specific user from environment variables
func setupYNABFromEnvForUser(userID string) {
	log.Printf("DEBUG: Checking for YNAB credentials for user %s", userID)

	// First check if user already has credentials in the database
	config, err := models.GetYNABConfig(database.DB, userID)
	if err == nil && config.HasCredentials {
		log.Printf("DEBUG: User %s already has YNAB credentials in ynab_config table, skipping setup from env", userID)
		return
	}

	// Check legacy table as well
	var count int
	err = database.DB.QueryRow(`
		SELECT COUNT(*) FROM user_ynab_settings 
		WHERE user_id = ? 
		AND token IS NOT NULL 
		AND token != ''
		AND budget_id IS NOT NULL
		AND budget_id != ''`,
		userID).Scan(&count)

	if err == nil && count > 0 {
		log.Printf("DEBUG: User %s already has YNAB credentials in legacy table, skipping setup from env", userID)
		return
	}

	// Only proceed with setup from environment variables if no existing credentials were found
	tokenEnvVar := fmt.Sprintf("YNAB_TOKEN_USER_%s", userID)
	token := os.Getenv(tokenEnvVar)
	if token == "" {
		log.Printf("DEBUG: No YNAB token found for user %s (env var: %s)", userID, tokenEnvVar)
		return
	}
	log.Printf("DEBUG: Found YNAB token for user %s", userID)

	budgetIDEnvVar := fmt.Sprintf("YNAB_BUDGET_ID_USER_%s", userID)
	accountIDEnvVar := fmt.Sprintf("YNAB_ACCOUNT_ID_USER_%s", userID)

	budgetID := os.Getenv(budgetIDEnvVar)
	accountID := os.Getenv(accountIDEnvVar)

	if budgetID == "" {
		log.Printf("DEBUG: No YNAB budget ID found for user %s (env var: %s)", userID, budgetIDEnvVar)
		return
	}

	if accountID == "" {
		log.Printf("DEBUG: No YNAB account ID found for user %s (env var: %s)", userID, accountIDEnvVar)
		return
	}

	log.Printf("DEBUG: Found complete YNAB credentials for user %s, updating database", userID)
	log.Printf("DEBUG: Using budget ID: %s, account ID: %s", budgetID, accountID)

	// Ensure user exists in users table
	_, err = database.DB.Exec(`
		INSERT OR IGNORE INTO users (id, username, name) 
		VALUES (?, ?, ?)
	`, userID, fmt.Sprintf("user_%s", userID), fmt.Sprintf("User %s", userID))
	if err != nil {
		log.Printf("DEBUG: Error ensuring user exists: %v", err)
	} else {
		log.Printf("DEBUG: Successfully ensured user %s exists in users table", userID)
	}

	// Create config update request
	configRequest := models.YNABConfigUpdateRequest{
		APIToken:  token,
		BudgetID:  budgetID,
		AccountID: accountID,
	}

	// Update YNAB config with encrypted values
	err = models.UpsertYNABConfig(database.DB, &configRequest, userID)
	if err != nil {
		log.Printf("DEBUG: Error updating YNAB config for user %s: %v", userID, err)
		return
	}

	log.Printf("DEBUG: Successfully updated YNAB config for user %s with encrypted values", userID)

	// Also update legacy table for backward compatibility
	// Store token with 'enc:' prefix for local dev
	hashedToken := fmt.Sprintf("enc:%s", token)
	if os.Getenv("FLY_APP_NAME") != "" {
		// In prod, just store a reference since real token is in env
		hashedToken = "[stored in environment variables]"
	}

	result, err := database.DB.Exec(`
		INSERT INTO user_ynab_settings (user_id, token, budget_id, account_id, sync_enabled)
		VALUES (?, ?, ?, ?, 1)
		ON CONFLICT(user_id) DO UPDATE SET
			token = excluded.token,
			budget_id = excluded.budget_id,
			account_id = excluded.account_id,
			sync_enabled = 1
	`, userID, hashedToken, budgetID, accountID)

	if err != nil {
		log.Printf("DEBUG: Error updating legacy YNAB settings for user %s: %v", userID, err)
	} else {
		rowsAffected, _ := result.RowsAffected()
		if rowsAffected > 0 {
			log.Printf("DEBUG: Successfully updated legacy YNAB settings for user %s", userID)
		}
	}
}

// InitialSync performs an initial sync of YNAB categories for all users
func InitialSync() {
	log.Println("Performing initial YNAB sync for all users...")

	// First check the new ynab_config table
	configRows, err := database.DB.Query(`
		SELECT user_id, encrypted_budget_id 
		FROM ynab_config 
		WHERE encrypted_api_token IS NOT NULL AND encrypted_api_token != ''
		AND encrypted_budget_id IS NOT NULL AND encrypted_budget_id != ''
		AND encrypted_account_id IS NOT NULL AND encrypted_account_id != ''
	`)

	if err != nil {
		log.Printf("Error fetching users from ynab_config: %v", err)
	} else {
		defer configRows.Close()

		// Process users from new config table
		var userCount int
		for configRows.Next() {
			userCount++
			var userID string
			var encryptedBudgetID string

			err := configRows.Scan(&userID, &encryptedBudgetID)
			if err != nil {
				log.Printf("Error scanning config data: %v", err)
				continue
			}

			// Decrypt budget ID
			budgetID, err := security.Decrypt(encryptedBudgetID)
			if err != nil {
				log.Printf("Error decrypting budget ID for user %s: %v", userID, err)
				continue
			}

			log.Printf("Found user %s with YNAB configured in new format, budget ID: %s", userID, budgetID)
			log.Printf("Syncing YNAB categories for user %s", userID)

			if err := SyncYNABCategoriesNew(userID, budgetID); err != nil {
				log.Printf("Error syncing categories for user %s: %v", userID, err)
			}
		}

		if userCount > 0 {
			log.Printf("Completed initial sync for %d users from ynab_config table", userCount)
			return
		}
	}

	// If no users found in new table, check legacy table
	legacyRows, err := database.DB.Query(`
		SELECT user_id, budget_id 
		FROM user_ynab_settings 
		WHERE sync_enabled = 1
		AND token IS NOT NULL AND token != ''
		AND budget_id IS NOT NULL AND budget_id != ''
		AND account_id IS NOT NULL AND account_id != ''
	`)
	if err != nil {
		log.Printf("Error fetching users with YNAB settings from legacy table: %v", err)
		return
	}
	defer legacyRows.Close()

	// Count users with YNAB settings in legacy table
	var userCount int
	for legacyRows.Next() {
		userCount++
		var userID, budgetID string
		err := legacyRows.Scan(&userID, &budgetID)
		if err != nil {
			log.Printf("Error scanning user data: %v", err)
			continue
		}

		log.Printf("Found user %s with YNAB sync enabled in legacy table, budget ID: %s", userID, budgetID)
		log.Printf("Syncing YNAB categories for user %s", userID)
		if err := SyncYNABCategoriesNew(userID, budgetID); err != nil {
			log.Printf("Error syncing categories for user %s: %v", userID, err)
		}
	}

	// If no users have YNAB configured, log this fact
	if userCount == 0 {
		log.Println("WARNING: No users found with YNAB sync enabled!")

		// Check if any users have YNAB settings at all, even if sync is disabled
		var count int
		err := database.DB.QueryRow("SELECT COUNT(*) FROM user_ynab_settings").Scan(&count)
		if err != nil {
			log.Printf("Error checking for any YNAB settings: %v", err)
		} else if count == 0 {
			log.Println("WARNING: No user_ynab_settings found in the database at all!")
		} else {
			log.Printf("Found %d user_ynab_settings records, but none have sync_enabled=1", count)
		}

		// List all environment variables related to YNAB
		log.Println("DEBUG: Checking for YNAB environment variables:")
		for _, env := range os.Environ() {
			if strings.Contains(strings.ToUpper(env), "YNAB") {
				parts := strings.SplitN(env, "=", 2)
				if len(parts) > 0 {
					log.Printf("DEBUG: Found YNAB env var: %s", parts[0])
				}
			}
		}
	}

	log.Println("Initial YNAB sync completed")
}

// LoadEnvVariables loads environment variables without doing any database operations
func LoadEnvVariables() {
	log.Println("Loading environment variables...")

	// Load .env file if it exists (for local dev)
	// Try first in the current directory, then in the parent directory
	envPaths := []string{".env", "../.env"}

	for _, path := range envPaths {
		if _, err := os.Stat(path); err == nil {
			log.Printf("Found .env file at %s", path)
			content, err := ioutil.ReadFile(path)
			if err == nil {
				log.Printf("Successfully read .env file")
				lines := strings.Split(string(content), "\n")
				for _, line := range lines {
					if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
						continue // Skip comments and empty lines
					}
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						key := strings.TrimSpace(parts[0])
						value := strings.TrimSpace(parts[1])
						os.Setenv(key, value)
						if strings.Contains(strings.ToUpper(key), "YNAB") {
							// Log the key but not the value for security
							log.Printf("Set environment variable: %s", key)
						}
					}
				}
				return // Exit after loading the first found .env file
			} else {
				log.Printf("Error reading .env file: %v", err)
			}
		}
	}

	log.Printf("No .env file found in search paths: %v", envPaths)
}

// SetupYNABForUser sets up YNAB for a user from env vars if they don't have credentials
func SetupYNABForUser(userID string) {
	log.Printf("Setting up YNAB for user %s", userID)

	// First check if user already has credentials in the database
	config, err := models.GetYNABConfig(database.DB, userID)
	if err == nil && config.HasCredentials {
		log.Printf("User %s already has YNAB credentials in ynab_config table, skipping setup from env", userID)
		return
	}

	// Check legacy table as well
	var count int
	err = database.DB.QueryRow(`
		SELECT COUNT(*) FROM user_ynab_settings 
		WHERE user_id = ? 
		AND token IS NOT NULL 
		AND token != ''
		AND budget_id IS NOT NULL
		AND budget_id != ''`,
		userID).Scan(&count)

	if err == nil && count > 0 {
		log.Printf("User %s already has YNAB credentials in legacy table, skipping setup from env", userID)
		return
	}

	// Check for user-specific YNAB credentials
	token := os.Getenv(fmt.Sprintf("YNAB_TOKEN_USER_%s", userID))
	if token == "" {
		log.Printf("No YNAB token found for user %s", userID)
		return
	}

	budgetID := os.Getenv(fmt.Sprintf("YNAB_BUDGET_ID_USER_%s", userID))
	accountID := os.Getenv(fmt.Sprintf("YNAB_ACCOUNT_ID_USER_%s", userID))

	if budgetID == "" || accountID == "" {
		log.Printf("Incomplete YNAB settings for user %s", userID)
		return
	}

	log.Printf("Found YNAB credentials for user %s", userID)

	// Ensure user exists
	_, err = database.DB.Exec(`
		INSERT OR IGNORE INTO users (id, username, name) 
		VALUES (?, ?, ?)
	`, userID, fmt.Sprintf("user_%s", userID), fmt.Sprintf("User %s", userID))

	if err != nil {
		log.Printf("Error ensuring user exists: %v", err)
		return
	}

	// Create config update request
	configRequest := models.YNABConfigUpdateRequest{
		APIToken:  token,
		BudgetID:  budgetID,
		AccountID: accountID,
	}

	// Update YNAB config with encrypted values
	err = models.UpsertYNABConfig(database.DB, &configRequest, userID)
	if err != nil {
		log.Printf("Error updating YNAB config for user %s: %v", userID, err)
		return
	}

	log.Printf("Successfully updated YNAB config for user %s", userID)

	// Also update legacy table for backward compatibility
	// Store token with 'enc:' prefix for local dev
	hashedToken := fmt.Sprintf("enc:%s", token)
	if os.Getenv("FLY_APP_NAME") != "" {
		// In prod, just store a reference since real token is in env
		hashedToken = "[stored in environment variables]"
	}

	// Update YNAB settings in legacy table
	_, err = database.DB.Exec(`
		INSERT INTO user_ynab_settings (user_id, token, budget_id, account_id, sync_enabled)
		VALUES (?, ?, ?, ?, 1)
		ON CONFLICT(user_id) DO UPDATE SET
			token = excluded.token,
			budget_id = excluded.budget_id,
			account_id = excluded.account_id,
			sync_enabled = 1
	`, userID, hashedToken, budgetID, accountID)

	if err != nil {
		log.Printf("Error updating legacy YNAB settings: %v", err)
		return
	}

	// Trigger an immediate sync
	go func() {
		if err := SyncYNABCategoriesNew(userID, budgetID); err != nil {
			log.Printf("Error during initial sync for user %s: %v", userID, err)
		} else {
			log.Printf("Successfully completed initial sync for user %s", userID)
		}
	}()
}
