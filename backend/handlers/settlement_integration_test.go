package handlers

import (
	"bennwallet/backend/middleware"
	"bennwallet/backend/testutil"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
)

// TestSettlementWorkflow_CompleteFlow tests the entire settlement workflow from creation to completion
func TestSettlementWorkflow_CompleteFlow(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()
	handler := NewSettlementHandler(db)

	// Create test users
	aliceID := "alice-123"
	bobID := "bob-456"

	_, err := db.Exec(`
		INSERT INTO users (id, username, name, role, status, is_admin) 
		VALUES ($1, 'alice', 'Alice Smith', 'user', 'active', false),
			   ($2, 'bob', 'Bob Johnson', 'user', 'active', false)
		ON CONFLICT (id) DO NOTHING
	`, aliceID, bobID)
	if err != nil {
		t.Fatalf("Failed to create test users: %v", err)
	}

	// Scenario: Alice paid for dinner ($120), Bob owes Alice
	// Later: Alice paid for groceries ($80), Bob owes Alice
	// Bob creates a settlement to pay Alice back for both transactions

	// Create first transaction - Alice paid for dinner, Bob owes
	var dinnerTxID string
	err = db.QueryRow(`
		INSERT INTO transactions (id, amount, description, date, transaction_date, type, entered, entered_by, user_id, paid_by, owed_by, paid)
		VALUES (uuid_generate_v4(), 120.00, 'Dinner at restaurant', TO_CHAR(NOW(), 'YYYY-MM-DD'), TO_CHAR(NOW(), 'YYYY-MM-DD'), 'expense', NOW(), $1, $1, $1, $2, false)
		RETURNING id
	`, aliceID, bobID).Scan(&dinnerTxID)
	if err != nil {
		t.Fatalf("Failed to create dinner transaction: %v", err)
	}

	// Create second transaction - Alice paid for groceries, Bob owes
	var groceriesTxID string
	err = db.QueryRow(`
		INSERT INTO transactions (id, amount, description, date, transaction_date, type, entered, entered_by, user_id, paid_by, owed_by, paid)
		VALUES (uuid_generate_v4(), 80.00, 'Groceries', TO_CHAR(NOW(), 'YYYY-MM-DD'), TO_CHAR(NOW(), 'YYYY-MM-DD'), 'expense', NOW(), $1, $1, $1, $2, false)
		RETURNING id
	`, aliceID, bobID).Scan(&groceriesTxID)
	if err != nil {
		t.Fatalf("Failed to create groceries transaction: %v", err)
	}

	// Step 1: Bob creates a settlement using the dinner transaction
	createReq := map[string]interface{}{
		"transactionId": dinnerTxID,
		"notes":         "Paying Alice back for shared expenses",
	}

	bodyBytes, _ := json.Marshal(createReq)
	req := httptest.NewRequest("POST", "/settlements", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, bobID)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.CreateSettlement(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to create settlement: status %d, body: %s", w.Code, w.Body.String())
	}

	var settlement map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &settlement); err != nil {
		t.Fatalf("Failed to unmarshal settlement response: %v", err)
	}

	settlementID := settlement["id"].(string)
	if settlement["totalAmount"].(float64) != 120.00 {
		t.Errorf("Expected initial total amount 120.00, got %f", settlement["totalAmount"])
	}

	// Step 2: Bob applies the groceries transaction to the same settlement
	applyReq := map[string]interface{}{
		"transactionId": groceriesTxID,
		"amount":        80.00,
	}

	bodyBytes, _ = json.Marshal(applyReq)
	req = httptest.NewRequest("POST", "/settlements/"+settlementID+"/apply", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	ctx = context.WithValue(req.Context(), middleware.UserIDKey, bobID)
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": settlementID})

	w = httptest.NewRecorder()
	handler.ApplyTransaction(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to apply transaction to settlement: status %d, body: %s", w.Code, w.Body.String())
	}

	if err := json.Unmarshal(w.Body.Bytes(), &settlement); err != nil {
		t.Fatalf("Failed to unmarshal updated settlement: %v", err)
	}

	if settlement["totalAmount"].(float64) != 200.00 {
		t.Errorf("Expected total amount 200.00 after applying second transaction, got %f", settlement["totalAmount"])
	}

	// Step 3: Verify both transactions are marked as paid
	var dinnerPaid, groceriesPaid bool
	err = db.QueryRow("SELECT paid FROM transactions WHERE id = $1", dinnerTxID).Scan(&dinnerPaid)
	if err != nil {
		t.Fatalf("Failed to check dinner transaction paid status: %v", err)
	}
	err = db.QueryRow("SELECT paid FROM transactions WHERE id = $1", groceriesTxID).Scan(&groceriesPaid)
	if err != nil {
		t.Fatalf("Failed to check groceries transaction paid status: %v", err)
	}

	if !dinnerPaid {
		t.Errorf("Expected dinner transaction to be marked as paid")
	}
	if !groceriesPaid {
		t.Errorf("Expected groceries transaction to be marked as paid")
	}

	// Step 4: Alice completes the settlement (simulating payment received)
	completeReq := map[string]interface{}{
		"status": "completed",
		"notes":  "Payment received via Venmo",
	}

	bodyBytes, _ = json.Marshal(completeReq)
	req = httptest.NewRequest("PATCH", "/settlements/"+settlementID, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	ctx = context.WithValue(req.Context(), middleware.UserIDKey, aliceID)
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": settlementID})

	w = httptest.NewRecorder()
	handler.UpdateSettlementStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to complete settlement: status %d, body: %s", w.Code, w.Body.String())
	}

	if err := json.Unmarshal(w.Body.Bytes(), &settlement); err != nil {
		t.Fatalf("Failed to unmarshal completed settlement: %v", err)
	}

	if settlement["status"].(string) != "completed" {
		t.Errorf("Expected settlement status to be completed, got %s", settlement["status"])
	}

	if settlement["completedAt"] == nil {
		t.Errorf("Expected completedAt to be set for completed settlement")
	}

	// Step 5: Verify settlement appears in both users' settlement lists
	// Check Alice's settlements
	req = httptest.NewRequest("GET", "/settlements", nil)
	ctx = context.WithValue(req.Context(), middleware.UserIDKey, aliceID)
	req = req.WithContext(ctx)

	w = httptest.NewRecorder()
	handler.GetUserSettlements(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to get Alice's settlements: status %d", w.Code)
	}

	var aliceSettlements []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &aliceSettlements); err != nil {
		t.Fatalf("Failed to unmarshal Alice's settlements: %v", err)
	}

	foundSettlement := false
	for _, s := range aliceSettlements {
		if s["id"].(string) == settlementID {
			foundSettlement = true
			break
		}
	}

	if !foundSettlement {
		t.Errorf("Expected Alice to see the settlement in her list")
	}

	// Check Bob's settlements
	req = httptest.NewRequest("GET", "/settlements", nil)
	ctx = context.WithValue(req.Context(), middleware.UserIDKey, bobID)
	req = req.WithContext(ctx)

	w = httptest.NewRecorder()
	handler.GetUserSettlements(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to get Bob's settlements: status %d", w.Code)
	}

	var bobSettlements []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &bobSettlements); err != nil {
		t.Fatalf("Failed to unmarshal Bob's settlements: %v", err)
	}

	foundSettlement = false
	for _, s := range bobSettlements {
		if s["id"].(string) == settlementID {
			foundSettlement = true
			break
		}
	}

	if !foundSettlement {
		t.Errorf("Expected Bob to see the settlement in his list")
	}
}

