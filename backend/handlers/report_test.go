package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bennwallet/backend/database"
	"bennwallet/backend/middleware"
	"bennwallet/backend/models"
)

// Define a constant for the test user ID
const testUserID = "test-user-id"

func setupReportTestDB() {
	// Create a test database connection
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		panic(err)
	}
	database.DB = db

	// Create users table first for foreign key support
	_, err = db.Exec(`
		CREATE TABLE users (
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
	`, testUserID, "testuser", "Test User", true, "admin")
	if err != nil {
		panic(err)
	}

	// Create permissions table
	_, err = db.Exec(`
		CREATE TABLE permissions (
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

	// Create transactions table
	_, err = db.Exec(`
		CREATE TABLE transactions (
			id TEXT PRIMARY KEY,
			amount REAL NOT NULL,
			description TEXT NOT NULL,
			date DATETIME NOT NULL,
			transaction_date DATETIME,
			type TEXT NOT NULL,
			payTo TEXT,
			paid BOOLEAN NOT NULL DEFAULT 0,
			paidDate TEXT,
			enteredBy TEXT NOT NULL,
			optional BOOLEAN NOT NULL DEFAULT 0,
			userId TEXT
		)
	`)
	if err != nil {
		panic(err)
	}

	// Insert sample data for testing
	insertTestTransactions()
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
	}{
		{"tx1", 100.00, "Groceries 1", startDate, "Food", "Sarah", true, "Patrick", false, testUserID},
		{"tx2", 50.00, "Restaurant", midDate, "Food", "Patrick", true, "Sarah", false, testUserID},
		{"tx3", 200.00, "Rent", endDate, "Housing", "Sarah", true, "Sarah", false, testUserID},
		{"tx4", 75.00, "Groceries 2", midDate, "Food", "Sarah", true, "Patrick", false, testUserID},
		{"tx5", 150.00, "Utilities", midDate, "Housing", "Patrick", true, "Patrick", false, testUserID},
		{"tx6", 60.00, "Entertainment", endDate, "Fun", "Sarah", true, "Sarah", false, testUserID},
		{"tx7", 30.00, "Optional Expense", midDate, "Misc", "Patrick", true, "Sarah", true, testUserID},
		{"tx8", 80.00, "Unpaid Bill", midDate, "Bills", "Sarah", false, "Patrick", false, testUserID},
	}

	for _, tx := range testTransactions {
		_, err := database.DB.Exec(`
			INSERT INTO transactions 
			(id, amount, description, date, type, payTo, paid, enteredBy, optional, userId)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, tx.id, tx.amount, tx.description, tx.date, tx.txType, tx.payTo, tx.paid, tx.enteredBy, tx.optional, tx.userId)

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
			expectedFirst: "Bills",
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
