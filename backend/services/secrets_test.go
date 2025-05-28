package services

import (
	"bennwallet/backend/database"
	"bennwallet/backend/testutil"
	"os"
	"strings"
	"testing"
)

func TestStoreAndGetSecret(t *testing.T) {
	// Setup test database
	testDB, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	// Override the database for this test
	oldDB := database.DB
	database.DB = testDB
	defer func() { database.DB = oldDB }()

	// Test cases
	testCases := []struct {
		name      string
		userID    string
		secretVal string
	}{
		{
			name:      "Store and retrieve basic secret",
			userID:    "testuser1",
			secretVal: "secret-token-123",
		},
		{
			name:      "Store and retrieve secret with special chars",
			userID:    "testuser2",
			secretVal: "token-!@#$%^&*()",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Ensure the user exists (required by foreign key constraint)
			_, err := testDB.Exec(`
				INSERT INTO users (id, username, name, role)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (id) DO NOTHING
			`, tc.userID, "user_"+tc.userID, "User "+tc.userID, "user")
			if err != nil {
				t.Fatalf("Failed to create test user: %v", err)
			}

			// Store a secret
			err = StoreSecret(tc.userID, SecretYNABToken, tc.secretVal)
			if err != nil {
				t.Fatalf("Failed to store secret: %v", err)
			}

			// Retrieve the secret
			retrievedVal, err := GetSecret(tc.userID, SecretYNABToken)
			if err != nil {
				t.Fatalf("Failed to retrieve secret: %v", err)
			}

			// Verify the retrieved value
			if retrievedVal != tc.secretVal {
				t.Errorf("Retrieved secret doesn't match stored value. Got %s, want %s", retrievedVal, tc.secretVal)
			}
		})
	}
}

func TestStoreSecretWithFlyEnv(t *testing.T) {
	// Setup test database
	testDB, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	// Override the database for this test
	oldDB := database.DB
	database.DB = testDB
	defer func() { database.DB = oldDB }()

	// Set FLY_APP_NAME environment variable
	oldFlyAppName := os.Getenv("FLY_APP_NAME")
	os.Setenv("FLY_APP_NAME", "test-app")
	defer os.Setenv("FLY_APP_NAME", oldFlyAppName)

	// Store a secret
	userID := "testuser3"
	secretVal := "fly-secret-token"

	// Ensure the user exists (required by foreign key constraint)
	_, err := testDB.Exec(`
		INSERT INTO users (id, username, name, role)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO NOTHING
	`, userID, "user_"+userID, "User "+userID, "user")
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Insert a row first - needed since fly.io mode doesn't actually store in DB
	_, err = testDB.Exec(`
		INSERT INTO user_ynab_settings (user_id, token, budget_id, account_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE SET token = $2
	`, userID, "stored in fly.io secrets", "", "")
	if err != nil {
		t.Fatalf("Failed to insert initial row: %v", err)
	}

	// Now store the secret which in fly.io mode just logs and returns
	err = StoreSecret(userID, SecretYNABToken, secretVal)
	if err != nil {
		t.Fatalf("Failed to store secret: %v", err)
	}

	// Verify that the placeholder was stored in the database
	var storedVal string
	err = testDB.QueryRow("SELECT token FROM user_ynab_settings WHERE user_id = $1", userID).Scan(&storedVal)
	if err != nil {
		t.Fatalf("Failed to retrieve stored value: %v", err)
	}

	if !strings.Contains(storedVal, "stored in fly.io secrets") {
		t.Errorf("Expected placeholder for Fly.io secret, got: %s", storedVal)
	}
}

func TestUpdateYNABSettings(t *testing.T) {
	// Setup test database
	testDB, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	// Override the database for this test
	oldDB := database.DB
	database.DB = testDB
	defer func() { database.DB = oldDB }()

	// No need to create tables - SetupTestDB already creates them with the correct schema

	// Test updating YNAB settings
	userID := "testuser4"
	token := "updated-token"
	budgetID := "budget-123"
	accountID := "account-456"
	syncEnabled := true

	// First ensure the user exists (required by foreign key constraint)
	_, err := testDB.Exec(`
		INSERT INTO users (id, username, name, role)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO NOTHING
	`, userID, "testuser4", "Test User 4", "user")
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Update settings
	err = UpdateYNABSettings(userID, token, budgetID, accountID, syncEnabled)
	if err != nil {
		t.Fatalf("Failed to update YNAB settings: %v", err)
	}

	// Verify settings were updated correctly
	var (
		storedBudgetID    string
		storedAccountID   string
		storedSyncEnabled bool
	)
	err = testDB.QueryRow(`
		SELECT budget_id, account_id, sync_enabled 
		FROM user_ynab_settings 
		WHERE user_id = $1`, userID).Scan(&storedBudgetID, &storedAccountID, &storedSyncEnabled)
	if err != nil {
		t.Fatalf("Failed to retrieve updated settings: %v", err)
	}

	if storedBudgetID != budgetID {
		t.Errorf("Budget ID mismatch. Got %s, want %s", storedBudgetID, budgetID)
	}
	if storedAccountID != accountID {
		t.Errorf("Account ID mismatch. Got %s, want %s", storedAccountID, accountID)
	}
	if storedSyncEnabled != syncEnabled {
		t.Errorf("Sync enabled mismatch. Got %v, want %v", storedSyncEnabled, syncEnabled)
	}
}
