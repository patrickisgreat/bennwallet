package handlers

import (
	"bennwallet/backend/middleware"
	"bennwallet/backend/testutil"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

func TestSettlementHandler_CreateSettlement(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()
	handler := NewSettlementHandler(db)

	// Create test users
	testUserID1 := "test-user-1"
	testUserID2 := "test-user-2"

	_, err := db.Exec(`
		INSERT INTO users (id, username, name, role, status, is_admin) 
		VALUES ($1, 'testuser1', 'Test User 1', 'user', 'active', false),
			   ($2, 'testuser2', 'Test User 2', 'user', 'active', false)
		ON CONFLICT (id) DO NOTHING
	`, testUserID1, testUserID2)
	if err != nil {
		t.Fatalf("Failed to create test users: %v", err)
	}

	// Create a test transaction
	var txID string
	err = db.QueryRow(`
		INSERT INTO transactions (id, amount, description, date, transaction_date, type, entered, entered_by, user_id, paid_by, owed_by, paid)
		VALUES (uuid_generate_v4(), 100.50, 'Test transaction', TO_CHAR(NOW(), 'YYYY-MM-DD'), TO_CHAR(NOW(), 'YYYY-MM-DD'), 'expense', NOW(), $1, $1, $1, $2, false)
		RETURNING id
	`, testUserID1, testUserID2).Scan(&txID)
	if err != nil {
		t.Fatalf("Failed to create test transaction: %v", err)
	}

	tests := []struct {
		name           string
		userID         string
		requestBody    map[string]interface{}
		expectedStatus int
		expectBody     bool
	}{
		{
			name:   "Valid settlement creation",
			userID: testUserID1,
			requestBody: map[string]interface{}{
				"transactionId": txID,
				"notes":         "Test settlement",
			},
			expectedStatus: http.StatusOK,
			expectBody:     true,
		},
		{
			name:   "Missing transaction ID",
			userID: testUserID1,
			requestBody: map[string]interface{}{
				"notes": "Missing transaction ID",
			},
			expectedStatus: http.StatusBadRequest,
			expectBody:     false,
		},
		{
			name:   "Invalid transaction ID",
			userID: testUserID1,
			requestBody: map[string]interface{}{
				"transactionId": "non-existent",
				"notes":         "Invalid transaction",
			},
			expectedStatus: http.StatusBadRequest,
			expectBody:     false,
		},
		{
			name:           "No user in context",
			userID:         "", // Empty userID simulates missing context
			requestBody:    map[string]interface{}{"transactionId": txID},
			expectedStatus: http.StatusUnauthorized,
			expectBody:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request body
			bodyBytes, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", "/settlements", bytes.NewBuffer(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			// Add user ID to context if provided
			if tt.userID != "" {
				ctx := context.WithValue(req.Context(), middleware.UserIDKey, tt.userID)
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()

			handler.CreateSettlement(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectBody {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Errorf("Failed to unmarshal response: %v", err)
				}

				// Check that response contains settlement data
				if response["id"] == nil {
					t.Errorf("Expected settlement ID in response")
				}

				if response["status"] != "active" {
					t.Errorf("Expected active status, got %v", response["status"])
				}
			}
		})
	}
}

func TestSettlementHandler_GetUserSettlements(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()
	handler := NewSettlementHandler(db)

	// Create test users
	testUserID1 := "test-user-1"
	testUserID2 := "test-user-2"

	_, err := db.Exec(`
		INSERT INTO users (id, username, name, role, status, is_admin) 
		VALUES ($1, 'testuser1', 'Test User 1', 'user', 'active', false),
			   ($2, 'testuser2', 'Test User 2', 'user', 'active', false)
		ON CONFLICT (id) DO NOTHING
	`, testUserID1, testUserID2)
	if err != nil {
		t.Fatalf("Failed to create test users: %v", err)
	}

	// Create test transactions and settlements
	var txID string
	err = db.QueryRow(`
		INSERT INTO transactions (id, amount, description, date, transaction_date, type, entered, entered_by, user_id, paid_by, owed_by, paid)
		VALUES (uuid_generate_v4(), 100.0, 'Test transaction', TO_CHAR(NOW(), 'YYYY-MM-DD'), TO_CHAR(NOW(), 'YYYY-MM-DD'), 'expense', NOW(), $1, $1, $1, $2, false)
		RETURNING id
	`, testUserID1, testUserID2).Scan(&txID)
	if err != nil {
		t.Fatalf("Failed to create test transaction: %v", err)
	}

	// Create a settlement
	createReq := map[string]interface{}{
		"transactionId": txID,
		"notes":         "Test settlement",
	}

	bodyBytes, _ := json.Marshal(createReq)
	req := httptest.NewRequest("POST", "/settlements", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, testUserID1)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.CreateSettlement(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to create test settlement: %v", w.Body.String())
	}

	tests := []struct {
		name           string
		userID         string
		expectedStatus int
		expectBody     bool
	}{
		{
			name:           "Get settlements for valid user",
			userID:         testUserID1,
			expectedStatus: http.StatusOK,
			expectBody:     true,
		},
		{
			name:           "No user in context",
			userID:         "",
			expectedStatus: http.StatusUnauthorized,
			expectBody:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/settlements", nil)

			// Add user ID to context if provided
			if tt.userID != "" {
				ctx := context.WithValue(req.Context(), middleware.UserIDKey, tt.userID)
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()

			handler.GetUserSettlements(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectBody {
				var response []map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Errorf("Failed to unmarshal response: %v", err)
				}

				if len(response) == 0 {
					t.Errorf("Expected non-empty settlements list")
				}
			}
		})
	}
}

func TestSettlementHandler_ApplyTransaction(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()
	handler := NewSettlementHandler(db)

	// Create test users
	testUserID1 := "test-user-1"
	testUserID2 := "test-user-2"

	_, err := db.Exec(`
		INSERT INTO users (id, username, name, role, status, is_admin) 
		VALUES ($1, 'testuser1', 'Test User 1', 'user', 'active', false),
			   ($2, 'testuser2', 'Test User 2', 'user', 'active', false)
		ON CONFLICT (id) DO NOTHING
	`, testUserID1, testUserID2)
	if err != nil {
		t.Fatalf("Failed to create test users: %v", err)
	}

	// Create test transactions
	var tx1ID, tx2ID string
	err = db.QueryRow(`
		INSERT INTO transactions (id, amount, description, date, transaction_date, type, entered, entered_by, user_id, paid_by, owed_by, paid)
		VALUES (uuid_generate_v4(), 100.0, 'First transaction', TO_CHAR(NOW(), 'YYYY-MM-DD'), TO_CHAR(NOW(), 'YYYY-MM-DD'), 'expense', NOW(), $1, $1, $1, $2, false)
		RETURNING id
	`, testUserID1, testUserID2).Scan(&tx1ID)
	if err != nil {
		t.Fatalf("Failed to create first test transaction: %v", err)
	}

	err = db.QueryRow(`
		INSERT INTO transactions (id, amount, description, date, transaction_date, type, entered, entered_by, user_id, paid_by, owed_by, paid)
		VALUES (uuid_generate_v4(), 50.0, 'Second transaction', TO_CHAR(NOW(), 'YYYY-MM-DD'), TO_CHAR(NOW(), 'YYYY-MM-DD'), 'expense', NOW(), $1, $1, $1, $2, false)
		RETURNING id
	`, testUserID1, testUserID2).Scan(&tx2ID)
	if err != nil {
		t.Fatalf("Failed to create second test transaction: %v", err)
	}

	// Create a settlement with first transaction
	createReq := map[string]interface{}{
		"transactionId": tx1ID,
		"notes":         "Test settlement",
	}

	bodyBytes, _ := json.Marshal(createReq)
	req := httptest.NewRequest("POST", "/settlements", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, testUserID1)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.CreateSettlement(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to create test settlement: %v", w.Body.String())
	}

	var settlement map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &settlement); err != nil {
		t.Fatalf("Failed to unmarshal settlement response: %v", err)
	}

	settlementID := settlement["id"].(string)

	tests := []struct {
		name           string
		userID         string
		settlementID   string
		requestBody    map[string]interface{}
		expectedStatus int
		expectBody     bool
	}{
		{
			name:         "Valid transaction application",
			userID:       testUserID1,
			settlementID: settlementID,
			requestBody: map[string]interface{}{
				"transactionId": tx2ID,
				"amount":        50.0,
			},
			expectedStatus: http.StatusOK,
			expectBody:     true,
		},
		{
			name:         "Invalid settlement ID",
			userID:       testUserID1,
			settlementID: "non-existent",
			requestBody: map[string]interface{}{
				"transactionId": tx2ID,
				"amount":        50.0,
			},
			expectedStatus: http.StatusBadRequest,
			expectBody:     false,
		},
		{
			name:         "Invalid transaction ID",
			userID:       testUserID1,
			settlementID: settlementID,
			requestBody: map[string]interface{}{
				"transactionId": "non-existent",
				"amount":        50.0,
			},
			expectedStatus: http.StatusBadRequest,
			expectBody:     false,
		},
		{
			name:         "No user in context",
			userID:       "",
			settlementID: settlementID,
			requestBody: map[string]interface{}{
				"transactionId": tx2ID,
				"amount":        50.0,
			},
			expectedStatus: http.StatusUnauthorized,
			expectBody:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyBytes, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", "/settlements/"+tt.settlementID+"/apply", bytes.NewBuffer(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			if tt.userID != "" {
				ctx := context.WithValue(req.Context(), middleware.UserIDKey, tt.userID)
				req = req.WithContext(ctx)
			}

			req = mux.SetURLVars(req, map[string]string{"id": tt.settlementID})

			w := httptest.NewRecorder()

			handler.ApplyTransaction(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectBody {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Errorf("Failed to unmarshal response: %v", err)
				}

				if response["totalAmount"].(float64) != 150.0 {
					t.Errorf("Expected total amount 150.0, got %f", response["totalAmount"].(float64))
				}
			}
		})
	}
}

