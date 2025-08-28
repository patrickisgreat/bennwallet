package handlers

import (
	"bennwallet/backend/middleware"
	"bennwallet/backend/testutil"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

// TestSettlementComprehensive tests all settlement functionality comprehensively
func TestSettlementComprehensive(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()
	handler := NewSettlementHandler(db)

	// Test user IDs
	aliceID := "alice-comprehensive-test"
	bobID := "bob-comprehensive-test"
	charlieID := "charlie-comprehensive-test"

	// Create test users
	createTestUsers(t, db, aliceID, bobID, charlieID)

	// Create test transactions for comprehensive testing
	transactions := createTestTransactions(t, db, aliceID, bobID, charlieID)

	t.Run("Single Transaction Settlement", func(t *testing.T) {
		testSingleTransactionSettlement(t, handler, transactions, aliceID, bobID)
	})

	t.Run("Multi Transaction Settlement", func(t *testing.T) {
		testMultiTransactionSettlement(t, handler, transactions, aliceID, bobID)
	})

	t.Run("Settlement Modifications", func(t *testing.T) {
		testSettlementModifications(t, handler, transactions, aliceID, bobID)
	})

	t.Run("Settlement Status Changes", func(t *testing.T) {
		testSettlementStatusChanges(t, handler, transactions, aliceID, bobID, charlieID, db)
	})

	t.Run("Settlement Edge Cases", func(t *testing.T) {
		testSettlementEdgeCases(t, handler, transactions, aliceID, bobID, charlieID)
	})

	t.Run("Settlement Permissions", func(t *testing.T) {
		testSettlementPermissions(t, handler, transactions, aliceID, bobID, charlieID)
	})

	t.Run("Settlement Queries", func(t *testing.T) {
		testSettlementQueries(t, handler, transactions, aliceID, bobID)
	})
}

func createTestUsers(t *testing.T, db *sql.DB, userIDs ...string) {
	users := []struct {
		ID       string
		Username string
		Name     string
	}{
		{userIDs[0], "alice_comp", "Alice Comprehensive"},
		{userIDs[1], "bob_comp", "Bob Comprehensive"},
		{userIDs[2], "charlie_comp", "Charlie Comprehensive"},
	}

	for _, user := range users {
		_, err := db.Exec(`
			INSERT INTO users (id, username, name, role, status, is_admin) 
			VALUES ($1, $2, $3, 'user', 'active', false)
			ON CONFLICT (id) DO NOTHING
		`, user.ID, user.Username, user.Name)
		if err != nil {
			t.Fatalf("Failed to create test user %s: %v", user.Name, err)
		}
	}
}

type TestTransaction struct {
	ID          string
	Amount      float64
	Description string
	PaidBy      string
	OwedBy      string
	EnteredBy   string
}

func createTestTransactions(t *testing.T, db *sql.DB, aliceID, bobID, charlieID string) map[string]TestTransaction {
	transactions := map[string]TestTransaction{
		// Alice paid, Bob owes
		"dinner1": {
			Amount:      120.00,
			Description: "Dinner at Italian place",
			PaidBy:      aliceID,
			OwedBy:      bobID,
			EnteredBy:   aliceID,
		},
		"groceries1": {
			Amount:      85.50,
			Description: "Weekly groceries",
			PaidBy:      aliceID,
			OwedBy:      bobID,
			EnteredBy:   aliceID,
		},
		"gas1": {
			Amount:      45.00,
			Description: "Gas for road trip",
			PaidBy:      aliceID,
			OwedBy:      bobID,
			EnteredBy:   aliceID,
		},
		// Bob paid, Alice owes
		"movie1": {
			Amount:      30.00,
			Description: "Movie tickets",
			PaidBy:      bobID,
			OwedBy:      aliceID,
			EnteredBy:   bobID,
		},
		"coffee1": {
			Amount:      15.75,
			Description: "Coffee meeting",
			PaidBy:      bobID,
			OwedBy:      aliceID,
			EnteredBy:   bobID,
		},
		// Alice paid, Charlie owes
		"lunch1": {
			Amount:      25.00,
			Description: "Business lunch",
			PaidBy:      aliceID,
			OwedBy:      charlieID,
			EnteredBy:   aliceID,
		},
		// Bob paid, Charlie owes
		"uber1": {
			Amount:      18.50,
			Description: "Uber to airport",
			PaidBy:      bobID,
			OwedBy:      charlieID,
			EnteredBy:   bobID,
		},
		// Large amounts for testing
		"rent1": {
			Amount:      800.00,
			Description: "Monthly rent split",
			PaidBy:      aliceID,
			OwedBy:      bobID,
			EnteredBy:   aliceID,
		},
		"vacation1": {
			Amount:      350.00,
			Description: "Vacation accommodation",
			PaidBy:      bobID,
			OwedBy:      aliceID,
			EnteredBy:   bobID,
		},
	}

	// Insert transactions into database
	for key, tx := range transactions {
		var txID string
		err := db.QueryRow(`
			INSERT INTO transactions 
			(id, amount, description, date, transaction_date, type, entered, entered_by, user_id, paid_by, owed_by, paid)
			VALUES (uuid_generate_v4(), $1, $2, TO_CHAR(NOW(), 'YYYY-MM-DD'), TO_CHAR(NOW(), 'YYYY-MM-DD'), 'expense', NOW(), $3, $3, $4, $5, false)
			RETURNING id
		`, tx.Amount, tx.Description, tx.EnteredBy, tx.PaidBy, tx.OwedBy).Scan(&txID)

		if err != nil {
			t.Fatalf("Failed to create transaction %s: %v", key, err)
		}

		// Update the transaction with the generated ID
		tx.ID = txID
		transactions[key] = tx
	}

	return transactions
}

func testSingleTransactionSettlement(t *testing.T, handler *SettlementHandler, transactions map[string]TestTransaction, aliceID, bobID string) {
	t.Run("Create Single Transaction Settlement", func(t *testing.T) {
		dinner := transactions["dinner1"]

		// Bob creates a settlement for dinner (he owes Alice)
		createReq := map[string]interface{}{
			"transactionId": dinner.ID,
			"notes":         "Paying Alice back for dinner",
		}

		settlement := makeSettlementRequest(t, handler.CreateSettlement, "POST", "/settlements", createReq, bobID)

		// Verify settlement was created correctly
		if settlement["totalAmount"].(float64) != dinner.Amount {
			t.Errorf("Expected total amount %.2f, got %.2f", dinner.Amount, settlement["totalAmount"].(float64))
		}

		if settlement["status"].(string) != "active" {
			t.Errorf("Expected status 'active', got '%s'", settlement["status"].(string))
		}

		// Verify transaction is marked as paid
		verifyTransactionPaid(t, handler.db, dinner.ID, true)

		// Store settlement ID for later tests
		settlementID := settlement["id"].(string)

		// Test getting the settlement
		retrievedSettlement := makeSettlementRequest(t, handler.GetSettlement, "GET", "/settlements/"+settlementID, nil, bobID)
		if retrievedSettlement["id"].(string) != settlementID {
			t.Errorf("Retrieved settlement ID mismatch")
		}
	})
}

func testMultiTransactionSettlement(t *testing.T, handler *SettlementHandler, transactions map[string]TestTransaction, aliceID, bobID string) {
	t.Run("Create Multi Transaction Settlement", func(t *testing.T) {
		groceries := transactions["groceries1"]
		gas := transactions["gas1"]
		rent := transactions["rent1"]

		// Bob creates a settlement for multiple transactions (he owes Alice)
		createReq := map[string]interface{}{
			"transactionIds": []string{groceries.ID, gas.ID, rent.ID},
			"notes":          "Paying Alice back for multiple expenses",
		}

		settlement := makeSettlementRequest(t, handler.CreateSettlement, "POST", "/settlements", createReq, bobID)

		expectedTotal := groceries.Amount + gas.Amount + rent.Amount
		if settlement["totalAmount"].(float64) != expectedTotal {
			t.Errorf("Expected total amount %.2f, got %.2f", expectedTotal, settlement["totalAmount"].(float64))
		}

		// Verify all transactions are marked as paid
		verifyTransactionPaid(t, handler.db, groceries.ID, true)
		verifyTransactionPaid(t, handler.db, gas.ID, true)
		verifyTransactionPaid(t, handler.db, rent.ID, true)

		// Verify settlement has correct number of items
		items := settlement["items"].([]interface{})
		if len(items) != 3 {
			t.Errorf("Expected 3 settlement items, got %d", len(items))
		}
	})

	t.Run("Invalid Multi Transaction Settlement", func(t *testing.T) {
		lunch := transactions["lunch1"] // Alice paid, Charlie owes
		movie := transactions["movie1"] // Bob paid, Alice owes

		// Try to create settlement with transactions involving different users
		createReq := map[string]interface{}{
			"transactionIds": []string{lunch.ID, movie.ID}, // Different user combinations
			"notes":          "Invalid multi-transaction settlement",
		}

		// This should fail
		w := makeSettlementRequestRaw(t, handler.CreateSettlement, "POST", "/settlements", createReq, aliceID)
		if w.Code == http.StatusOK {
			t.Errorf("Expected settlement creation to fail for transactions with different users")
		}
	})
}

func testSettlementModifications(t *testing.T, handler *SettlementHandler, transactions map[string]TestTransaction, aliceID, bobID string) {
	t.Run("Add and Remove Transactions", func(t *testing.T) {
		// Create initial settlement with one transaction
		movie := transactions["movie1"]
		createReq := map[string]interface{}{
			"transactionId": movie.ID,
			"notes":         "Movie night payment",
		}

		settlement := makeSettlementRequest(t, handler.CreateSettlement, "POST", "/settlements", createReq, aliceID)
		settlementID := settlement["id"].(string)

		// Add another transaction
		coffee := transactions["coffee1"]
		applyReq := map[string]interface{}{
			"transactionId": coffee.ID,
			"amount":        coffee.Amount,
		}

		updatedSettlement := makeSettlementRequestWithID(t, handler.ApplyTransaction, "POST", "/settlements/{id}/apply", applyReq, aliceID, settlementID)

		expectedTotal := movie.Amount + coffee.Amount
		if updatedSettlement["totalAmount"].(float64) != expectedTotal {
			t.Errorf("Expected total amount %.2f after adding transaction, got %.2f", expectedTotal, updatedSettlement["totalAmount"].(float64))
		}

		// Remove the coffee transaction
		w := makeSettlementRequestRawWithIDs(t, handler.RemoveTransaction, "DELETE", "/settlements/{id}/transactions/{transactionId}", nil, aliceID, settlementID, coffee.ID)
		if w.Code != http.StatusOK {
			t.Errorf("Failed to remove transaction: status %d, body: %s", w.Code, w.Body.String())
		}

		// Verify coffee transaction is no longer paid
		verifyTransactionPaid(t, handler.db, coffee.ID, false)

		// Get updated settlement
		finalSettlement := makeSettlementRequest(t, handler.GetSettlement, "GET", "/settlements/"+settlementID, nil, aliceID)
		if finalSettlement["totalAmount"].(float64) != movie.Amount {
			t.Errorf("Expected total amount %.2f after removing transaction, got %.2f", movie.Amount, finalSettlement["totalAmount"].(float64))
		}
	})
}

func testSettlementStatusChanges(t *testing.T, handler *SettlementHandler, transactions map[string]TestTransaction, aliceID, bobID, charlieID string, db *sql.DB) {
	t.Run("Complete Settlement", func(t *testing.T) {
		// Create settlement
		vacation := transactions["vacation1"]
		createReq := map[string]interface{}{
			"transactionId": vacation.ID,
			"notes":         "Vacation expense settlement",
		}

		settlement := makeSettlementRequest(t, handler.CreateSettlement, "POST", "/settlements", createReq, aliceID)
		settlementID := settlement["id"].(string)

		// Complete the settlement (Bob marks as completed - he paid)
		statusReq := map[string]interface{}{
			"status": "completed",
			"notes":  "Payment received via Venmo",
		}

		completedSettlement := makeSettlementRequestWithID(t, handler.UpdateSettlementStatus, "PUT", "/settlements/{id}/status", statusReq, bobID, settlementID)

		if completedSettlement["status"].(string) != "completed" {
			t.Errorf("Expected status 'completed', got '%s'", completedSettlement["status"].(string))
		}

		if completedSettlement["completedAt"] == nil {
			t.Errorf("Expected completedAt to be set")
		}
	})

	t.Run("Cancel Settlement", func(t *testing.T) {
		// Create settlement using a transaction between Bob and Charlie
		// First need to create a transaction between them
		var uberTxID string
		err := db.QueryRow(`
			INSERT INTO transactions 
			(id, amount, description, date, transaction_date, type, entered, entered_by, user_id, paid_by, owed_by, paid)
			VALUES (uuid_generate_v4(), $1, $2, TO_CHAR(NOW(), 'YYYY-MM-DD'), TO_CHAR(NOW(), 'YYYY-MM-DD'), 'expense', NOW(), $3, $3, $4, $5, false)
			RETURNING id
		`, 25.00, "Uber for test cancellation", bobID, bobID, charlieID).Scan(&uberTxID)
		if err != nil {
			t.Fatalf("Failed to create test transaction: %v", err)
		}

		createReq := map[string]interface{}{
			"transactionId": uberTxID,
			"notes":         "Uber ride settlement",
		}

		settlement := makeSettlementRequest(t, handler.CreateSettlement, "POST", "/settlements", createReq, charlieID)
		settlementID := settlement["id"].(string)

		// Cancel the settlement
		statusReq := map[string]interface{}{
			"status": "cancelled",
			"notes":  "Different payment method used",
		}

		cancelledSettlement := makeSettlementRequestWithID(t, handler.UpdateSettlementStatus, "PUT", "/settlements/{id}/status", statusReq, charlieID, settlementID)

		if cancelledSettlement["status"].(string) != "cancelled" {
			t.Errorf("Expected status 'cancelled', got '%s'", cancelledSettlement["status"].(string))
		}

		// Verify transaction is no longer marked as paid
		verifyTransactionPaid(t, handler.db, uberTxID, false)
	})
}

func testSettlementEdgeCases(t *testing.T, handler *SettlementHandler, transactions map[string]TestTransaction, aliceID, bobID, charlieID string) {
	t.Run("Empty Transaction List", func(t *testing.T) {
		createReq := map[string]interface{}{
			"transactionIds": []string{},
			"notes":          "Empty transaction list",
		}

		w := makeSettlementRequestRaw(t, handler.CreateSettlement, "POST", "/settlements", createReq, aliceID)
		if w.Code == http.StatusOK {
			t.Errorf("Expected settlement creation to fail for empty transaction list")
		}
	})

	t.Run("Nonexistent Transaction", func(t *testing.T) {
		createReq := map[string]interface{}{
			"transactionId": "nonexistent-transaction-id",
			"notes":         "Nonexistent transaction",
		}

		w := makeSettlementRequestRaw(t, handler.CreateSettlement, "POST", "/settlements", createReq, aliceID)
		if w.Code == http.StatusOK {
			t.Errorf("Expected settlement creation to fail for nonexistent transaction")
		}
	})

	t.Run("User Not Involved in Transaction", func(t *testing.T) {
		lunch := transactions["lunch1"] // Alice paid, Charlie owes

		// Bob tries to create settlement for transaction he's not involved in
		createReq := map[string]interface{}{
			"transactionId": lunch.ID,
			"notes":         "Transaction I'm not involved in",
		}

		w := makeSettlementRequestRaw(t, handler.CreateSettlement, "POST", "/settlements", createReq, bobID)
		if w.Code == http.StatusOK {
			t.Errorf("Expected settlement creation to fail when user not involved in transaction")
		}
	})

	t.Run("Already Paid Transaction", func(t *testing.T) {
		// Mark a transaction as paid
		lunch := transactions["lunch1"]
		_, err := handler.db.Exec("UPDATE transactions SET paid = true WHERE id = $1", lunch.ID)
		if err != nil {
			t.Fatalf("Failed to mark transaction as paid: %v", err)
		}

		// Try to create settlement with already paid transaction
		createReq := map[string]interface{}{
			"transactionId": lunch.ID,
			"notes":         "Already paid transaction",
		}

		// This should work but may have different behavior
		settlement := makeSettlementRequest(t, handler.CreateSettlement, "POST", "/settlements", createReq, charlieID)

		// Verify settlement was created (the system should handle paid transactions)
		if settlement["totalAmount"].(float64) != lunch.Amount {
			t.Errorf("Expected total amount %.2f, got %.2f", lunch.Amount, settlement["totalAmount"].(float64))
		}
	})
}

func testSettlementPermissions(t *testing.T, handler *SettlementHandler, transactions map[string]TestTransaction, aliceID, bobID, charlieID string) {
	t.Run("Settlement Access Permissions", func(t *testing.T) {
		// Alice creates a settlement with Bob
		movie := transactions["movie1"]
		createReq := map[string]interface{}{
			"transactionId": movie.ID,
			"notes":         "Movie settlement",
		}

		settlement := makeSettlementRequest(t, handler.CreateSettlement, "POST", "/settlements", createReq, aliceID)
		settlementID := settlement["id"].(string)

		// Charlie (not involved) tries to access the settlement
		w := makeSettlementRequestRaw(t, handler.GetSettlement, "GET", "/settlements/"+settlementID, nil, charlieID)
		// This should work as getting settlements doesn't restrict access in the current implementation

		// Charlie tries to modify the settlement
		statusReq := map[string]interface{}{
			"status": "cancelled",
			"notes":  "Unauthorized cancellation",
		}

		w = makeSettlementRequestRawWithID(t, handler.UpdateSettlementStatus, "PUT", "/settlements/{id}/status", statusReq, charlieID, settlementID)
		if w.Code == http.StatusOK {
			t.Errorf("Expected status update to fail for unauthorized user")
		}
	})
}

func testSettlementQueries(t *testing.T, handler *SettlementHandler, transactions map[string]TestTransaction, aliceID, bobID string) {
	t.Run("Get User Settlements", func(t *testing.T) {
		// Create a few settlements for Alice
		createReq1 := map[string]interface{}{
			"transactionId": transactions["movie1"].ID,
			"notes":         "Movie settlement 1",
		}
		makeSettlementRequest(t, handler.CreateSettlement, "POST", "/settlements", createReq1, aliceID)

		createReq2 := map[string]interface{}{
			"transactionId": transactions["coffee1"].ID,
			"notes":         "Coffee settlement 1",
		}
		makeSettlementRequest(t, handler.CreateSettlement, "POST", "/settlements", createReq2, aliceID)

		// Get Alice's settlements
		w := makeSettlementRequestRaw(t, handler.GetUserSettlements, "GET", "/settlements", nil, aliceID)
		if w.Code != http.StatusOK {
			t.Fatalf("Failed to get user settlements: status %d", w.Code)
		}

		var settlements []map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &settlements); err != nil {
			t.Fatalf("Failed to unmarshal settlements: %v", err)
		}

		if len(settlements) < 2 {
			t.Errorf("Expected at least 2 settlements for Alice, got %d", len(settlements))
		}
	})

	t.Run("Get Available Settlement Transactions", func(t *testing.T) {
		// First, create some additional transactions specifically for this test
		// that haven't been consumed by previous tests
		var additionalTxID1, additionalTxID2 string

		// Create transaction where Alice paid and Bob owes
		err := handler.db.QueryRow(`
			INSERT INTO transactions 
			(id, amount, description, date, transaction_date, type, entered, entered_by, user_id, paid_by, owed_by, paid)
			VALUES (uuid_generate_v4(), $1, $2, TO_CHAR(NOW(), 'YYYY-MM-DD'), TO_CHAR(NOW(), 'YYYY-MM-DD'), 'expense', NOW(), $3, $3, $4, $5, false)
			RETURNING id
		`, 75.00, "Test transaction for available check", aliceID, aliceID, bobID).Scan(&additionalTxID1)
		if err != nil {
			t.Fatalf("Failed to create additional transaction 1: %v", err)
		}

		// Create transaction where Bob paid and Alice owes
		err = handler.db.QueryRow(`
			INSERT INTO transactions 
			(id, amount, description, date, transaction_date, type, entered, entered_by, user_id, paid_by, owed_by, paid)
			VALUES (uuid_generate_v4(), $1, $2, TO_CHAR(NOW(), 'YYYY-MM-DD'), TO_CHAR(NOW(), 'YYYY-MM-DD'), 'expense', NOW(), $3, $3, $4, $5, false)
			RETURNING id
		`, 42.50, "Another test transaction for available check", bobID, bobID, aliceID).Scan(&additionalTxID2)
		if err != nil {
			t.Fatalf("Failed to create additional transaction 2: %v", err)
		}

		// Create a settlement using one of the additional transactions
		createReq := map[string]interface{}{
			"transactionId": additionalTxID1,
			"notes":         "Settlement for available test",
		}

		settlement := makeSettlementRequest(t, handler.CreateSettlement, "POST", "/settlements", createReq, aliceID)
		settlementID := settlement["id"].(string)

		// Get available transactions for this settlement
		w := makeSettlementRequestRawWithID(t, handler.GetAvailableSettlementTransactions, "GET", "/settlements/{id}/available-transactions", nil, aliceID, settlementID)
		if w.Code != http.StatusOK {
			t.Fatalf("Failed to get available transactions: status %d", w.Code)
		}

		var availableTransactions []map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &availableTransactions); err != nil {
			t.Fatalf("Failed to unmarshal available transactions: %v", err)
		}

		// Should have at least one available transaction (the second additional transaction)
		if len(availableTransactions) == 0 {
			t.Errorf("Expected some available transactions for settlement, got 0")
		} else {
			// Verify that the second additional transaction is in the available list
			found := false
			for _, tx := range availableTransactions {
				if tx["id"].(string) == additionalTxID2 {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected to find additional transaction %s in available transactions", additionalTxID2)
			}
		}
	})

	t.Run("Get Transaction Settlements", func(t *testing.T) {
		// Get settlements for a specific transaction
		vacation := transactions["vacation1"]
		w := makeSettlementRequestRawWithID(t, handler.GetTransactionSettlements, "GET", "/transactions/{transactionId}/settlements", nil, aliceID, vacation.ID)

		if w.Code != http.StatusOK {
			t.Fatalf("Failed to get transaction settlements: status %d", w.Code)
		}

		var settlements []map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &settlements); err != nil {
			t.Fatalf("Failed to unmarshal transaction settlements: %v", err)
		}

		// Should find the settlement we created earlier
		if len(settlements) == 0 {
			t.Errorf("Expected to find settlements for transaction")
		}
	})
}

