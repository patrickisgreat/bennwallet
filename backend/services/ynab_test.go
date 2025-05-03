package services

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"bennwallet/backend/database"
)

// setupYNABTestDB sets up an in-memory database for testing YNAB functionality
func setupYNABTestDB() {
	// Create test database
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		panic(err)
	}
	database.DB = db

	// Create the necessary tables
	createTables := []string{
		`CREATE TABLE ynab_config (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			encrypted_api_token TEXT,
			budget_id TEXT,
			account_id TEXT,
			sync_frequency INTEGER DEFAULT 24,
			last_sync_time DATETIME,
			has_credentials BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE user_ynab_settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			token TEXT,
			budget_id TEXT,
			account_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE ynab_category_groups (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			user_id TEXT NOT NULL,
			last_updated DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE ynab_categories (
			id TEXT PRIMARY KEY,
			group_id TEXT NOT NULL,
			name TEXT NOT NULL,
			user_id TEXT NOT NULL,
			last_updated DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (group_id) REFERENCES ynab_category_groups(id)
		)`,
	}

	for _, createSQL := range createTables {
		_, err := database.DB.Exec(createSQL)
		if err != nil {
			panic(err)
		}
	}
}

// insertTestYNABConfig adds a test YNAB configuration to the database
func insertTestYNABConfig(userID string, encryptedToken string, budgetID string) {
	_, err := database.DB.Exec(
		"INSERT INTO ynab_config (user_id, encrypted_api_token, budget_id, account_id, has_credentials) VALUES (?, ?, ?, ?, ?)",
		userID, encryptedToken, budgetID, "account-123", true,
	)
	if err != nil {
		panic(err)
	}
}

// insertLegacyYNABSettings adds a test legacy YNAB configuration
func insertLegacyYNABSettings(userID string, token string, budgetID string) {
	_, err := database.DB.Exec(
		"INSERT INTO user_ynab_settings (user_id, token, budget_id, account_id) VALUES (?, ?, ?, ?)",
		userID, token, budgetID, "account-123",
	)
	if err != nil {
		panic(err)
	}
}

// Skip test for now due to dependency on security.Decrypt
// This test will be improved once we have a better way to mock the dependencies
func TestSyncYNABCategoriesNew(t *testing.T) {
	t.Skip("Skipping test until proper mocking of security.Decrypt is implemented")

	setupYNABTestDB()
	defer database.DB.Close()

	// Mock YNAB API server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request
		if r.Header.Get("Authorization") != "Bearer decrypted-token" {
			t.Error("Incorrect Authorization header")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Return mock categories response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"data": {
				"category_groups": [
					{
						"id": "group1",
						"name": "Immediate Obligations",
						"hidden": false,
						"deleted": false,
						"categories": [
							{
								"id": "cat1",
								"name": "Rent/Mortgage",
								"hidden": false,
								"deleted": false
							},
							{
								"id": "cat2",
								"name": "Electric",
								"hidden": false,
								"deleted": false
							}
						]
					},
					{
						"id": "group2",
						"name": "True Expenses",
						"hidden": false,
						"deleted": false,
						"categories": [
							{
								"id": "cat3",
								"name": "Auto Maintenance",
								"hidden": false,
								"deleted": false
							},
							{
								"id": "cat4",
								"name": "Hidden Category",
								"hidden": true,
								"deleted": false
							}
						]
					},
					{
						"id": "internal:deleted",
						"name": "Deleted Group",
						"hidden": false,
						"deleted": true,
						"categories": []
					}
				]
			}
		}`))
	}))
	defer mockServer.Close()

	// Test with the new config format
	t.Run("New Config Format", func(t *testing.T) {
		// Insert test data
		userID := "test-user-1"
		budgetID := "budget-123"

		insertTestYNABConfig(userID, "encrypted-token", budgetID)

		// Call the function would go here
		// err := SyncYNABCategoriesNew(userID, budgetID)
		// if err != nil {
		// 	t.Fatalf("Error syncing categories: %v", err)
		// }
	})

	// Test with the legacy config format
	t.Run("Legacy Config Format", func(t *testing.T) {
		// Clear tables
		database.DB.Exec("DELETE FROM ynab_categories")
		database.DB.Exec("DELETE FROM ynab_category_groups")
		database.DB.Exec("DELETE FROM ynab_config")

		// Insert legacy test data
		userID := "test-user-2"
		budgetID := "budget-456"

		insertLegacyYNABSettings(userID, "enc:decrypted-token", budgetID)

		// Call the function would go here
		// err := SyncYNABCategoriesNew(userID, budgetID)
		// if err != nil {
		// 	t.Fatalf("Error syncing categories with legacy config: %v", err)
		// }
	})
}