// TestSettlementWorkflow_CancellationFlow tests the cancellation workflow
func TestSettlementWorkflow_CancellationFlow(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()
	handler := NewSettlementHandler(db)

	// Create test users
	aliceID := "alice-123"
	bobID := "bob-456"

	_, err := db.Exec(`
		INSERT INTO users (id, username, name, role, status, is_admin) 
		VALUES ($1, 'alice', 'Alice Smith', 'user', 'active', false),
			   ($2, 'bob', 'Bob Johnson', 'user', 'active', false)
		ON CONFLICT (id) DO NOTHING
	`, aliceID, bobID)
	if err != nil {
		t.Fatalf("Failed to create test users: %v", err)
	}

	// Create a transaction - Alice paid for dinner, Bob owes
	var txID string
	err = db.QueryRow(`
		INSERT INTO transactions (id, amount, description, date, transaction_date, type, entered, entered_by, user_id, paid_by, owed_by, paid)
		VALUES (uuid_generate_v4(), 120.00, 'Dinner at restaurant', TO_CHAR(NOW(), 'YYYY-MM-DD'), TO_CHAR(NOW(), 'YYYY-MM-DD'), 'expense', NOW(), $1, $1, $1, $2, false)
		RETURNING id
	`, aliceID, bobID).Scan(&txID)
	if err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	// Bob creates a settlement
	createReq := map[string]interface{}{
		"transactionId": txID,
		"notes":         "Paying Alice back for dinner",
	}

	bodyBytes, _ := json.Marshal(createReq)
	req := httptest.NewRequest("POST", "/settlements", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, bobID)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.CreateSettlement(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to create settlement: status %d, body: %s", w.Code, w.Body.String())
	}

	var settlement map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &settlement); err != nil {
		t.Fatalf("Failed to unmarshal settlement response: %v", err)
	}

	settlementID := settlement["id"].(string)

	// Bob cancels the settlement
	cancelReq := map[string]interface{}{
		"status": "cancelled",
		"notes":  "Changed mind about payment method",
	}

	bodyBytes, _ = json.Marshal(cancelReq)
	req = httptest.NewRequest("PATCH", "/settlements/"+settlementID, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	ctx = context.WithValue(req.Context(), middleware.UserIDKey, bobID)
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": settlementID})

	w = httptest.NewRecorder()
	handler.UpdateSettlementStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to cancel settlement: status %d, body: %s", w.Code, w.Body.String())
	}

	if err := json.Unmarshal(w.Body.Bytes(), &settlement); err != nil {
		t.Fatalf("Failed to unmarshal cancelled settlement: %v", err)
	}

	if settlement["status"].(string) != "cancelled" {
		t.Errorf("Expected settlement status to be cancelled, got %s", settlement["status"])
	}

	// Verify transaction is no longer marked as paid
	var paid bool
	err = db.QueryRow("SELECT paid FROM transactions WHERE id = $1", txID).Scan(&paid)
	if err != nil {
		t.Fatalf("Failed to check transaction paid status: %v", err)
	}

	if paid {
		t.Errorf("Expected transaction to be marked as unpaid after settlement cancellation")
	}
}

