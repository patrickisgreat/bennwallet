package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"bennwallet/backend/database"
	"bennwallet/backend/models"

	_ "github.com/lib/pq"
)

func TestCreateCategory(t *testing.T) {
	// Create a test database connection using our common setup helper
	db, err := SetupPostgresTestDB()
	if err != nil {
		t.Fatalf("Failed to set up test database: %v", err)
	}
	database.DB = db
	defer database.DB.Close()

	// Check if categories table exists
	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'categories')").Scan(&exists)
	if err != nil {
		t.Fatalf("Error checking if categories table exists: %v", err)
	}

	// Only create the table if it doesn't exist
	if !exists {
		// Create categories table with PostgreSQL syntax
		_, err = db.Exec(`
			CREATE TABLE categories (
				id SERIAL PRIMARY KEY,
				name TEXT NOT NULL,
				description TEXT,
				user_id TEXT NOT NULL,
				color TEXT
			)
		`)
		if err != nil {
			t.Fatalf("Error creating categories table: %v", err)
		}
	} else {
		// Clear existing categories for a clean test
		_, err = db.Exec("DELETE FROM categories")
		if err != nil {
			t.Fatalf("Error clearing categories table: %v", err)
		}
	}

	// Setup
	reqBody := models.Category{
		Name:        "Test Category",
		Description: "Test Description",
		UserID:      "test-user",
	}

	jsonBody, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/categories", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	// Add mock authentication
	req = MockAuthContext(req, "test-user-id")
	w := httptest.NewRecorder()

	// Execute
	CreateCategory(w, req)

	// Since we've changed the implementation to return NotImplemented, update the test
	if w.Code != http.StatusNotImplemented {
		t.Errorf("Expected status code %d, got %d", http.StatusNotImplemented, w.Code)
	}
}

func TestGetCategories(t *testing.T) {
	// Create a test database connection using our common setup helper
	db, err := SetupPostgresTestDB()
	if err != nil {
		t.Fatalf("Failed to set up test database: %v", err)
	}
	database.DB = db
	defer database.DB.Close()

	// Check if ynab_categories table exists
	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'ynab_categories')").Scan(&exists)
	if err != nil {
		t.Fatalf("Error checking if ynab_categories table exists: %v", err)
	}

	// Only create the table if it doesn't exist
	if !exists {
		// Create ynab_categories table with PostgreSQL syntax
		_, err = db.Exec(`
			CREATE TABLE IF NOT EXISTS ynab_categories (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				category_group_id TEXT,
				hidden BOOLEAN DEFAULT false,
				budget_amount DECIMAL(15,2),
				user_id TEXT NOT NULL,
				created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
				last_updated TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
			)
		`)
		if err != nil {
			t.Fatalf("Error creating ynab_categories table: %v", err)
		}
	} else {
		// Clear existing categories for a clean test
		_, err = db.Exec("DELETE FROM ynab_categories")
		if err != nil {
			t.Fatalf("Error clearing ynab_categories table: %v", err)
		}
	}

	// First add a test category
	_, err = db.Exec(`
		INSERT INTO ynab_categories (id, name, user_id, hidden)
		VALUES ($1, $2, $3, $4)
	`, "test-category-id", "Test Category", "test-user-id", false)
	if err != nil {
		t.Fatal(err)
	}

	// Setup request with userId query parameter
	req := httptest.NewRequest("GET", "/categories", nil)
	// Add mock authentication
	req = MockAuthContext(req, "test-user-id")
	w := httptest.NewRecorder()

	// Execute
	GetCategories(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	var response []models.Category
	err = json.NewDecoder(w.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Error decoding response: %v", err)
	}

	// Verify we got the category we created
	if len(response) != 1 {
		t.Errorf("Expected 1 category, got %d", len(response))
	}

	if response[0].Name != "Test Category" {
		t.Errorf("Expected category name 'Test Category', got '%s'", response[0].Name)
	}
}
