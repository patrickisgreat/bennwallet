package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bennwallet/backend/database"
	"bennwallet/backend/models"
	"bennwallet/backend/testutil"

	_ "github.com/lib/pq"
)

// Define a constant for the test user ID
const testUserID = "test-user-id"

func setupReportTestDB(t *testing.T) {
	// Use the centralized test setup
	db, cleanup := testutil.SetupTestDB(t)
	database.DB = db
	t.Cleanup(cleanup)

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

	// Create test user first
	_, err := database.DB.Exec(`
		INSERT INTO users (id, username, name, role, status, is_admin) 
		VALUES ($1, 'testuser', 'Test User', 'user', 'active', false)
		ON CONFLICT (id) DO NOTHING
	`, testUserID)
	if err != nil {
		panic(err)
	}

	// Insert test category groups
	_, err = database.DB.Exec(`
		INSERT INTO ynab_category_groups (id, name, category_group_id, user_id, hidden)
		VALUES 
		('group-1', 'Food', 'group-1', 'test-user-id', false),
		('group-2', 'Housing', 'group-2', 'test-user-id', false),
		('group-3', 'Fun', 'group-3', 'test-user-id', false),
		('group-4', 'Misc', 'group-4', 'test-user-id', false)
		ON CONFLICT (id) DO UPDATE SET
		name = EXCLUDED.name,
		category_group_id = EXCLUDED.category_group_id,
		user_id = EXCLUDED.user_id,
		hidden = EXCLUDED.hidden
	`)
	if err != nil {
		panic(err)
	}

	// Insert test categories
	_, err = database.DB.Exec(`
		INSERT INTO ynab_categories (id, name, user_id, category_group_id, hidden)
		VALUES 
		('cat-test-user-id-Food', 'Food', 'test-user-id', 'group-1', false),
		('cat-test-user-id-Housing', 'Housing', 'test-user-id', 'group-2', false),
		('cat-test-user-id-Fun', 'Fun', 'test-user-id', 'group-3', false),
		('cat-test-user-id-Misc', 'Misc', 'test-user-id', 'group-4', false)
		ON CONFLICT (id) DO UPDATE SET
		name = EXCLUDED.name,
		user_id = EXCLUDED.user_id,
		category_group_id = EXCLUDED.category_group_id,
		hidden = EXCLUDED.hidden
	`)
	if err != nil {
		panic(err)
	}

	// Clear existing transactions first
	_, err = database.DB.Exec(`
		DELETE FROM transaction_categories;
		DELETE FROM transactions;
	`)
	if err != nil {
		panic(err)
	}

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
		// Food transactions
		{"tx1", 100.00, "Groceries 1", startDate, "Food", "Sarah", true, "Patrick", false, testUserID, "cat-test-user-id-Food"},
		{"tx2", 50.00, "Restaurant", midDate, "Food", "Patrick", true, "Sarah", false, testUserID, "cat-test-user-id-Food"},
		{"tx4", 75.00, "Groceries 2", midDate, "Food", "Sarah", true, "Patrick", false, testUserID, "cat-test-user-id-Food"},

		// Housing transactions
		{"tx3", 200.00, "Rent", endDate, "Housing", "Sarah", true, "Sarah", false, testUserID, "cat-test-user-id-Housing"},
		{"tx5", 150.00, "Utilities", midDate, "Housing", "Patrick", true, "Patrick", false, testUserID, "cat-test-user-id-Housing"},
		{"tx8", 80.00, "Unpaid Bill", midDate, "Bills", "Sarah", false, "Patrick", false, testUserID, "cat-test-user-id-Housing"},

		// Fun transaction
		{"tx6", 60.00, "Entertainment", endDate, "Fun", "Sarah", true, "Sarah", false, testUserID, "cat-test-user-id-Fun"},

		// Optional transaction
		{"tx7", 30.00, "Optional Expense", midDate, "Misc", "Patrick", true, "Sarah", true, testUserID, "cat-test-user-id-Misc"},
	}

	for _, tx := range testTransactions {
		_, err := database.DB.Exec(`
			INSERT INTO transactions 
			(id, amount, description, date, transaction_date, type, pay_to, paid, entered_by, optional, user_id)
			VALUES ($1, $2, $3, TO_CHAR($4::date, 'YYYY-MM-DD'), TO_CHAR($5::date, 'YYYY-MM-DD'), $6, $7, $8, $9, $10, $11)
		`, tx.id, tx.amount, tx.description, tx.date, tx.date, tx.txType, tx.payTo, tx.paid, tx.enteredBy, tx.optional, tx.userId)

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
	setupReportTestDB(t)
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
			name: "All paid transactions, exclude optional",
			filter: models.ReportFilter{
				Paid:     boolPtr(true),
				Optional: boolPtr(true), // true = exclude optional transactions
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
			expectedCount: 3,         // Housing, Food, and Misc (optional)
			expectedTotal: 305.00,    // 50 + 75 + 150 + 30
			expectedFirst: "Housing", // Highest total in this date range
		},
		// Note: We don't have a way to filter for only unpaid transactions in the UI
		// The paid checkbox is for "show only paid" not "show only unpaid"
		{
			name:   "All transactions (no filters)",
			filter: models.ReportFilter{
				// No paid or optional filter - should show all transactions
			},
			expectedCount: 4,         // Food, Housing, Fun, Misc categories
			expectedTotal: 745.00,    // All transactions including unpaid and optional
			expectedFirst: "Housing", // Housing has highest total: 430.00 (350 paid + 80 unpaid)
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request with filter
			filterJSON, _ := json.Marshal(tc.filter)
			req := httptest.NewRequest("GET", "/reports/splits", bytes.NewBuffer(filterJSON))
			req.Header.Set("Content-Type", "application/json")
			req = MockAuthContext(req, testUserID)

			// Create response recorder
			w := httptest.NewRecorder()

			// Call handler
			GetYNABSplits(w, req)

			// Check response
			if w.Code != http.StatusOK {
				t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
				return
			}

			// Parse response
			var response []models.CategoryTotal
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Errorf("Failed to decode response: %v", err)
				return
			}

			// Check number of categories
			if len(response) != tc.expectedCount {
				t.Errorf("Expected %d categories, got %d", tc.expectedCount, len(response))
			}

			// Check first category name
			if len(response) > 0 && response[0].Category != tc.expectedFirst {
				t.Errorf("Expected first category to be %s, got %s", tc.expectedFirst, response[0].Category)
			}

			// Check total amount
			var total float64
			for _, cat := range response {
				total += cat.Total
			}
			if total != tc.expectedTotal {
				t.Errorf("Expected total around %f, got %f", tc.expectedTotal, total)
			}
		})
	}
}

// Helper function to create a bool pointer
func boolPtr(b bool) *bool {
	return &b
}