// TestSettlementWorkflow_RemoveTransactionFlow tests removing transactions from settlements
func TestSettlementWorkflow_RemoveTransactionFlow(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()
	handler := NewSettlementHandler(db)

	// Create test users
	aliceID := "alice-123"
	bobID := "bob-456"

	_, err := db.Exec(`
		INSERT INTO users (id, username, name, role, status, is_admin) 
		VALUES ($1, 'alice', 'Alice Smith', 'user', 'active', false),
			   ($2, 'bob', 'Bob Johnson', 'user', 'active', false)
		ON CONFLICT (id) DO NOTHING
	`, aliceID, bobID)
	if err != nil {
		t.Fatalf("Failed to create test users: %v", err)
	}

	// Create two transactions
	var tx1ID, tx2ID string
	err = db.QueryRow(`
		INSERT INTO transactions (id, amount, description, date, transaction_date, type, entered, entered_by, user_id, paid_by, owed_by, paid)
		VALUES (uuid_generate_v4(), 120.00, 'Dinner', TO_CHAR(NOW(), 'YYYY-MM-DD'), TO_CHAR(NOW(), 'YYYY-MM-DD'), 'expense', NOW(), $1, $1, $1, $2, false)
		RETURNING id
	`, aliceID, bobID).Scan(&tx1ID)
	if err != nil {
		t.Fatalf("Failed to create first transaction: %v", err)
	}

	err = db.QueryRow(`
		INSERT INTO transactions (id, amount, description, date, transaction_date, type, entered, entered_by, user_id, paid_by, owed_by, paid)
		VALUES (uuid_generate_v4(), 80.00, 'Groceries', TO_CHAR(NOW(), 'YYYY-MM-DD'), TO_CHAR(NOW(), 'YYYY-MM-DD'), 'expense', NOW(), $1, $1, $1, $2, false)
		RETURNING id
	`, aliceID, bobID).Scan(&tx2ID)
	if err != nil {
		t.Fatalf("Failed to create second transaction: %v", err)
	}

	// Bob creates a settlement with both transactions
	createReq := map[string]interface{}{
		"transactionId": tx1ID,
		"notes":         "Paying Alice back",
	}

	bodyBytes, _ := json.Marshal(createReq)
	req := httptest.NewRequest("POST", "/settlements", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, bobID)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.CreateSettlement(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to create settlement: status %d, body: %s", w.Code, w.Body.String())
	}

	var settlement map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &settlement); err != nil {
		t.Fatalf("Failed to unmarshal settlement response: %v", err)
	}

	settlementID := settlement["id"].(string)

	// Add second transaction
	applyReq := map[string]interface{}{
		"transactionId": tx2ID,
		"amount":        80.00,
	}

	bodyBytes, _ = json.Marshal(applyReq)
	req = httptest.NewRequest("POST", "/settlements/"+settlementID+"/apply", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	ctx = context.WithValue(req.Context(), middleware.UserIDKey, bobID)
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": settlementID})

	w = httptest.NewRecorder()
	handler.ApplyTransaction(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to apply second transaction: status %d, body: %s", w.Code, w.Body.String())
	}

	// Remove first transaction
	req = httptest.NewRequest("DELETE", "/settlements/"+settlementID+"/transactions/"+tx1ID, nil)
	ctx = context.WithValue(req.Context(), middleware.UserIDKey, bobID)
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{
		"id":            settlementID,
		"transactionId": tx1ID,
	})

	w = httptest.NewRecorder()
	handler.RemoveTransaction(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to remove transaction: status %d, body: %s", w.Code, w.Body.String())
	}

	// Verify settlement amount is updated
	req = httptest.NewRequest("GET", "/settlements/"+settlementID, nil)
	ctx = context.WithValue(req.Context(), middleware.UserIDKey, bobID)
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": settlementID})

	w = httptest.NewRecorder()
	handler.GetSettlement(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to get settlement: status %d", w.Code)
	}

	if err := json.Unmarshal(w.Body.Bytes(), &settlement); err != nil {
		t.Fatalf("Failed to unmarshal settlement: %v", err)
	}

	if settlement["totalAmount"].(float64) != 80.00 {
		t.Errorf("Expected total amount 80.00 after removing transaction, got %f", settlement["totalAmount"])
	}

	// Verify first transaction is no longer marked as paid
	var paid bool
	err = db.QueryRow("SELECT paid FROM transactions WHERE id = $1", tx1ID).Scan(&paid)
	if err != nil {
		t.Fatalf("Failed to check transaction paid status: %v", err)
	}

	if paid {
		t.Errorf("Expected removed transaction to be marked as unpaid")
	}
}

