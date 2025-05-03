package services

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"bennwallet/backend/database"
	"bennwallet/backend/models"
	"bennwallet/backend/security"
)

// SyncYNABCategoriesNew syncs YNAB categories for a user (using the new encrypted config)
func SyncYNABCategoriesNew(userID, budgetID string) error {
	log.Printf("Syncing YNAB categories for user %s with budget %s", userID, budgetID)

	// Get config to retrieve API token
	config, err := models.GetYNABConfig(database.DB, userID)
	if err != nil {
		return fmt.Errorf("error getting YNAB config: %w", err)
	}

	var token string
	if config.HasCredentials && config.EncryptedAPIToken != "" {
		// Get from encrypted field
		token, err = security.Decrypt(config.EncryptedAPIToken)
		if err != nil {
			return fmt.Errorf("error decrypting API token: %w", err)
		}
	} else {
		// Try legacy format
		var dbToken string
		err := database.DB.QueryRow(
			"SELECT token FROM user_ynab_settings WHERE user_id = ?",
			userID,
		).Scan(&dbToken)

		if err != nil {
			return fmt.Errorf("error getting YNAB token from legacy table: %w", err)
		}

		if strings.HasPrefix(dbToken, "enc:") {
			// For local dev, token is prefixed in DB
			token = strings.TrimPrefix(dbToken, "enc:")
		} else {
			return fmt.Errorf("unsupported token format in legacy table")
		}
	}

	// Make API request to YNAB
	url := fmt.Sprintf("https://api.ynab.com/v1/budgets/%s/categories", budgetID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error making request to YNAB API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Printf("YNAB API error: %s", string(body))
		return fmt.Errorf("YNAB API returned status %d", resp.StatusCode)
	}

	// Parse response
	var response models.YNABCategoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("error decoding response: %w", err)
	}

	// Begin transaction
	tx, err := database.DB.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}
	defer tx.Rollback()

	// First, clear out existing categories for this user
	_, err = tx.Exec("DELETE FROM ynab_categories WHERE user_id = ?", userID)
	if err != nil {
		return fmt.Errorf("error deleting existing categories: %w", err)
	}

	_, err = tx.Exec("DELETE FROM ynab_category_groups WHERE user_id = ?", userID)
	if err != nil {
		return fmt.Errorf("error deleting existing category groups: %w", err)
	}

	// Now insert the new category groups and categories
	now := time.Now()
	for _, group := range response.Data.CategoryGroups {
		// Skip internal, hidden, or deleted groups
		if group.Hidden || group.Deleted || strings.HasPrefix(group.ID, "internal:") {
			continue
		}

		// Insert category group
		_, err = tx.Exec(
			`INSERT OR REPLACE INTO ynab_category_groups (id, name, user_id, last_updated)
			VALUES (?, ?, ?, ?)`,
			group.ID, group.Name, userID, now,
		)
		if err != nil {
			return fmt.Errorf("error inserting category group %s: %w", group.Name, err)
		}

		// Insert categories
		for _, category := range group.Categories {
			// Skip hidden or deleted categories
			if category.Hidden || category.Deleted {
				continue
			}

			_, err = tx.Exec(
				`INSERT OR REPLACE INTO ynab_categories (id, group_id, name, user_id, last_updated)
				VALUES (?, ?, ?, ?, ?)`,
				category.ID, group.ID, category.Name, userID, now,
			)
			if err != nil {
				return fmt.Errorf("error inserting category %s: %w", category.Name, err)
			}
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}

	// Update last sync time
	if err := models.UpdateLastSyncTime(database.DB, userID); err != nil {
		log.Printf("Error updating last sync time: %v", err)
	}

	log.Printf("Successfully synced %d category groups for user %s", len(response.Data.CategoryGroups), userID)
	return nil
}
