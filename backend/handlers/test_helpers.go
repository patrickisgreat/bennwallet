package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"

	"bennwallet/backend/database"
	"bennwallet/backend/middleware"
)

// Define a constant for the test user ID that can be used across all tests
const TestUserID = "test-user-id"

// SetupTestAuth adds authentication context to the request
func SetupTestAuth(req *http.Request) *http.Request {
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, TestUserID)
	return req.WithContext(ctx)
}

// SetupTestDB initializes a test database with common tables needed for tests
func SetupTestDB() {
	// Create a test database connection
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		panic(err)
	}
	database.DB = db

	// Create users table first for foreign key support
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT,
			name TEXT,
			status TEXT,
			isAdmin BOOLEAN DEFAULT 0,
			role TEXT DEFAULT 'user'
		)
	`)
	if err != nil {
		panic(err)
	}

	// Insert test user
	_, err = db.Exec(`
		INSERT INTO users (id, username, name, isAdmin, role)
		VALUES (?, ?, ?, ?, ?)
	`, TestUserID, "testuser", "Test User", true, "admin")
	if err != nil {
		panic(err)
	}

	// Create permissions table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS permissions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			granted_user_id TEXT NOT NULL,
			owner_user_id TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			permission_type TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP,
			UNIQUE(granted_user_id, owner_user_id, resource_type, permission_type)
		)
	`)
	if err != nil {
		panic(err)
	}
}

// CleanupTestDB closes the test database connection
func CleanupTestDB() {
	if database.DB != nil {
		database.DB.Close()
	}
}

// TestHandler wraps a handler function to add auth context
func TestHandler(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Add auth context to the request
		r = SetupTestAuth(r)

		// Call the original handler
		h(w, r)
	}
}

// TestRequest creates a test request with auth context already set up
func TestRequest(method, url string, body *string) *http.Request {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, url, strings.NewReader(*body))
	} else {
		req = httptest.NewRequest(method, url, nil)
	}

	return SetupTestAuth(req)
}

// MockAuthContext adds a mock user ID to the request context for testing
func MockAuthContext(req *http.Request, userID string) *http.Request {
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
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
