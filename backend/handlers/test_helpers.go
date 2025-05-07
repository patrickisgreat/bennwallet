package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	"bennwallet/backend/database"
	"bennwallet/backend/middleware"

	_ "github.com/lib/pq"
)

// Define a constant for the test user ID that can be used across all tests
const TestUserID = "test-user-id"

// SetupTestAuth adds authentication context to the request
func SetupTestAuth(req *http.Request) *http.Request {
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, TestUserID)
	return req.WithContext(ctx)
}

// getTestDBConfig returns PostgreSQL test database configuration
func getTestDBConfig() database.PostgresConfig {
	return database.PostgresConfig{
		Host:     getEnvOrDefault("TEST_DB_HOST", "localhost"),
		Port:     getEnvOrDefault("TEST_DB_PORT", "5432"),
		User:     getEnvOrDefault("TEST_DB_USER", "postgres"),
		Password: getEnvOrDefault("TEST_DB_PASSWORD", "postgres"),
		DBName:   getEnvOrDefault("TEST_DB_NAME", "bennwallet_test"),
		SSLMode:  "disable",
	}
}

// getEnvOrDefault gets environment variable or returns default
func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// SetupTestDB initializes a test database with common tables needed for tests
func SetupTestDB() {
	// Create a test database connection
	config := getTestDBConfig()
	db, err := sql.Open("postgres", config.ConnectionString())
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
			is_admin BOOLEAN DEFAULT FALSE,
			role TEXT DEFAULT 'user'
		)
	`)
	if err != nil {
		panic(err)
	}

	// Insert test user
	_, err = db.Exec(`
		INSERT INTO users (id, username, name, is_admin, role)
		VALUES ($1, $2, $3, $4, $5)
	`, TestUserID, "testuser", "Test User", true, "admin")
	if err != nil {
		panic(err)
	}

	// Create permissions table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS permissions (
			id SERIAL PRIMARY KEY,
			granted_user_id TEXT NOT NULL,
			owner_user_id TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			permission_type TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP WITH TIME ZONE,
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
		// Clean up tables
		tables := []string{"users", "permissions"}
		for _, table := range tables {
			database.DB.Exec("DROP TABLE IF EXISTS " + table + " CASCADE")
		}

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

// CreateTestDB creates a new PostgreSQL database for testing
func CreateTestDB() *sql.DB {
	// Create test database
	config := getTestDBConfig()
	db, err := sql.Open("postgres", config.ConnectionString())
	if err != nil {
		panic(err)
	}
	database.DB = db
	return db
}
