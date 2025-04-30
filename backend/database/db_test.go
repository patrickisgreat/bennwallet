package database

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestMain(m *testing.M) {
	// Use in-memory database for tests
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		panic(err)
	}
	DB = db

	// Initialize database with tables
	err = InitDB()
	if err != nil {
		panic(err)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	DB.Close()

	os.Exit(code)
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
