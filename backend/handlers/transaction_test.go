package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bennwallet/backend/database"
	"bennwallet/backend/models"
)

func setupTransactionTestDB() {
	// Create a test database connection
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		panic(err)
	}
	database.DB = db

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
	// Add mock authentication
	req = MockAuthContext(req, "test-user-id")
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
	err = database.DB.QueryRow("SELECT COUNT(*) FROM transactions WHERE description = ?", reqBody.Description).Scan(&count)
	if err != nil {
		t.Fatalf("Error checking transaction: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 transaction, got %d", count)
	}

	// Verify transaction date was stored correctly
	var storedTxDate time.Time
	err = database.DB.QueryRow("SELECT transaction_date FROM transactions WHERE description = ?", reqBody.Description).Scan(&storedTxDate)
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
	err = database.DB.QueryRow("SELECT userId FROM transactions WHERE description = ?", reqBody.Description).Scan(&userID)
	if err != nil {
		t.Fatalf("Error checking transaction userId: %v", err)
	}

	if userID != "test-user-id" {
		t.Errorf("Expected userId 'test-user-id', got '%s'", userID)
	}
}

func TestGetTransactions(t *testing.T) {
	setupTransactionTestDB()
	defer database.DB.Close()

	// First add a test transaction
	_, err := database.DB.Exec(`
		INSERT INTO transactions (id, amount, description, date, type, payTo, paid, paidDate, enteredBy, optional, userId)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "test-id", 100.50, "Test Transaction", time.Now(), "Test", "Test Payee", true, time.Now().Format("2006-01-02"), "test-user", false, "test-user-id")
	if err != nil {
		t.Fatal(err)
	}

	// Setup
	req := httptest.NewRequest("GET", "/transactions", nil)
	// Add mock authentication
	req = MockAuthContext(req, "test-user-id")
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
