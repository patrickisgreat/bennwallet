package handlers

import (
	"context"
	"database/sql"
	"net/http"

	"bennwallet/backend/database"
	"bennwallet/backend/middleware"
	"bennwallet/backend/testutil"

	_ "github.com/lib/pq"
)

// Define a constant for the test user ID that can be used across all tests
const TestUserID = "test-user-id"

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

// MockAuthContext adds a mock user ID to the request context for testing
func MockAuthContext(req *http.Request, userID string) *http.Request {
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
	return req.WithContext(ctx)
}

// SetupPostgresTestDB creates a PostgreSQL database connection for testing and returns it
func SetupPostgresTestDB() (*sql.DB, error) {
	// Use the testutil package's setup function for consistency
	return testutil.SetupPostgresTestDB()
}
