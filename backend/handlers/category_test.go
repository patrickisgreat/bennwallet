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

func TestAddCategory(t *testing.T) {
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
	AddCategory(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	var response models.Category
	err = json.NewDecoder(w.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Error decoding response: %v", err)
	}

	// Verify category was created in database
	var count int
	err = database.DB.QueryRow("SELECT COUNT(*) FROM categories WHERE name = $1", reqBody.Name).Scan(&count)
	if err != nil {
		t.Fatalf("Error checking category: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 category, got %d", count)
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

	// First add a test category
	_, err = db.Exec(`
		INSERT INTO categories (name, description, user_id, color)
		VALUES ($1, $2, $3, $4)
	`, "Test Category", "Test Description", "test-user-id", "#FF0000")
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
