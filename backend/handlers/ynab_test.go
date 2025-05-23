package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"bennwallet/backend/database"
	"bennwallet/backend/models"
)

func setupYNABTestDB() {
	// Create a test database connection
	db, err := SetupPostgresTestDB()
	if err != nil {
		panic(err)
	}
	database.DB = db

	// Create user_ynab_settings table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_ynab_settings (
			user_id TEXT PRIMARY KEY,
			token TEXT,
			budget_id TEXT,
			account_id TEXT,
			sync_enabled BOOLEAN DEFAULT true,
			last_synced TIMESTAMP
		)
	`)
	if err != nil {
		panic(err)
	}

	// Insert test YNAB settings for our test user
	_, err = db.Exec(`
		INSERT INTO user_ynab_settings (user_id, token, budget_id, account_id, sync_enabled)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE SET
		token = $2,
		budget_id = $3,
		account_id = $4,
		sync_enabled = $5
	`, TestUserID, "test-token", "test-budget", "test-account", true)
	if err != nil {
		panic(err)
	}

	// Use the helper function to seed YNAB test data with proper groups and categories
	err = SeedYNABTestData(db)
	if err != nil {
		panic(err)
	}
}

func TestGetYNABCategories(t *testing.T) {
	setupYNABTestDB()
	defer database.DB.Close()

	// Create a test request with authentication
	req := MockAuthContext(httptest.NewRequest("GET", "/api/ynab/categories", nil), TestUserID)
	w := httptest.NewRecorder()

	// Call the handler
	GetYNABCategories(w, req)

	// Check response status code
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Parse the response
	var categoryGroups []struct {
		ID         string                `json:"id"`
		Name       string                `json:"name"`
		Categories []models.YNABCategory `json:"categories"`
	}
	err := json.NewDecoder(w.Body).Decode(&categoryGroups)
	if err != nil {
		t.Fatalf("Error decoding response: %v", err)
	}

	// Verify we got the expected number of category groups
	expectedGroups := 3 // We created 3 groups in SeedYNABTestData
	if len(categoryGroups) != expectedGroups {
		t.Errorf("Expected %d category groups, got %d", expectedGroups, len(categoryGroups))
	}

	// Verify each group has the correct categories
	categoryCountByGroup := map[string]int{
		"test-group-1": 2, // Groceries, Rent
		"test-group-2": 2, // Entertainment, Dining Out
		"test-group-3": 2, // Internet, Electricity
	}

	for _, group := range categoryGroups {
		expectedCount, exists := categoryCountByGroup[group.ID]
		if !exists {
			t.Errorf("Unexpected category group ID: %s", group.ID)
			continue
		}

		if len(group.Categories) != expectedCount {
			t.Errorf("Expected %d categories in group %s, got %d", expectedCount, group.ID, len(group.Categories))
		}

		// Verify the category group ID is correctly set
		for _, cat := range group.Categories {
			if cat.CategoryGroupID != group.ID {
				t.Errorf("Category %s has wrong group ID: expected %s, got %s", cat.Name, group.ID, cat.CategoryGroupID)
			}
		}
	}
}
