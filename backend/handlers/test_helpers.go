package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"bennwallet/backend/database"
)

// MockAuthContext adds a mock user ID to the request context for testing
func MockAuthContext(req *http.Request, userID string) *http.Request {
	ctx := context.WithValue(req.Context(), "user_id", userID)
	return req.WithContext(ctx)
}

// NewAuthenticatedRequest creates a new HTTP request with a mock authenticated user
func NewAuthenticatedRequest(method, url string, body interface{}) *http.Request {
	var req *http.Request

	if body != nil {
		// Convert body to JSON buffer if needed
		buf, _ := json.Marshal(body)
		req = httptest.NewRequest(method, url, bytes.NewBuffer(buf))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, url, nil)
	}

	// Add mock user authentication
	return MockAuthContext(req, "test-user-id")
}

// CleanupTestDB cleans up all test data in the database
func CleanupTestDB() {
	// Only try to clean up if DB is available
	if database.DB == nil {
		return
	}

	// Tables that might need cleanup
	tables := []string{
		"transactions",
		"users",
		"categories",
		"ynab_config",
		"user_ynab_settings",
		"ynab_categories",
		"ynab_category_groups",
	}

	for _, table := range tables {
		// Check if the table exists first
		var exists bool
		err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name=?)", table).Scan(&exists)
		if err != nil || !exists {
			continue // Skip this table if it doesn't exist or we can't check
		}

		// Delete all rows from the table
		_, err = database.DB.Exec("DELETE FROM " + table)
		if err != nil {
			// Just log the error, don't fail the test
			// log.Printf("Error cleaning up table %s: %v", table, err)
		}
	}
}

// CreateTestDB creates a new in-memory database for testing
func CreateTestDB() *sql.DB {
	// Create test database
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		panic(err)
	}
	database.DB = db
	return db
}
