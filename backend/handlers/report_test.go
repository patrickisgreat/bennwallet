package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"bennwallet/backend/database"
	"bennwallet/backend/middleware"
	"bennwallet/backend/models"

	_ "github.com/lib/pq"
)

// Define a constant for the test user ID
const testUserID = "test-user-id"

func setupReportTestDB() {
	// Create a test database connection
	config := database.PostgresConfig{
		Host:     getEnvWithDefault("TEST_DB_HOST", "localhost"),
		Port:     getEnvWithDefault("TEST_DB_PORT", "5432"),
		User:     getEnvWithDefault("TEST_DB_USER", "postgres"),
		Password: getEnvWithDefault("TEST_DB_PASSWORD", "postgres"),
		DBName:   getEnvWithDefault("TEST_DB_NAME", "bennwallet_test"),
		SSLMode:  "disable",
	}

	db, err := sql.Open("postgres", config.ConnectionString())
	if err != nil {
		panic(err)
	}
	database.DB = db

	// Clear existing tables for this test
	_, err = db.Exec(`
		DROP TABLE IF EXISTS transactions CASCADE;
		DROP TABLE IF EXISTS permissions CASCADE;
		DROP TABLE IF EXISTS users CASCADE;
	`)
	if err != nil {
		panic(err)
	}

	// Create users table first for foreign key support
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT,
			name TEXT,
			status TEXT,
			is_admin BOOLEAN DEFAULT false,
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
		ON CONFLICT (id) DO UPDATE SET 
		  username = EXCLUDED.username,
		  name = EXCLUDED.name,
		  is_admin = EXCLUDED.is_admin,
		  role = EXCLUDED.role
	`, testUserID, "testuser", "Test User", true, "admin")
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
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP,
			UNIQUE(granted_user_id, owner_user_id, resource_type, permission_type)
		)
	`)
	if err != nil {
		panic(err)
	}

	// Create transactions table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS transactions (
			id TEXT PRIMARY KEY,
			amount NUMERIC(15,2) NOT NULL,
			description TEXT NOT NULL,
			date TIMESTAMP NOT NULL,
			transaction_date TIMESTAMP,
			type TEXT NOT NULL,
			pay_to TEXT,
			paid BOOLEAN NOT NULL DEFAULT false,
			paid_date TEXT,
			entered_by TEXT NOT NULL,
			optional BOOLEAN NOT NULL DEFAULT false,
			user_id TEXT,
			note TEXT
		)
	`)
	if err != nil {
		panic(err)
	}

	// Create ynab_category_groups table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ynab_category_groups (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			category_group_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			hidden BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(category_group_id, user_id)
		)
	`)
	if err != nil {
		panic(err)
	}

	// Create ynab_categories table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ynab_categories (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			user_id TEXT NOT NULL,
			group_id TEXT NOT NULL,
			category_group_id TEXT NOT NULL,
			hidden BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(category_group_id, user_id),
			FOREIGN KEY (group_id) REFERENCES ynab_category_groups(id)
		)
	`)
	if err != nil {
		panic(err)
	}

	// Insert test category groups
	_, err = db.Exec(`
		INSERT INTO ynab_category_groups (id, name, category_group_id, user_id, hidden)
		VALUES 
		('group-1', 'Food', 'group-1', $1, false),
		('group-2', 'Housing', 'group-2', $1, false),
		('group-3', 'Fun', 'group-3', $1, false)
		ON CONFLICT (id) DO NOTHING
	`, testUserID)
	if err != nil {
		panic(err)
	}

	// Insert test categories
	_, err = db.Exec(`
		INSERT INTO ynab_categories (id, name, user_id, group_id, category_group_id, hidden)
		VALUES 
		('cat-test-user-id-Food', 'Food', $1, 'group-1', 'group-1', false),
		('cat-test-user-id-Housing', 'Housing', $1, 'group-2', 'group-2', false),
		('cat-test-user-id-Fun', 'Fun', $1, 'group-3', 'group-3', false)
		ON CONFLICT (id) DO UPDATE SET
		name = EXCLUDED.name,
		group_id = EXCLUDED.group_id,
		category_group_id = EXCLUDED.category_group_id,
		hidden = EXCLUDED.hidden
	`, testUserID)
	if err != nil {
		panic(err)
	}

	// Create transaction_categories join table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS transaction_categories (
			id SERIAL PRIMARY KEY,
			transaction_id TEXT NOT NULL REFERENCES transactions(id),
			category_id TEXT NOT NULL REFERENCES ynab_categories(id),
			amount NUMERIC(15,2) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(transaction_id, category_id)
		)
	`)
	if err != nil {
		panic(err)
	}

	// Insert sample data for testing
	insertTestTransactions()
}

