package services

import (
	"bennwallet/backend/testutil"
	"testing"
)

func TestSettlementService_CreateSettlement(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()
	service := NewSettlementService(db)

	// Create test users
	testUserID1 := "test-user-1"
	testUserID2 := "test-user-2"

	_, err := db.Exec(`
		INSERT INTO users (id, username, name, role, status, is_admin) 
		VALUES ($1, 'testuser1', 'Test User 1', 'user', 'active', false),
			   ($2, 'testuser2', 'Test User 2', 'user', 'active', false)
	`, testUserID1, testUserID2)
	if err != nil {
		t.Fatalf("Failed to create test users: %v", err)
	}

	// Create a test transaction where user1 paid and user2 owes
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
		transactionID  string
		notes          string
		expectError    bool
		expectedStatus string
	}{
		{
			name:           "Create settlement as creditor (user who paid)",
			userID:         testUserID1,
			transactionID:  txID,
			notes:          "Settlement for dinner",
			expectError:    false,
			expectedStatus: "active",
		},
		{
			name:          "Invalid transaction ID",
			userID:        testUserID1,
			transactionID: "non-existent",
			notes:         "This should fail",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settlement, err := service.CreateSettlement(tt.userID, tt.transactionID, tt.notes)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if settlement == nil {
				t.Errorf("Expected settlement but got nil")
				return
			}

			if settlement.Status != tt.expectedStatus {
				t.Errorf("Expected status %s, got %s", tt.expectedStatus, settlement.Status)
			}

			if settlement.Notes != tt.notes {
				t.Errorf("Expected notes %s, got %s", tt.notes, settlement.Notes)
			}

			if settlement.CreatorID != tt.userID {
				t.Errorf("Expected CreatorID %s, got %s", tt.userID, settlement.CreatorID)
			}

			if len(settlement.Items) != 1 {
				t.Errorf("Expected 1 settlement item, got %d", len(settlement.Items))
			}

			if settlement.TotalAmount != 100.50 {
				t.Errorf("Expected total amount 100.50, got %f", settlement.TotalAmount)
			}
		})
	}
}

func TestSettlementService_ApplyTransactionToSettlement(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()
	service := NewSettlementService(db)

	// Create test users
	testUserID1 := "test-user-1"
	testUserID2 := "test-user-2"

	_, err := db.Exec(`
		INSERT INTO users (id, username, name, role, status, is_admin) 
		VALUES ($1, 'testuser1', 'Test User 1', 'user', 'active', false),
			   ($2, 'testuser2', 'Test User 2', 'user', 'active', false)
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

	// Create initial settlement with first transaction
	settlement, err := service.CreateSettlement(testUserID1, tx1ID, "Initial settlement")
	if err != nil {
		t.Fatalf("Failed to create initial settlement: %v", err)
	}

	tests := []struct {
		name          string
		settlementID  string
		transactionID string
		amount        float64
		userID        string
		expectError   bool
		expectedTotal float64
	}{
		{
			name:          "Apply second transaction to settlement",
			settlementID:  settlement.ID,
			transactionID: tx2ID,
			amount:        50.0,
			userID:        testUserID1,
			expectError:   false,
			expectedTotal: 150.0,
		},
		{
			name:          "Apply to non-existent settlement",
			settlementID:  "non-existent",
			transactionID: tx2ID,
			amount:        50.0,
			userID:        testUserID1,
			expectError:   true,
		},
		{
			name:          "Apply non-existent transaction",
			settlementID:  settlement.ID,
			transactionID: "non-existent",
			amount:        50.0,
			userID:        testUserID1,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ApplyTransactionToSettlement(tt.settlementID, tt.transactionID, tt.amount, tt.userID)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verify the settlement was updated correctly
			var totalAmount float64
			err = db.QueryRow(`
				SELECT total_amount 
				FROM settlements 
				WHERE id = $1
			`, tt.settlementID).Scan(&totalAmount)

			if err != nil {
				t.Errorf("Failed to get updated settlement: %v", err)
				return
			}

			if totalAmount != tt.expectedTotal {
				t.Errorf("Expected total amount %f, got %f", tt.expectedTotal, totalAmount)
			}
		})
	}
}

func TestSettlementService_UpdateSettlementStatus(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()
	service := NewSettlementService(db)

	// Create test users
	testUserID1 := "test-user-1"
	testUserID2 := "test-user-2"

	_, err := db.Exec(`
		INSERT INTO users (id, username, name, role, status, is_admin) 
		VALUES ($1, 'testuser1', 'Test User 1', 'user', 'active', false),
			   ($2, 'testuser2', 'Test User 2', 'user', 'active', false)
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

	// Create a settlement
	settlement, err := service.CreateSettlement(testUserID1, txID, "Test settlement")
	if err != nil {
		t.Fatalf("Failed to create test settlement: %v", err)
	}

	tests := []struct {
		name          string
		settlementID  string
		status        string
		userID        string
		notes         string
		expectError   bool
		expectedTotal float64
	}{
		{
			name:          "Complete settlement",
			settlementID:  settlement.ID,
			status:        "completed",
			userID:        testUserID1,
			notes:         "Settlement completed",
			expectError:   false,
			expectedTotal: 100.50,
		},
		{
			name:         "Update non-existent settlement",
			settlementID: "non-existent",
			status:       "completed",
			userID:       testUserID1,
			notes:        "This should fail",
			expectError:  true,
		},
		{
			name:         "Invalid status",
			settlementID: settlement.ID,
			status:       "invalid-status",
			userID:       testUserID1,
			notes:        "This should fail",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updatedSettlement, err := service.UpdateSettlementStatus(tt.settlementID, tt.status, tt.userID, tt.notes)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if updatedSettlement == nil {
				t.Errorf("Expected updated settlement but got nil")
				return
			}

			if updatedSettlement.Status != tt.status {
				t.Errorf("Expected status %s, got %s", tt.status, updatedSettlement.Status)
			}

			if updatedSettlement.TotalAmount != tt.expectedTotal {
				t.Errorf("Expected total amount %f, got %f", tt.expectedTotal, updatedSettlement.TotalAmount)
			}
		})
	}
}