// Helper functions

func makeSettlementRequest(t *testing.T, handlerFunc http.HandlerFunc, method, path string, body interface{}, userID string) map[string]interface{} {
	w := makeSettlementRequestRaw(t, handlerFunc, method, path, body, userID)

	if w.Code != http.StatusOK {
		t.Fatalf("Request failed: status %d, body: %s", w.Code, w.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	return result
}

func makeSettlementRequestWithID(t *testing.T, handlerFunc http.HandlerFunc, method, path string, body interface{}, userID, settlementID string) map[string]interface{} {
	w := makeSettlementRequestRawWithID(t, handlerFunc, method, path, body, userID, settlementID)

	if w.Code != http.StatusOK {
		t.Fatalf("Request failed: status %d, body: %s", w.Code, w.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	return result
}

func makeSettlementRequestRaw(t *testing.T, handlerFunc http.HandlerFunc, method, path string, body interface{}, userID string) *httptest.ResponseRecorder {
	var reqBody []byte
	if body != nil {
		var err error
		reqBody, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("Failed to marshal request body: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, bytes.NewBuffer(reqBody))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handlerFunc(w, req)

	return w
}

func makeSettlementRequestRawWithID(t *testing.T, handlerFunc http.HandlerFunc, method, pathTemplate string, body interface{}, userID, settlementID string) *httptest.ResponseRecorder {
	var reqBody []byte
	if body != nil {
		var err error
		reqBody, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("Failed to marshal request body: %v", err)
		}
	}

	path := strings.ReplaceAll(pathTemplate, "{id}", settlementID)
	path = strings.ReplaceAll(path, "{transactionId}", settlementID)

	req := httptest.NewRequest(method, path, bytes.NewBuffer(reqBody))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
	req = req.WithContext(ctx)
	vars := map[string]string{"id": settlementID}
	if strings.Contains(pathTemplate, "{transactionId}") {
		vars = map[string]string{"transactionId": settlementID}
	}
	req = mux.SetURLVars(req, vars)

	w := httptest.NewRecorder()
	handlerFunc(w, req)

	return w
}

func makeSettlementRequestRawWithIDs(t *testing.T, handlerFunc http.HandlerFunc, method, pathTemplate string, body interface{}, userID, settlementID, transactionID string) *httptest.ResponseRecorder {
	var reqBody []byte
	if body != nil {
		var err error
		reqBody, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("Failed to marshal request body: %v", err)
		}
	}

	req := httptest.NewRequest(method, pathTemplate, bytes.NewBuffer(reqBody))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{
		"id":            settlementID,
		"transactionId": transactionID,
	})

	w := httptest.NewRecorder()
	handlerFunc(w, req)

	return w
}

func verifyTransactionPaid(t *testing.T, db *sql.DB, transactionID string, expectedPaid bool) {
	var paid bool
	err := db.QueryRow("SELECT paid FROM transactions WHERE id = $1", transactionID).Scan(&paid)
	if err != nil {
		t.Fatalf("Failed to check transaction paid status: %v", err)
	}

	if paid != expectedPaid {
		t.Errorf("Expected transaction %s paid status to be %v, got %v", transactionID, expectedPaid, paid)
	}
}
