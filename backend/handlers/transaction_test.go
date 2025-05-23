package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bennwallet/backend/database"
	"bennwallet/backend/models"

	_ "github.com/lib/pq"
)

func setupTransactionTestDB() {
	// Create a test database connection
	db, err := SetupPostgresTestDB()
	if err != nil {
		panic(err)
	}
	database.DB = db

	// Check if tables already exist to avoid duplicate type errors
	var txTableExists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'transactions')").Scan(&txTableExists)
	if err != nil {
		panic(err)
	}

	// Only create tables if they don't exist
	if !txTableExists {
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
	}

	// Check if transaction_categories table exists
	var tcTableExists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'transaction_categories')").Scan(&tcTableExists)
	if err != nil {
		panic(err)
	}

	if !tcTableExists {
		// Create transaction_categories table if it doesn't exist
		_, err = db.Exec(`
			CREATE TABLE IF NOT EXISTS transaction_categories (
				id SERIAL PRIMARY KEY,
				transaction_id TEXT NOT NULL,
				category_id TEXT NOT NULL,
				amount NUMERIC(15,2),
				UNIQUE(transaction_id, category_id)
			)
		`)
		if err != nil {
			panic(err)
		}
	}

	// Use the helper function to seed YNAB test data with proper groups and categories
	err = SeedYNABTestData(db)
	if err != nil {
		panic(fmt.Errorf("failed to seed YNAB test data: %w", err))
	}
}

func TestAddTransaction(t *testing.T) {
	setupTransactionTestDB()
	defer database.DB.Close()

	// Setup
	now := time.Now()
	txDate := now.AddDate(0, 0, -3) // Set the transaction date 3 days before entry

	reqBody := models.Transaction{
		Amount:          100.50,
		Description:     "Test Transaction",
		Date:            now,
		TransactionDate: txDate,
		Type:            "Test",
		PayTo:           "Test Payee",
		Paid:            true,
		PaidDate:        now.Format("2006-01-02"),
		EnteredBy:       "test-user",
		Optional:        false,
	}

	jsonBody, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/transactions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	// Use our helper to add authentication
	req = MockAuthContext(req, TestUserID)
	w := httptest.NewRecorder()

	// Execute
	AddTransaction(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	var response models.Transaction
	err := json.NewDecoder(w.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Error decoding response: %v", err)
	}

	// Verify transaction was created in database
	var count int
	err = database.DB.QueryRow("SELECT COUNT(*) FROM transactions WHERE description = $1", reqBody.Description).Scan(&count)
	if err != nil {
		t.Fatalf("Error checking transaction: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 transaction, got %d", count)
	}

	// Verify transaction date was stored correctly
	var storedTxDate time.Time
	err = database.DB.QueryRow("SELECT transaction_date FROM transactions WHERE description = $1", reqBody.Description).Scan(&storedTxDate)
	if err != nil {
		t.Fatalf("Error checking transaction date: %v", err)
	}

	// Format both dates to YYYY-MM-DD to compare just the date component
	expectedDateStr := txDate.Format("2006-01-02")
	storedDateStr := storedTxDate.Format("2006-01-02")
	if storedDateStr != expectedDateStr {
		t.Errorf("Expected transaction date %s, got %s", expectedDateStr, storedDateStr)
	}

	// Verify user ID was set from auth context
	var userID string
	err = database.DB.QueryRow("SELECT user_id FROM transactions WHERE description = $1", reqBody.Description).Scan(&userID)
	if err != nil {
		t.Fatalf("Error checking transaction userId: %v", err)
	}

	if userID != TestUserID {
		t.Errorf("Expected userId '%s', got '%s'", TestUserID, userID)
	}
}

func TestGetTransactions(t *testing.T) {
	setupTransactionTestDB()
	defer database.DB.Close()

	// First add a test transaction
	_, err := database.DB.Exec(`
		INSERT INTO transactions (id, amount, description, date, type, pay_to, paid, paid_date, entered_by, optional, user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, "test-id", 100.50, "Test Transaction", time.Now(), "Test", "Test Payee", true, time.Now().Format("2006-01-02"), "test-user", false, TestUserID)
	if err != nil {
		t.Fatal(err)
	}

	// Setup request with authentication
	req := MockAuthContext(httptest.NewRequest("GET", "/transactions", nil), TestUserID)
	w := httptest.NewRecorder()

	// Execute
	GetTransactions(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	var response []models.Transaction
	err = json.NewDecoder(w.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Error decoding response: %v", err)
	}

	// Verify we got the transaction we created
	if len(response) != 1 {
		t.Errorf("Expected 1 transaction, got %d", len(response))
	}

	if response[0].Description != "Test Transaction" {
		t.Errorf("Expected description 'Test Transaction', got '%s'", response[0].Description)
	}
}