func TestSettlementService_GetUserSettlements(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()
	service := NewSettlementService(db)

	// Create test users
	testUserID1 := "test-user-1"
	testUserID2 := "test-user-2"

	_, err := db.Exec(`
		INSERT INTO users (id, username, name, role, status, is_admin) 
		VALUES ($1, 'testuser1', 'Test User 1', 'user', 'active', false),
			   ($2, 'testuser2', 'Test User 2', 'user', 'active', false)
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

	// Create settlements
	_, err = service.CreateSettlement(testUserID1, tx1ID, "First settlement")
	if err != nil {
		t.Fatalf("Failed to create first settlement: %v", err)
	}

	_, err = service.CreateSettlement(testUserID1, tx2ID, "Second settlement")
	if err != nil {
		t.Fatalf("Failed to create second settlement: %v", err)
	}

	tests := []struct {
		name        string
		userID      string
		status      string
		expectError bool
		expected    int
	}{
		{
			name:        "Get all settlements for user",
			userID:      testUserID1,
			status:      "",
			expectError: false,
			expected:    2,
		},
		{
			name:        "Get active settlements for user",
			userID:      testUserID1,
			status:      "active",
			expectError: false,
			expected:    2,
		},
		{
			name:        "Get completed settlements for user",
			userID:      testUserID1,
			status:      "completed",
			expectError: false,
			expected:    0,
		},
		{
			name:        "Get settlements for non-existent user",
			userID:      "non-existent",
			status:      "",
			expectError: false,
			expected:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settlements, err := service.GetUserSettlements(tt.userID, tt.status, 0, 0)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(settlements) != tt.expected {
				t.Errorf("Expected %d settlements, got %d", tt.expected, len(settlements))
			}
		})
	}
}

func TestSettlementService_RemoveTransactionFromSettlement(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()
	service := NewSettlementService(db)

	// Create test users
	testUserID1 := "test-user-1"
	testUserID2 := "test-user-2"

	_, err := db.Exec(`
		INSERT INTO users (id, username, name, role, status, is_admin) 
		VALUES ($1, 'testuser1', 'Test User 1', 'user', 'active', false),
			   ($2, 'testuser2', 'Test User 2', 'user', 'active', false)
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

	// Create settlement with both transactions
	settlement, err := service.CreateSettlement(testUserID1, tx1ID, "Test settlement")
	if err != nil {
		t.Fatalf("Failed to create settlement: %v", err)
	}

	err = service.ApplyTransactionToSettlement(settlement.ID, tx2ID, 50.0, testUserID1)
	if err != nil {
		t.Fatalf("Failed to apply second transaction: %v", err)
	}

	tests := []struct {
		name          string
		settlementID  string
		transactionID string
		userID        string
		expectError   bool
		expectedTotal float64
	}{
		{
			name:          "Remove second transaction",
			settlementID:  settlement.ID,
			transactionID: tx2ID,
			userID:        testUserID1,
			expectError:   false,
			expectedTotal: 100.0,
		},
		{
			name:          "Remove non-existent transaction",
			settlementID:  settlement.ID,
			transactionID: "non-existent",
			userID:        testUserID1,
			expectError:   true,
		},
		{
			name:          "Remove from non-existent settlement",
			settlementID:  "non-existent",
			transactionID: tx2ID,
			userID:        testUserID1,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.RemoveTransactionFromSettlement(tt.settlementID, tt.transactionID, tt.userID)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verify the settlement was updated correctly
			var totalAmount float64
			err = db.QueryRow(`
				SELECT total_amount 
				FROM settlements 
				WHERE id = $1
			`, tt.settlementID).Scan(&totalAmount)

			if err != nil {
				t.Errorf("Failed to get updated settlement: %v", err)
				return
			}

			if totalAmount != tt.expectedTotal {
				t.Errorf("Expected total amount %f, got %f", tt.expectedTotal, totalAmount)
			}
		})
	}
}