// Helper function to get environment variable with default
func getEnvWithDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func insertTestTransactions() {
	// Date format for consistent testing
	dateFormat := "2006-01-02"

	// Create sample dates for testing
	startDate, _ := time.Parse(dateFormat, "2023-01-01")
	midDate, _ := time.Parse(dateFormat, "2023-02-15")
	endDate, _ := time.Parse(dateFormat, "2023-03-31")

	// Insert test transactions
	testTransactions := []struct {
		id          string
		amount      float64
		description string
		date        time.Time
		txType      string
		payTo       string
		paid        bool
		enteredBy   string
		optional    bool
		userId      string
		categoryId  string
	}{
		{"tx1", 100.00, "Groceries 1", startDate, "Food", "Sarah", true, "Patrick", false, testUserID, "cat-test-user-id-Food"},
		{"tx2", 50.00, "Restaurant", midDate, "Food", "Patrick", true, "Sarah", false, testUserID, "cat-test-user-id-Food"},
		{"tx3", 200.00, "Rent", endDate, "Housing", "Sarah", true, "Sarah", false, testUserID, "cat-test-user-id-Housing"},
		{"tx4", 75.00, "Groceries 2", midDate, "Food", "Sarah", true, "Patrick", false, testUserID, "cat-test-user-id-Food"},
		{"tx5", 150.00, "Utilities", midDate, "Housing", "Patrick", true, "Patrick", false, testUserID, "cat-test-user-id-Housing"},
		{"tx6", 60.00, "Entertainment", endDate, "Fun", "Sarah", true, "Sarah", false, testUserID, "cat-test-user-id-Fun"},
		{"tx7", 30.00, "Optional Expense", midDate, "Misc", "Patrick", true, "Sarah", true, testUserID, "cat-test-user-id-Food"},
		{"tx8", 80.00, "Unpaid Bill", midDate, "Bills", "Sarah", false, "Patrick", false, testUserID, "cat-test-user-id-Housing"},
	}

	for _, tx := range testTransactions {
		_, err := database.DB.Exec(`
			INSERT INTO transactions 
			(id, amount, description, date, type, pay_to, paid, entered_by, optional, user_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, tx.id, tx.amount, tx.description, tx.date, tx.txType, tx.payTo, tx.paid, tx.enteredBy, tx.optional, tx.userId)

		if err != nil {
			panic(err)
		}

		// Insert transaction category
		_, err = database.DB.Exec(`
			INSERT INTO transaction_categories (transaction_id, category_id, amount)
			VALUES ($1, $2, $3)
		`, tx.id, tx.categoryId, tx.amount)

		if err != nil {
			panic(err)
		}
	}
}

func TestGetYNABSplits(t *testing.T) {
	setupReportTestDB()
	defer func() {
		CleanupTestDB()
		database.DB.Close()
	}()

	testCases := []struct {
		name          string
		filter        models.ReportFilter
		expectedCount int
		expectedTotal float64
		expectedFirst string // category name of first result
	}{
		{
			name: "All paid transactions, no optional",
			filter: models.ReportFilter{
				Paid:     boolPtr(true),
				Optional: boolPtr(false),
			},
			expectedCount: 3,         // Food, Housing, Fun categories
			expectedTotal: 635.00,    // Sum of all paid, non-optional transactions
			expectedFirst: "Housing", // Highest total should be Housing: 350.00
		},
		{
			name: "Food category only",
			filter: models.ReportFilter{
				Category: "Food",
				Paid:     boolPtr(true),
			},
			expectedCount: 1,
			expectedTotal: 225.00, // 100 + 50 + 75
			expectedFirst: "Food",
		},
		{
			name: "Entered by Patrick",
			filter: models.ReportFilter{
				EnteredBy: "Patrick",
				Paid:      boolPtr(true),
			},
			expectedCount: 2,      // Food and Housing
			expectedTotal: 325.00, // 100 + 75 + 150
			expectedFirst: "Food", // Food has higher total in this set
		},
		{
			name: "Date range filter",
			filter: models.ReportFilter{
				StartDate: "2023-02-01",
				EndDate:   "2023-02-28",
				Paid:      boolPtr(true),
			},
			expectedCount: 2,         // Housing and Food
			expectedTotal: 275.00,    // 50 + 75 + 150, without the optional transaction
			expectedFirst: "Housing", // Highest total in this date range
		},
		{
			name: "Include optional transactions",
			filter: models.ReportFilter{
				Paid:     boolPtr(true),
				Optional: boolPtr(true),
			},
			expectedCount: 4,         // Food, Housing, Fun, Misc
			expectedTotal: 665.00,    // All paid transactions including optional
			expectedFirst: "Housing", // Still highest total
		},
		{
			name: "Unpaid transactions only",
			filter: models.ReportFilter{
				Paid: boolPtr(false),
			},
			expectedCount: 1,
			expectedTotal: 80.00,
			expectedFirst: "Housing",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request
			requestBody, _ := json.Marshal(tc.filter)
			req := httptest.NewRequest("POST", "/reports/ynab-splits", bytes.NewBuffer(requestBody))
			req.Header.Set("Content-Type", "application/json")

			// Add authentication context with test user ID
			ctx := context.WithValue(req.Context(), middleware.UserIDKey, testUserID)
			req = req.WithContext(ctx)

			// Create response recorder
			w := httptest.NewRecorder()

			// Call the handler
			GetYNABSplits(w, req)

			// Check response code
			if w.Code != http.StatusOK {
				t.Errorf("Expected status OK, got %v", w.Code)
			}

			// Parse the response
			var response []models.CategoryTotal
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			// Check the result count
			if len(response) != tc.expectedCount {
				t.Errorf("Expected %d categories, got %d", tc.expectedCount, len(response))
			}

			// Skip further checks if response is empty
			if len(response) == 0 {
				return
			}

			// Check first category (should be highest total)
			if response[0].Category != tc.expectedFirst {
				t.Errorf("Expected first category to be %s, got %s", tc.expectedFirst, response[0].Category)
			}

			// Calculate total amount
			var total float64
			for _, cat := range response {
				total += cat.Total
			}

			// Check with a small tolerance for floating point comparisons
			tolerance := 0.01
			if total < tc.expectedTotal-tolerance || total > tc.expectedTotal+tolerance {
				t.Errorf("Expected total around %f, got %f", tc.expectedTotal, total)
			}
		})
	}
}

// Helper function to create a bool pointer
func boolPtr(b bool) *bool {
	return &b
}
