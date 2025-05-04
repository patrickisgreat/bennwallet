package services

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"bennwallet/backend/database"
	"bennwallet/backend/models"
)

// SyncYNABCategories syncs YNAB categories for a specific user
func SyncYNABCategories(userID string, budgetID string) error {
	log.Printf("DEBUG: Starting YNAB categories sync for user %s with budget ID %s", userID, budgetID)

	// Get YNAB token directly from database for now
	var token string
	var tokenErr error

	// Retry up to 3 times for database operations
	for retries := 0; retries < 3; retries++ {
		err := database.DB.QueryRow(`
			SELECT token FROM user_ynab_settings WHERE user_id = ?
		`, userID).Scan(&token)

		if err != nil {
			if strings.Contains(err.Error(), "database is locked") {
				log.Printf("DEBUG: Database locked when getting token, retry %d/3", retries+1)
				time.Sleep(time.Duration(retries+1) * 500 * time.Millisecond)
				continue
			}
			tokenErr = err
			break
		}
		tokenErr = nil
		break
	}

	if tokenErr != nil {
		log.Printf("DEBUG: Error getting YNAB token for user %s: %v", userID, tokenErr)
		return fmt.Errorf("error getting YNAB token: %w", tokenErr)
	}

	log.Printf("DEBUG: Successfully retrieved token for user %s", userID)

	// If token starts with "enc:", remove the prefix
	if strings.HasPrefix(token, "enc:") {
		token = strings.TrimPrefix(token, "enc:")
		log.Printf("DEBUG: Removed 'enc:' prefix from token")
	}

	url := fmt.Sprintf("https://api.ynab.com/v1/budgets/%s/categories", budgetID)
	log.Printf("DEBUG: Making request to YNAB API: %s", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("DEBUG: Error creating HTTP request: %v", err)
		return fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("DEBUG: Error making HTTP request to YNAB API: %v", err)
		return fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("DEBUG: YNAB API response status: %d %s", resp.StatusCode, resp.Status)

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Printf("DEBUG: YNAB API error response: %s", string(body))
		return fmt.Errorf("YNAB API returned status code %d: %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("DEBUG: Error reading response body: %v", err)
		return fmt.Errorf("error reading response body: %w", err)
	}

	log.Printf("DEBUG: Received %d bytes from YNAB API", len(body))

	var categoryResponse models.YNABCategoryResponse
	if err := json.Unmarshal(body, &categoryResponse); err != nil {
		log.Printf("DEBUG: Error unmarshaling response: %v", err)
		log.Printf("DEBUG: Response body: %s", string(body))
		return fmt.Errorf("error unmarshaling response: %w", err)
	}

	log.Printf("DEBUG: Successfully unmarshaled YNAB categories response")

	// Count categories received
	var totalCategories int
	for _, group := range categoryResponse.Data.CategoryGroups {
		if !group.Deleted && !group.Hidden {
			totalCategories += len(group.Categories)
		}
	}

	log.Printf("DEBUG: Received %d category groups and %d total categories",
		len(categoryResponse.Data.CategoryGroups), totalCategories)

	// Retry the database transaction up to 3 times
	var dbErr error
	for attempt := 0; attempt < 3; attempt++ {
		dbErr = processCategoriesTransaction(userID, categoryResponse)
		if dbErr == nil {
			break
		}

		if strings.Contains(dbErr.Error(), "database is locked") {
			log.Printf("DEBUG: Database locked during transaction, retry %d/3", attempt+1)
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}

		// If it's not a lock error, break immediately
		break
	}

	if dbErr != nil {
		return dbErr
	}

	log.Printf("Successfully synced YNAB categories for user %s", userID)
	return nil
}

// processCategoriesTransaction handles the database transaction part of category syncing
func processCategoriesTransaction(userID string, categoryResponse models.YNABCategoryResponse) error {
	// Begin transaction
	tx, err := database.DB.Begin()
	if err != nil {
		log.Printf("DEBUG: Error beginning database transaction: %v", err)
		return fmt.Errorf("error beginning transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	syncTime := time.Now()
	var insertedGroups, insertedCategories int

	// Prepare statements for better performance
	stmtCategoryGroup, err := tx.Prepare(`
		INSERT OR REPLACE INTO ynab_category_groups (id, name, user_id, last_updated)
		VALUES (?, ?, ?, ?)
	`)
	if err != nil {
		log.Printf("DEBUG: Error preparing category group statement: %v", err)
		return fmt.Errorf("error preparing statement: %w", err)
	}
	defer stmtCategoryGroup.Close()

	stmtCategory, err := tx.Prepare(`
		INSERT OR REPLACE INTO ynab_categories (id, group_id, name, user_id, last_updated)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		log.Printf("DEBUG: Error preparing category statement: %v", err)
		return fmt.Errorf("error preparing statement: %w", err)
	}
	defer stmtCategory.Close()

	// Store category groups and categories
	for _, group := range categoryResponse.Data.CategoryGroups {
		// Skip deleted or hidden groups
		if group.Deleted || group.Hidden {
			continue
		}

		// Insert or update category group
		_, err = stmtCategoryGroup.Exec(group.ID, group.Name, userID, syncTime)
		if err != nil {
			log.Printf("DEBUG: Error inserting category group %s: %v", group.ID, err)
			return fmt.Errorf("error inserting category group: %w", err)
		}

		insertedGroups++

		// Insert categories
		for _, cat := range group.Categories {
			// Skip deleted or hidden categories
			if cat.Deleted || cat.Hidden {
				continue
			}

			_, err = stmtCategory.Exec(cat.ID, group.ID, cat.Name, userID, syncTime)
			if err != nil {
				log.Printf("DEBUG: Error inserting category %s: %v", cat.ID, err)
				return fmt.Errorf("error inserting category: %w", err)
			}

			insertedCategories++
		}
	}

	log.Printf("DEBUG: Inserted or updated %d category groups and %d categories for user %s",
		insertedGroups, insertedCategories, userID)

	// Convert YNAB categories to local categories for use in the transaction form
	result, err := tx.Exec(`
		INSERT INTO categories (name, description, user_id, color)
		SELECT y.name, 'Synced from YNAB', ?, COALESCE(
			(SELECT color FROM categories WHERE name = y.name AND user_id = ? LIMIT 1),
			?) 
		FROM ynab_categories y
		WHERE y.user_id = ?
		ON CONFLICT(name, user_id) DO UPDATE SET
			description = 'Synced from YNAB',
			last_updated = ?
	`, userID, userID, generateRandomColor(), userID, syncTime)

	if err != nil {
		log.Printf("DEBUG: Error converting to local categories: %v", err)
		return fmt.Errorf("error converting to local categories: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("DEBUG: Converted %d YNAB categories to local categories", rowsAffected)

	err = tx.Commit()
	if err != nil {
		log.Printf("DEBUG: Error committing transaction: %v", err)
		return fmt.Errorf("error committing transaction: %w", err)
	}

	log.Printf("DEBUG: Successfully committed transaction for user %s", userID)
	return nil
}

// SyncAllUsersYNABCategories syncs YNAB categories for all users with sync enabled
func SyncAllUsersYNABCategories() {
	log.Println("Starting YNAB categories sync for all users")

	// Get all users with sync enabled
	rows, err := database.DB.Query(`
		SELECT user_id, budget_id FROM user_ynab_settings 
		WHERE sync_enabled = 1
	`)
	if err != nil {
		log.Printf("Error fetching users for YNAB sync: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var userID, budgetID string
		err := rows.Scan(&userID, &budgetID)
		if err != nil {
			log.Printf("Error scanning user data: %v", err)
			continue
		}

		if err := SyncYNABCategories(userID, budgetID); err != nil {
			log.Printf("Error syncing YNAB categories for user %s: %v", userID, err)
		}
	}

	log.Println("Completed YNAB categories sync for all users")
}

// generateRandomColor generates a random color for categories
func generateRandomColor() string {
	colors := []string{
		"#FF6B6B", "#4ECDC4", "#45B7D1", "#96CEB4", "#FFEEAD",
		"#D4A5A5", "#9B59B6", "#3498DB", "#1ABC9C", "#F1C40F",
	}

	// Simple deterministic selection for now
	return colors[time.Now().Nanosecond()%len(colors)]
}

func init() {
	// Load .env file if it exists (for local dev)
	// Try first in the current directory, then in the parent directory
	envPaths := []string{".env", "../.env"}

	for _, path := range envPaths {
		if _, err := os.Stat(path); err == nil {
			log.Printf("DEBUG: Found .env file at %s", path)
			content, err := ioutil.ReadFile(path)
			if err == nil {
				log.Printf("DEBUG: Successfully read .env file")
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
							log.Printf("DEBUG: Set environment variable: %s", key)
						}
					}
				}
				return // Exit after loading the first found .env file
			} else {
				log.Printf("DEBUG: Error reading .env file: %v", err)
			}
		}
	}

	log.Printf("DEBUG: No .env file found in search paths: %v", envPaths)
}