func TestSettlementHandler_UpdateSettlementStatus(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()
	handler := NewSettlementHandler(db)

	// Create test users
	testUserID1 := "test-user-1"
	testUserID2 := "test-user-2"

	_, err := db.Exec(`
		INSERT INTO users (id, username, name, role, status, is_admin) 
		VALUES ($1, 'testuser1', 'Test User 1', 'user', 'active', false),
			   ($2, 'testuser2', 'Test User 2', 'user', 'active', false)
		ON CONFLICT (id) DO NOTHING
	`, testUserID1, testUserID2)
	if err != nil {
		t.Fatalf("Failed to create test users: %v", err)
	}

	// Create test transaction
	var txID string
	err = db.QueryRow(`
		INSERT INTO transactions (id, amount, description, date, transaction_date, type, entered, entered_by, user_id, paid_by, owed_by, paid)
		VALUES (uuid_generate_v4(), 100.0, 'Test transaction', TO_CHAR(NOW(), 'YYYY-MM-DD'), TO_CHAR(NOW(), 'YYYY-MM-DD'), 'expense', NOW(), $1, $1, $1, $2, false)
		RETURNING id
	`, testUserID1, testUserID2).Scan(&txID)
	if err != nil {
		t.Fatalf("Failed to create test transaction: %v", err)
	}

	// Create a settlement
	createReq := map[string]interface{}{
		"transactionId": txID,
		"notes":         "Test settlement",
	}

	bodyBytes, _ := json.Marshal(createReq)
	req := httptest.NewRequest("POST", "/settlements", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, testUserID1)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.CreateSettlement(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to create test settlement: %v", w.Body.String())
	}

	var settlement map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &settlement); err != nil {
		t.Fatalf("Failed to unmarshal settlement response: %v", err)
	}

	settlementID := settlement["id"].(string)

	tests := []struct {
		name           string
		userID         string
		settlementID   string
		requestBody    map[string]interface{}
		expectedStatus int
		expectBody     bool
	}{
		{
			name:         "Valid status update",
			userID:       testUserID1,
			settlementID: settlementID,
			requestBody: map[string]interface{}{
				"status": "completed",
				"notes":  "Payment received",
			},
			expectedStatus: http.StatusOK,
			expectBody:     true,
		},
		{
			name:         "Invalid settlement ID",
			userID:       testUserID1,
			settlementID: "non-existent",
			requestBody: map[string]interface{}{
				"status": "completed",
				"notes":  "Payment received",
			},
			expectedStatus: http.StatusNotFound,
			expectBody:     false,
		},
		{
			name:         "Invalid status",
			userID:       testUserID1,
			settlementID: settlementID,
			requestBody: map[string]interface{}{
				"status": "invalid",
				"notes":  "Invalid status",
			},
			expectedStatus: http.StatusBadRequest,
			expectBody:     false,
		},
		{
			name:         "No user in context",
			userID:       "",
			settlementID: settlementID,
			requestBody: map[string]interface{}{
				"status": "completed",
				"notes":  "Payment received",
			},
			expectedStatus: http.StatusUnauthorized,
			expectBody:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyBytes, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("PATCH", "/settlements/"+tt.settlementID, bytes.NewBuffer(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			if tt.userID != "" {
				ctx := context.WithValue(req.Context(), middleware.UserIDKey, tt.userID)
				req = req.WithContext(ctx)
			}

			req = mux.SetURLVars(req, map[string]string{"id": tt.settlementID})

			w := httptest.NewRecorder()

			handler.UpdateSettlementStatus(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectBody {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Errorf("Failed to unmarshal response: %v", err)
				}

				if response["status"].(string) != "completed" {
					t.Errorf("Expected status completed, got %s", response["status"].(string))
				}
			}
		})
	}
}