// TestSettlementWorkflow_ConcurrentOperations tests handling of concurrent operations
func TestSettlementWorkflow_ConcurrentOperations(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()
	handler := NewSettlementHandler(db)

	// Create test users
	aliceID := "alice-123"
	bobID := "bob-456"

	_, err := db.Exec(`
		INSERT INTO users (id, username, name, role, status, is_admin) 
		VALUES ($1, 'alice', 'Alice Smith', 'user', 'active', false),
			   ($2, 'bob', 'Bob Johnson', 'user', 'active', false)
		ON CONFLICT (id) DO NOTHING
	`, aliceID, bobID)
	if err != nil {
		t.Fatalf("Failed to create test users: %v", err)
	}

	// Create multiple transactions
	var txIDs []string
	for i := 0; i < 3; i++ {
		var txID string
		err = db.QueryRow(`
			INSERT INTO transactions (id, amount, description, date, transaction_date, type, entered, entered_by, user_id, paid_by, owed_by, paid)
			VALUES (uuid_generate_v4(), 100.00, 'Test transaction', TO_CHAR(NOW(), 'YYYY-MM-DD'), TO_CHAR(NOW(), 'YYYY-MM-DD'), 'expense', NOW(), $1, $1, $1, $2, false)
			RETURNING id
		`, aliceID, bobID).Scan(&txID)
		if err != nil {
			t.Fatalf("Failed to create transaction %d: %v", i+1, err)
		}
		txIDs = append(txIDs, txID)
	}

	// Bob creates a settlement with first transaction
	createReq := map[string]interface{}{
		"transactionId": txIDs[0],
		"notes":         "Paying Alice back",
	}

	bodyBytes, _ := json.Marshal(createReq)
	req := httptest.NewRequest("POST", "/settlements", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, bobID)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.CreateSettlement(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to create settlement: status %d, body: %s", w.Code, w.Body.String())
	}

	var settlement map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &settlement); err != nil {
		t.Fatalf("Failed to unmarshal settlement response: %v", err)
	}

	settlementID := settlement["id"].(string)

	// Add second transaction
	applyReq := map[string]interface{}{
		"transactionId": txIDs[1],
		"amount":        100.00,
	}

	bodyBytes, _ = json.Marshal(applyReq)
	req = httptest.NewRequest("POST", "/settlements/"+settlementID+"/apply", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	ctx = context.WithValue(req.Context(), middleware.UserIDKey, bobID)
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": settlementID})

	w = httptest.NewRecorder()
	handler.ApplyTransaction(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to apply second transaction: status %d, body: %s", w.Code, w.Body.String())
	}

	// Try to add third transaction while completing settlement
	done := make(chan bool)
	go func() {
		// Add third transaction
		applyReq := map[string]interface{}{
			"transactionId": txIDs[2],
			"amount":        100.00,
		}

		bodyBytes, _ := json.Marshal(applyReq)
		req := httptest.NewRequest("POST", "/settlements/"+settlementID+"/apply", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, bobID)
		req = req.WithContext(ctx)
		req = mux.SetURLVars(req, map[string]string{"id": settlementID})

		w := httptest.NewRecorder()
		handler.ApplyTransaction(w, req)
		done <- true
	}()

	// Complete settlement
	completeReq := map[string]interface{}{
		"status": "completed",
		"notes":  "Payment received",
	}

	bodyBytes, _ = json.Marshal(completeReq)
	req = httptest.NewRequest("PATCH", "/settlements/"+settlementID, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	ctx = context.WithValue(req.Context(), middleware.UserIDKey, aliceID)
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": settlementID})

	w = httptest.NewRecorder()
	handler.UpdateSettlementStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to complete settlement: status %d, body: %s", w.Code, w.Body.String())
	}

	// Wait for concurrent operation to complete
	<-done

	// Verify final state
	req = httptest.NewRequest("GET", "/settlements/"+settlementID, nil)
	ctx = context.WithValue(req.Context(), middleware.UserIDKey, bobID)
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": settlementID})

	w = httptest.NewRecorder()
	handler.GetSettlement(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to get settlement: status %d", w.Code)
	}

	if err := json.Unmarshal(w.Body.Bytes(), &settlement); err != nil {
		t.Fatalf("Failed to unmarshal settlement: %v", err)
	}

	if settlement["status"].(string) != "completed" {
		t.Errorf("Expected settlement status to be completed, got %s", settlement["status"])
	}

	// Verify all transactions are marked as paid
	for _, txID := range txIDs {
		var paid bool
		err = db.QueryRow("SELECT paid FROM transactions WHERE id = $1", txID).Scan(&paid)
		if err != nil {
			t.Fatalf("Failed to check transaction %s paid status: %v", txID, err)
		}
		if !paid {
			t.Errorf("Expected transaction %s to be marked as paid", txID)
		}
	}
}
