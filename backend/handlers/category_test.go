package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"bennwallet/backend/database"
	"bennwallet/backend/models"
)

func setupCategoryTestDB() {
	// Create a test database connection
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		panic(err)
	}
	database.DB = db

	// Create categories table
	_, err = db.Exec(`
		CREATE TABLE categories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT,
			user_id TEXT NOT NULL,
			color TEXT
		)
	`)
	if err != nil {
		panic(err)
	}
}

func TestAddCategory(t *testing.T) {
	setupCategoryTestDB()
	defer database.DB.Close()

	// Setup
	reqBody := models.Category{
		Name:        "Test Category",
		Description: "Test Description",
		UserID:      "test-user",
	}

	jsonBody, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/categories", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	AddCategory(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	var response models.Category
	err := json.NewDecoder(w.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Error decoding response: %v", err)
	}

	// Verify category was created in database
	var count int
	err = database.DB.QueryRow("SELECT COUNT(*) FROM categories WHERE name = ?", reqBody.Name).Scan(&count)
	if err != nil {
		t.Fatalf("Error checking category: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 category, got %d", count)
	}
}

func TestGetCategories(t *testing.T) {
	setupCategoryTestDB()
	defer database.DB.Close()

	// First add a test category
	_, err := database.DB.Exec(`
		INSERT INTO categories (name, description, user_id, color)
		VALUES (?, ?, ?, ?)
	`, "Test Category", "Test Description", "test-user", "#FF0000")
	if err != nil {
		t.Fatal(err)
	}

	// Setup request with userId query parameter
	req := httptest.NewRequest("GET", "/categories?userId=test-user", nil)
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
