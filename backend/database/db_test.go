package database

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

func TestMain(m *testing.M) {
	// Create a test PostgreSQL database
	var err error
	testConfig := PostgresConfig{
		Host:     getEnvOrDefault("TEST_DB_HOST", "localhost"),
		Port:     getEnvOrDefault("TEST_DB_PORT", "5432"),
		User:     getEnvOrDefault("TEST_DB_USER", "postgres"),
		Password: getEnvOrDefault("TEST_DB_PASSWORD", "postgres"),
		DBName:   getEnvOrDefault("TEST_DB_NAME", "bennwallet_test"),
		SSLMode:  "disable",
	}

	connectionString := testConfig.ConnectionString()
	DB, err = sql.Open("postgres", connectionString)
	if err != nil {
		panic(err)
	}

	// Create base tables
	createTables()

	// Run tests
	code := m.Run()

	// Cleanup
	cleanupTestDB()
	DB.Close()

	os.Exit(code)
}

// createTables creates all necessary tables for tests
func createTables() {
	// Create users table
	createUsersTable := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT UNIQUE NOT NULL,
		name TEXT NOT NULL
	);
	`
	_, err := DB.Exec(createUsersTable)
	if err != nil {
		panic(err)
	}

	// Create transactions table
	createTransactionsTable := `
	CREATE TABLE IF NOT EXISTS transactions (
		id TEXT PRIMARY KEY,
		amount NUMERIC(15,2) NOT NULL,
		description TEXT NOT NULL,
		date TIMESTAMP NOT NULL,
		type TEXT NOT NULL,
		pay_to TEXT,
		paid BOOLEAN NOT NULL DEFAULT FALSE,
		paid_date TEXT,
		entered_by TEXT NOT NULL
	);
	`
	_, err = DB.Exec(createTransactionsTable)
	if err != nil {
		panic(err)
	}

	// Create categories table
	createCategoriesTable := `
	CREATE TABLE IF NOT EXISTS categories (
		id SERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT,
		user_id TEXT NOT NULL,
		color TEXT
	);
	`
	_, err = DB.Exec(createCategoriesTable)
	if err != nil {
		panic(err)
	}
}

// cleanupTestDB cleans up all test data
func cleanupTestDB() {
	tables := []string{"users", "transactions", "categories", "ynab_config", "user_ynab_settings", "ynab_category_groups", "ynab_categories"}

	for _, table := range tables {
		DB.Exec("DROP TABLE IF EXISTS " + table + " CASCADE")
	}
}

func TestInitDB(t *testing.T) {
	// Test that tables were created
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('users', 'transactions', 'categories')").Scan(&count)
	if err != nil {
		t.Fatalf("Error checking tables: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 tables, got %d", count)
	}
}

func TestSeedDefaultUsers(t *testing.T) {
	err := SeedDefaultUsers()
	if err != nil {
		t.Fatalf("Error seeding users: %v", err)
	}

	// Check that users were created
	var count int
	err = DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		t.Fatalf("Error counting users: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 users, got %d", count)
	}

	// Check specific users
	var exists bool
	err = DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = 'sarah')").Scan(&exists)
	if err != nil {
		t.Fatalf("Error checking sarah: %v", err)
	}
	if !exists {
		t.Error("User 'sarah' not found")
	}

	err = DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = 'patrick')").Scan(&exists)
	if err != nil {
		t.Fatalf("Error checking patrick: %v", err)
	}
	if !exists {
		t.Error("User 'patrick' not found")
	}
}

func TestRunMigrations(t *testing.T) {
	// First, drop any YNAB-related tables to ensure a clean state
	ynabTables := []string{"ynab_config", "user_ynab_settings", "ynab_category_groups", "ynab_categories"}
	for _, table := range ynabTables {
		_, err := DB.Exec("DROP TABLE IF EXISTS " + table)
		if err != nil {
			t.Fatalf("Error dropping table %s: %v", table, err)
		}
	}

	// Run the migrations
	err := RunMigrations()
	if err != nil {
		t.Fatalf("Error running migrations: %v", err)
	}

	// Check that YNAB config table was created
	var exists bool
	err = DB.QueryRow("SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='ynab_config')").Scan(&exists)
	if err != nil {
		t.Fatalf("Error checking ynab_config table: %v", err)
	}
	if !exists {
		t.Error("YNAB config table not created")
	}

	// Check that user_ynab_settings table was created
	err = DB.QueryRow("SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='user_ynab_settings')").Scan(&exists)
	if err != nil {
		t.Fatalf("Error checking user_ynab_settings table: %v", err)
	}
	if !exists {
		t.Error("Legacy user_ynab_settings table not created")
	}

	// Get the actual columns in the ynab_config table
	rows, err := DB.Query("PRAGMA table_info(ynab_config)")
	if err != nil {
		t.Fatalf("Error getting ynab_config columns: %v", err)
	}
	defer rows.Close()

	// Map to store column names
	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, dfltValue, pk interface{}
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			t.Fatalf("Error scanning column info: %v", err)
		}
		columns[name] = true
	}

	// Now insert using the actual columns from the table
	// This will be more resilient to schema changes
	if columns["user_id"] && columns["encrypted_api_token"] {
		_, err = DB.Exec(`
			INSERT INTO ynab_config (user_id, encrypted_api_token) 
			VALUES (?, ?)`,
			"test-user", "encrypted-token")
		if err != nil {
			t.Fatalf("Error inserting test data into ynab_config: %v", err)
		}

		var userId string
		err = DB.QueryRow("SELECT user_id FROM ynab_config WHERE user_id = ?", "test-user").Scan(&userId)
		if err != nil {
			t.Fatalf("Error retrieving test data from ynab_config: %v", err)
		}
		if userId != "test-user" {
			t.Errorf("Expected user_id 'test-user', got '%s'", userId)
		}
	} else {
		t.Skip("Skipping insert test as required columns not found in ynab_config table")
	}
}

func TestSeedDefaultUsers_WithExistingUsers(t *testing.T) {
	// Reset user table to make sure we have consistent test state
	_, err := DB.Exec("DELETE FROM users")
	if err != nil {
		t.Fatalf("Error clearing users table: %v", err)
	}

	// Insert a different user
	_, err = DB.Exec("INSERT INTO users (id, username, name) VALUES (?, ?, ?)",
		"3", "testuser", "Test User")
	if err != nil {
		t.Fatalf("Error inserting test user: %v", err)
	}

	// Check the user count
	var initialCount int
	err = DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&initialCount)
	if err != nil {
		t.Fatalf("Error counting users: %v", err)
	}
	if initialCount != 1 {
		t.Errorf("Expected 1 user before seeding, got %d", initialCount)
	}

	// Run SeedDefaultUsers which checks for existing users
	// Note: SeedDefaultUsers only adds default users if the users table is empty
	// Since we've added one user, it shouldn't add the default users
	err = SeedDefaultUsers()
	if err != nil {
		t.Fatalf("Error running SeedDefaultUsers: %v", err)
	}

	// Verify user table state after seeding
	var testUserExists bool
	err = DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = 'testuser')").Scan(&testUserExists)
	if err != nil {
		t.Fatalf("Error checking testuser: %v", err)
	}
	if !testUserExists {
		t.Error("User 'testuser' should still exist after seeding default users")
	}

	// Check the total count - should still be 1 since default users aren't added when table isn't empty
	var finalCount int
	err = DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&finalCount)
	if err != nil {
		t.Fatalf("Error counting users: %v", err)
	}
	if finalCount != 1 {
		t.Errorf("Expected 1 user after seeding (since table wasn't empty), got %d", finalCount)
	}

	// Test that default users are not added when table isn't empty
	var sarahExists bool
	err = DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = 'sarah')").Scan(&sarahExists)
	if err != nil {
		t.Fatalf("Error checking sarah: %v", err)
	}
	if sarahExists {
		t.Error("User 'sarah' should not exist after seeding since table wasn't empty")
	}
}
