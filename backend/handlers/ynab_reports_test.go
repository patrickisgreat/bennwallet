package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bennwallet/backend/database"
	"bennwallet/backend/middleware"
	"bennwallet/backend/models"
	"bennwallet/backend/testutil"

	_ "github.com/lib/pq"
)

const testUserID = "test-user-id"

func setupReportTestDB(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	database.DB = db
	t.Cleanup(cleanup)

	insertTestTransactions()
}

func insertTestTransactions() {
	dateFormat := "2006-01-02"

	startDate, _ := time.Parse(dateFormat, "2023-01-01")
	midDate, _ := time.Parse(dateFormat, "2023-02-15")
	endDate, _ := time.Parse(dateFormat, "2023-03-31")

	_, err := database.DB.Exec(`
		INSERT INTO users (id, username, name, role, status, is_admin) 
		VALUES 
		($1, 'testuser', 'Test User', 'user', 'active', false),
		('Patrick', 'patrick-test', 'Patrick', 'user', 'active', false),
		('Sarah', 'sarah-test', 'Sarah', 'user', 'active', false)
		ON CONFLICT (id) DO NOTHING
	`, testUserID)
	if err != nil {
		panic(err)
	}

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

	_, err = database.DB.Exec(`
		DELETE FROM transaction_categories;
		DELETE FROM transactions;
	`)
	if err != nil {
		panic(err)
	}

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
		{"tx4", 75.00, "Groceries 2", midDate, "Food", "Sarah", true, "Patrick", false, testUserID, "cat-test-user-id-Food"},
		{"tx3", 200.00, "Rent", endDate, "Housing", "Sarah", true, "Sarah", false, testUserID, "cat-test-user-id-Housing"},
		{"tx5", 150.00, "Utilities", midDate, "Housing", "Patrick", true, "Patrick", false, testUserID, "cat-test-user-id-Housing"},
		{"tx8", 80.00, "Unpaid Bill", midDate, "Bills", "Sarah", false, "Patrick", false, testUserID, "cat-test-user-id-Housing"},
		{"tx6", 60.00, "Entertainment", endDate, "Fun", "Sarah", true, "Sarah", false, testUserID, "cat-test-user-id-Fun"},
		{"tx7", 30.00, "Optional Expense", midDate, "Misc", "Patrick", true, "Sarah", true, testUserID, "cat-test-user-id-Misc"},
	}

	for _, tx := range testTransactions {
		owedBy := tx.enteredBy
		if tx.payTo == tx.enteredBy {
			owedBy = tx.payTo
		}

		_, err := database.DB.Exec(`
			INSERT INTO transactions 
			(id, amount, description, date, transaction_date, type, paid_by, owed_by, paid, entered_by, optional, user_id)
			VALUES ($1, $2, $3, TO_CHAR($4::date, 'YYYY-MM-DD'), TO_CHAR($5::date, 'YYYY-MM-DD'), $6, $7, $8, $9, $10, $11, $12)
		`, tx.id, tx.amount, tx.description, tx.date, tx.date, tx.txType, tx.payTo, owedBy, tx.paid, tx.enteredBy, tx.optional, tx.userId)

		if err != nil {
			panic(err)
		}

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
		expectedFirst string
	}{
		{
			name: "All paid transactions, exclude optional",
			filter: models.ReportFilter{
				Paid:     boolPtr(true),
				Optional: boolPtr(true),
			},
			expectedCount: 3,
			expectedTotal: 635.00,
			expectedFirst: "Housing",
		},
		{
			name: "Food category only",
			filter: models.ReportFilter{
				Category: "Food",
				Paid:     boolPtr(true),
			},
			expectedCount: 1,
			expectedTotal: 225.00,
			expectedFirst: "Food",
		},
		{
			name: "Paid by Patrick",
			filter: models.ReportFilter{
				EnteredBy: "Patrick",
				Paid:      boolPtr(true),
			},
			expectedCount: 3,
			expectedTotal: 230.00,
			expectedFirst: "Housing",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.filter)
			req := httptest.NewRequest("POST", "/reports/ynab-splits", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(setTestUserContext(req.Context()))

			w := httptest.NewRecorder()
			GetYNABSplits(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
				t.Logf("Response body: %s", w.Body.String())
				return
			}

			var results []models.CategoryTotal
			if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			if len(results) != tc.expectedCount {
				t.Errorf("Expected %d categories, got %d", tc.expectedCount, len(results))
				for i, result := range results {
					t.Logf("Result %d: %s = %.2f", i, result.Category, result.Total)
				}
				return
			}

			if len(results) > 0 && results[0].Category != tc.expectedFirst {
				t.Errorf("Expected first category to be %s, got %s", tc.expectedFirst, results[0].Category)
			}

			var actualTotal float64
			for _, result := range results {
				actualTotal += result.Total
			}

			if actualTotal != tc.expectedTotal {
				t.Errorf("Expected total %.2f, got %.2f", tc.expectedTotal, actualTotal)
			}
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func setTestUserContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, middleware.UserIDKey, testUserID)
}
