package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"testing"

	// Import the PostgreSQL driver
	_ "github.com/lib/pq"
)

// DB is the global database connection
var DB *sql.DB

// InitDB initializes the database connection
func InitDB() error {
	var err error

	// Connect to PostgreSQL
	log.Println("Connecting to PostgreSQL...")
	DB, err = CreatePostgresDB()
	if err != nil {
		return err
	}

	// Configure connection pooling
	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(5)
	DB.SetConnMaxLifetime(5 * time.Minute)

	// Create schema if needed
	if err := CreatePostgresSchema(DB); err != nil {
		return err
	}

	// If we need to reset the database, seed default data
	if os.Getenv("RESET_DB") == "true" {
		if err := SeedDefaultData(DB); err != nil {
			return err
		}
	}

	return nil
}

// SeedDefaultUsers seeds default users for testing
func SeedDefaultUsers() error {
	// Insert default users for testing purposes
	_, err := DB.Exec(`
		INSERT INTO users (id, username, name, status, is_admin, role)
		VALUES 
		('1', 'sarah', 'Sarah', 'approved', true, 'admin'),
		('2', 'patrick', 'Patrick', 'approved', true, 'admin')
		ON CONFLICT (id) DO UPDATE SET
		   username = EXCLUDED.username,
		   name = EXCLUDED.name,
		   status = EXCLUDED.status,
		   is_admin = EXCLUDED.is_admin,
		   role = EXCLUDED.role
	`)

	return err
}

// SetupTestDB creates a new test database for PostgreSQL testing
func SetupTestDB(t testing.TB) (*sql.DB, func()) {
	// Create a test PostgreSQL database
	testConfig := PostgresConfig{
		Host:     getEnvOrDefault("TEST_DB_HOST", "localhost"),
		Port:     getEnvOrDefault("TEST_DB_PORT", "5432"),
		User:     getEnvOrDefault("TEST_DB_USER", "postgres"),
		Password: getEnvOrDefault("TEST_DB_PASSWORD", "postgres"),
		DBName:   getEnvOrDefault("TEST_DB_NAME", "bennwallet_test"),
		SSLMode:  "disable",
	}

	connectionString := testConfig.ConnectionString()
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		t.Fatalf("Failed to create test database connection: %v", err)
	}

	// First drop all existing tables to avoid conflicts
	_, err = db.Exec(`
		DO $$ 
		DECLARE
			r RECORD;
		BEGIN
			-- Disable foreign key checks during table deletion
			EXECUTE 'SET CONSTRAINTS ALL DEFERRED';
			
			-- Drop all tables in the public schema
			FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = 'public') LOOP
				EXECUTE 'DROP TABLE IF EXISTS ' || quote_ident(r.tablename) || ' CASCADE';
			END LOOP;
			
			-- Re-enable foreign key checks
			EXECUTE 'SET CONSTRAINTS ALL IMMEDIATE';
		END $$;
	`)
	if err != nil {
		t.Logf("Warning: Failed to drop existing tables: %v", err)
	}

	// Create schema
	if err := CreatePostgresSchema(db); err != nil {
		t.Fatalf("Failed to create test schema: %v", err)
	}

	// Insert test user for foreign key constraints
	_, err = db.Exec(`
		INSERT INTO users (id, username, name, role, status, is_admin) 
		VALUES 
		('test-user-id', 'testuser', 'Test User', 'admin', 'approved', true),
		('1', 'sarah', 'Sarah', 'admin', 'approved', true),
		('2', 'patrick', 'Patrick', 'admin', 'approved', true),
		('admin1', 'admin1', 'Admin One', 'admin', 'approved', true)
		ON CONFLICT (id) DO NOTHING
	`)
	if err != nil {
		t.Logf("Warning: Failed to insert test users: %v", err)
	}

	// Return the db and a cleanup function
	return db, func() {
		// Drop all tables on cleanup
		db.Exec(`
			DO $$ 
			DECLARE
				r RECORD;
			BEGIN
				-- Disable foreign key checks during table deletion
				EXECUTE 'SET CONSTRAINTS ALL DEFERRED';
				
				-- Drop all tables in the public schema
				FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = 'public') LOOP
					EXECUTE 'DROP TABLE IF EXISTS ' || quote_ident(r.tablename) || ' CASCADE';
				END LOOP;
				
				-- Re-enable foreign key checks
				EXECUTE 'SET CONSTRAINTS ALL IMMEDIATE';
			END $$;
		`)
		db.Close()
	}
}

// GetDBType returns the type of database being used
func GetDBType() string {
	return "postgres"
}

// CreatePostgresSchema creates the database schema
func CreatePostgresSchema(db *sql.DB) error {
	// Create users table first for foreign key support
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT,
			name TEXT,
			status TEXT DEFAULT 'approved',
			is_admin BOOLEAN DEFAULT FALSE,
			role TEXT DEFAULT 'user'
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	// Create permissions table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS permissions (
			id SERIAL PRIMARY KEY,
			granted_user_id TEXT NOT NULL,
			owner_user_id TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			permission_type TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP WITH TIME ZONE,
			UNIQUE(granted_user_id, owner_user_id, resource_type, permission_type)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create permissions table: %w", err)
	}

	// Create transactions table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS transactions (
			id TEXT PRIMARY KEY,
			amount NUMERIC(15,2) NOT NULL,
			description TEXT NOT NULL,
			date TIMESTAMP NOT NULL,
			transaction_date TIMESTAMP,
			type TEXT NOT NULL,
			pay_to TEXT,
			paid BOOLEAN NOT NULL DEFAULT false,
			paid_date TEXT,
			entered_by TEXT NOT NULL,
			optional BOOLEAN NOT NULL DEFAULT false,
			user_id TEXT,
			note TEXT
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create transactions table: %w", err)
	}

	// Create transaction_categories table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS transaction_categories (
			id SERIAL PRIMARY KEY,
			transaction_id TEXT NOT NULL,
			category_id TEXT NOT NULL,
			amount NUMERIC(15,2) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(transaction_id, category_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create transaction_categories table: %w", err)
	}

	// Create ynab_category_groups table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ynab_category_groups (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			category_group_id TEXT,
			user_id TEXT NOT NULL,
			hidden BOOLEAN DEFAULT false,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create ynab_category_groups table: %w", err)
	}

	// Create ynab_categories table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ynab_categories (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			group_id TEXT,
			category_group_id TEXT,
			user_id TEXT NOT NULL,
			hidden BOOLEAN DEFAULT false,
			budget_amount DECIMAL(15,2),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create ynab_categories table: %w", err)
	}

	// Create user_ynab_settings table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_ynab_settings (
			user_id TEXT PRIMARY KEY,
			token TEXT,
			budget_id TEXT,
			account_id TEXT,
			sync_enabled BOOLEAN DEFAULT true,
			last_synced TIMESTAMP WITH TIME ZONE
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create user_ynab_settings table: %w", err)
	}

	return nil
}