func TestSettlementHandler_RemoveTransaction(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()
	handler := NewSettlementHandler(db)

	// Create test users
	testUserID1 := "test-user-1"
	testUserID2 := "test-user-2"

	_, err := db.Exec(`
		INSERT INTO users (id, username, name, role, status, is_admin) 
		VALUES ($1, 'testuser1', 'Test User 1', 'user', 'active', false),
			   ($2, 'testuser2', 'Test User 2', 'user', 'active', false)
		ON CONFLICT (id) DO NOTHING
	`, testUserID1, testUserID2)
	if err != nil {
		t.Fatalf("Failed to create test users: %v", err)
	}

	// Create test transactions
	var tx1ID, tx2ID string
	err = db.QueryRow(`
		INSERT INTO transactions (id, amount, description, date, transaction_date, type, entered, entered_by, user_id, paid_by, owed_by, paid)
		VALUES (uuid_generate_v4(), 100.0, 'First transaction', TO_CHAR(NOW(), 'YYYY-MM-DD'), TO_CHAR(NOW(), 'YYYY-MM-DD'), 'expense', NOW(), $1, $1, $1, $2, false)
		RETURNING id
	`, testUserID1, testUserID2).Scan(&tx1ID)
	if err != nil {
		t.Fatalf("Failed to create first test transaction: %v", err)
	}

	err = db.QueryRow(`
		INSERT INTO transactions (id, amount, description, date, transaction_date, type, entered, entered_by, user_id, paid_by, owed_by, paid)
		VALUES (uuid_generate_v4(), 50.0, 'Second transaction', TO_CHAR(NOW(), 'YYYY-MM-DD'), TO_CHAR(NOW(), 'YYYY-MM-DD'), 'expense', NOW(), $1, $1, $1, $2, false)
		RETURNING id
	`, testUserID1, testUserID2).Scan(&tx2ID)
	if err != nil {
		t.Fatalf("Failed to create second test transaction: %v", err)
	}

	// Create a settlement with both transactions
	createReq := map[string]interface{}{
		"transactionId": tx1ID,
		"notes":         "Test settlement",
	}

	bodyBytes, _ := json.Marshal(createReq)
	req := httptest.NewRequest("POST", "/settlements", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, testUserID1)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.CreateSettlement(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to create test settlement: %v", w.Body.String())
	}

	var settlement map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &settlement); err != nil {
		t.Fatalf("Failed to unmarshal settlement response: %v", err)
	}

	settlementID := settlement["id"].(string)

	// Add second transaction
	applyReq := map[string]interface{}{
		"transactionId": tx2ID,
		"amount":        50.0,
	}

	bodyBytes, _ = json.Marshal(applyReq)
	req = httptest.NewRequest("POST", "/settlements/"+settlementID+"/apply", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	ctx = context.WithValue(req.Context(), middleware.UserIDKey, testUserID1)
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": settlementID})

	w = httptest.NewRecorder()
	handler.ApplyTransaction(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to apply second transaction: %v", w.Body.String())
	}

	tests := []struct {
		name           string
		userID         string
		settlementID   string
		transactionID  string
		expectedStatus int
		expectBody     bool
	}{
		{
			name:           "Valid transaction removal",
			userID:         testUserID1,
			settlementID:   settlementID,
			transactionID:  tx2ID,
			expectedStatus: http.StatusOK,
			expectBody:     true,
		},
		{
			name:           "Invalid settlement ID",
			userID:         testUserID1,
			settlementID:   "non-existent",
			transactionID:  tx2ID,
			expectedStatus: http.StatusBadRequest,
			expectBody:     false,
		},
		{
			name:           "Invalid transaction ID",
			userID:         testUserID1,
			settlementID:   settlementID,
			transactionID:  "non-existent",
			expectedStatus: http.StatusBadRequest,
			expectBody:     false,
		},
		{
			name:           "No user in context",
			userID:         "",
			settlementID:   settlementID,
			transactionID:  tx2ID,
			expectedStatus: http.StatusUnauthorized,
			expectBody:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("DELETE", "/settlements/"+tt.settlementID+"/transactions/"+tt.transactionID, nil)

			if tt.userID != "" {
				ctx := context.WithValue(req.Context(), middleware.UserIDKey, tt.userID)
				req = req.WithContext(ctx)
			}

			req = mux.SetURLVars(req, map[string]string{
				"id":            tt.settlementID,
				"transactionId": tt.transactionID,
			})

			w := httptest.NewRecorder()

			handler.RemoveTransaction(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectBody {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Errorf("Failed to unmarshal response: %v", err)
				}

				if response["totalAmount"].(float64) != 100.0 {
					t.Errorf("Expected total amount 100.0, got %f", response["totalAmount"].(float64))
				}
			}
		})
	}
}

func TestSettlementHandler_InvalidJSONRequests(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()
	handler := NewSettlementHandler(db)

	tests := []struct {
		name           string
		method         string
		path           string
		body           string
		expectedStatus int
	}{
		{
			name:           "Invalid JSON in create request",
			method:         "POST",
			path:           "/settlements",
			body:           "{invalid json}",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Invalid JSON in apply request",
			method:         "POST",
			path:           "/settlements/123/apply",
			body:           "{invalid json}",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Invalid JSON in update request",
			method:         "PATCH",
			path:           "/settlements/123",
			body:           "{invalid json}",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			// Add authentication context to test JSON parsing
			ctx := context.WithValue(req.Context(), middleware.UserIDKey, "test-user")
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()

			switch tt.method {
			case "POST":
				if strings.Contains(tt.path, "/apply") {
					req = mux.SetURLVars(req, map[string]string{"id": "123"})
					handler.ApplyTransaction(w, req)
				} else {
					handler.CreateSettlement(w, req)
				}
			case "PATCH":
				req = mux.SetURLVars(req, map[string]string{"id": "123"})
				handler.UpdateSettlementStatus(w, req)
			}

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}
