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
	"bennwallet/backend/testutil"

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
	// Use testutil package for consistent test database configuration
	return database.PostgresConfig{
		Host:     getEnvOrDefault("POSTGRES_HOST", "localhost"),
		Port:     getEnvOrDefault("POSTGRES_PORT", "5432"),
		User:     getEnvOrDefault("POSTGRES_USER", "postgres"),
		Password: getEnvOrDefault("POSTGRES_PASSWORD", "postgres"),
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

	// Use shared schema creation
	if err := database.CreatePostgresSchema(db); err != nil {
		panic(err)
	}

	// Insert test user
	_, err = db.Exec(`
		INSERT INTO users (id, username, name, status, is_admin, role)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			username = EXCLUDED.username,
			name = EXCLUDED.name,
			status = EXCLUDED.status,
			is_admin = EXCLUDED.is_admin,
			role = EXCLUDED.role
	`, TestUserID, "testuser", "Test User", "approved", true, "admin")
	if err != nil {
		panic(err)
	}

	// Create permissions table (already created by schema, but safe to keep for idempotency)
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

// SetupPostgresTestDB creates a PostgreSQL database connection for testing and returns it
func SetupPostgresTestDB() (*sql.DB, error) {
	// Use the testutil package's setup function for consistency
	return testutil.SetupPostgresTestDB()
}

// SeedYNABTestData seeds test data for YNAB category groups and categories
func SeedYNABTestData(db *sql.DB) error {
	// Create YNAB group tables if they don't exist
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS ynab_category_groups (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			category_group_id TEXT,
			user_id TEXT NOT NULL,
			hidden BOOLEAN DEFAULT false,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Create YNAB categories table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ynab_categories (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			group_id TEXT,
			category_group_id TEXT,
			user_id TEXT NOT NULL,
			hidden BOOLEAN DEFAULT false,
			budget_amount DECIMAL(15,2),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Define test category groups
	categoryGroups := []struct {
		id      string
		name    string
		user_id string
	}{
		{id: "test-group-1", name: "Essentials", user_id: TestUserID},
		{id: "test-group-2", name: "Lifestyle", user_id: TestUserID},
		{id: "test-group-3", name: "Monthly Bills", user_id: TestUserID},
	}

	// Insert category groups
	for _, group := range categoryGroups {
		_, err := db.Exec(`
			INSERT INTO ynab_category_groups (id, name, category_group_id, user_id, hidden)
			VALUES ($1, $2, $1, $3, $4)
			ON CONFLICT (id) DO UPDATE SET
			name = $2,
			category_group_id = $1
		`, group.id, group.name, group.user_id, false)

		if err != nil {
			return err
		}
	}

	// Define test categories
	categories := []struct {
		id             string
		name           string
		user_id        string
		category_group string
	}{
		{id: "test-cat-1", name: "Groceries", user_id: TestUserID, category_group: "test-group-1"},
		{id: "test-cat-2", name: "Rent", user_id: TestUserID, category_group: "test-group-1"},
		{id: "test-cat-3", name: "Entertainment", user_id: TestUserID, category_group: "test-group-2"},
		{id: "test-cat-4", name: "Dining Out", user_id: TestUserID, category_group: "test-group-2"},
		{id: "test-cat-5", name: "Internet", user_id: TestUserID, category_group: "test-group-3"},
		{id: "test-cat-6", name: "Electricity", user_id: TestUserID, category_group: "test-group-3"},
	}

	// Insert categories with both group_id and category_group_id set
	for _, cat := range categories {
		_, err := db.Exec(`
			INSERT INTO ynab_categories (id, name, group_id, category_group_id, user_id, hidden, budget_amount)
			VALUES ($1, $2, $3, $3, $4, $5, $6)
			ON CONFLICT (id) DO UPDATE SET
			name = $2,
			group_id = $3,
			category_group_id = $3
		`, cat.id, cat.name, cat.category_group, cat.user_id, false, 0.0)

		if err != nil {
			return err
		}
	}

	return nil
}
