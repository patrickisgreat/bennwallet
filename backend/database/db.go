package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	"testing"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDB() error {
	var dbPath string
	if os.Getenv("FLY_APP_NAME") != "" {
		// We're running on Fly.io, use the mounted volume
		dbPath = filepath.Join("/data", "transactions.db")
	} else if os.Getenv("TEST_DB") == "1" {
		// We're running tests, use in-memory database
		dbPath = ":memory:"
	} else {
		// Local development
		dbPath = "./database.db"
	}

	var err error
	// Add connection parameters to better handle concurrency
	dsn := dbPath + "?_journal=WAL&_timeout=10000&_busy_timeout=10000"
	DB, err = sql.Open("sqlite3", dsn)
	if err != nil {
		return err
	}

	// Configure database connection
	DB.SetMaxOpenConns(5) // Increase from 1 to 5
	DB.SetMaxIdleConns(5) // Increase from 1 to 5

	// Add this line
	DB.SetConnMaxLifetime(time.Minute * 5)

	// Execute PRAGMA statements for better concurrency handling
	_, err = DB.Exec("PRAGMA journal_mode=WAL;")
	if err != nil {
		return err
	}

	_, err = DB.Exec("PRAGMA busy_timeout=5000;")
	if err != nil {
		return err
	}

	// Test the connection
	err = DB.Ping()
	if err != nil {
		return err
	}

	// Create users table
	createUsersTable := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT UNIQUE NOT NULL,
		name TEXT NOT NULL
	);
	`
	_, err = DB.Exec(createUsersTable)
	if err != nil {
		return err
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
		return err
	}

	// Create categories table with user_id as TEXT
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
		return err
	}

	// Run migrations
	if err := RunMigrations(); err != nil {
		return err
	}

	return nil
}

func SeedDefaultUsers() error {
	// Check if users exist
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		// Insert default users
		defaultUsers := []struct {
			id       string
			username string
			name     string
		}{
			{id: "1", username: "sarah", name: "Sarah"},
			{id: "2", username: "patrick", name: "Patrick"},
		}

		for _, user := range defaultUsers {
			_, err := DB.Exec("INSERT INTO users (id, username, name) VALUES (?, ?, ?)",
				user.id, user.username, user.name)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// SetupTestDB creates a new test database and returns it along with a cleanup function
func SetupTestDB(t *testing.T) (*sql.DB, func()) {
	// Create a new in-memory database
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Create tables
	createUsersTable := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT UNIQUE NOT NULL,
		name TEXT NOT NULL
	);
	`
	_, err = db.Exec(createUsersTable)
	if err != nil {
		t.Fatalf("Failed to create users table: %v", err)
	}

	// Create transactions table
	createTransactionsTable := `
	CREATE TABLE IF NOT EXISTS transactions (
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
	);
	`
	_, err = db.Exec(createTransactionsTable)
	if err != nil {
		t.Fatalf("Failed to create transactions table: %v", err)
	}

	// Create categories table
	createCategoriesTable := `
	CREATE TABLE IF NOT EXISTS categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		description TEXT,
		user_id TEXT NOT NULL,
		color TEXT,
		UNIQUE(name, user_id)
	);
	`
	_, err = db.Exec(createCategoriesTable)
	if err != nil {
		t.Fatalf("Failed to create categories table: %v", err)
	}

	// Create YNAB config table with all required columns
	createYNABConfigTable := `
	CREATE TABLE IF NOT EXISTS ynab_config (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		encrypted_api_token TEXT,
		encrypted_budget_id TEXT,
		encrypted_account_id TEXT,
		last_sync_time TIMESTAMP,
		sync_frequency INTEGER DEFAULT 60,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
		UNIQUE(user_id)
	);
	`
	_, err = db.Exec(createYNABConfigTable)
	if err != nil {
		t.Fatalf("Failed to create ynab_config table: %v", err)
	}

	// Create user_ynab_settings table
	createUserYNABSettingsTable := `
	CREATE TABLE IF NOT EXISTS user_ynab_settings (
		user_id TEXT PRIMARY KEY,
		token TEXT,
		budget_id TEXT,
		account_id TEXT,
		sync_enabled INTEGER,
		last_synced TIMESTAMP
	);
	`
	_, err = db.Exec(createUserYNABSettingsTable)
	if err != nil {
		t.Fatalf("Failed to create user_ynab_settings table: %v", err)
	}

	// Create YNAB category groups table
	createYNABCategoryGroupsTable := `
	CREATE TABLE IF NOT EXISTS ynab_category_groups (
		id TEXT NOT NULL,
		name TEXT NOT NULL,
		user_id TEXT NOT NULL,
		last_updated DATETIME NOT NULL,
		PRIMARY KEY (id, user_id)
	);
	`
	_, err = db.Exec(createYNABCategoryGroupsTable)
	if err != nil {
		t.Fatalf("Failed to create ynab_category_groups table: %v", err)
	}

	// Create YNAB categories table
	createYNABCategoriesTable := `
	CREATE TABLE IF NOT EXISTS ynab_categories (
		id TEXT NOT NULL,
		group_id TEXT NOT NULL,
		name TEXT NOT NULL,
		user_id TEXT NOT NULL,
		last_updated DATETIME NOT NULL,
		PRIMARY KEY (id, user_id),
		FOREIGN KEY (group_id, user_id) REFERENCES ynab_category_groups(id, user_id)
	);
	`
	_, err = db.Exec(createYNABCategoriesTable)
	if err != nil {
		t.Fatalf("Failed to create ynab_categories table: %v", err)
	}

	// Return the database and a cleanup function
	return db, func() {
		db.Close()
	}
}
