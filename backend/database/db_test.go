package database

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestMain(m *testing.M) {
	// Directly create an in-memory database for tests
	var err error
	DB, err = sql.Open("sqlite3", ":memory:")
	if err != nil {
		panic(err)
	}

	// Create tables manually
	createTables()

	// Run tests
	code := m.Run()

	// Cleanup
	DB.Close()

	os.Exit(code)
}

// createTables creates all necessary tables for tests
func createTables() {
	// Create users table
	createUsersTable := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
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
		amount REAL NOT NULL,
		description TEXT NOT NULL,
		date DATETIME NOT NULL,
		type TEXT NOT NULL,
		payTo TEXT,
		paid BOOLEAN NOT NULL DEFAULT 0,
		paidDate TEXT,
		enteredBy TEXT NOT NULL
	);
	`
	_, err = DB.Exec(createTransactionsTable)
	if err != nil {
		panic(err)
	}

	// Create categories table
	createCategoriesTable := `
	CREATE TABLE IF NOT EXISTS categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
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
