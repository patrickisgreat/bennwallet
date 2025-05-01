package database

import (
	"bennwallet/backend/migrations"
	"database/sql"
	"os"
	"path/filepath"
	"time"

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
	if err := migrations.RunMigrations(DB); err != nil {
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
