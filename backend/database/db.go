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

// PostgresConfig holds database connection parameters
type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// GetPostgresConfigFromEnv reads PostgreSQL configuration from environment variables
func GetPostgresConfigFromEnv() PostgresConfig {
	// Check if we're in a test environment
	if os.Getenv("GO_ENV") == "test" {
		return PostgresConfig{
			Host:     "localhost",
			Port:     "5432",
			User:     "postgres",
			Password: "postgres",
			DBName:   "bennwallet_test",
			SSLMode:  "disable",
		}
	}

	// Otherwise use environment variables with defaults
	return PostgresConfig{
		Host:     getEnvOrDefault("DB_HOST", "localhost"),
		Port:     getEnvOrDefault("DB_PORT", "5432"),
		User:     getEnvOrDefault("DB_USER", "postgres"),
		Password: getEnvOrDefault("DB_PASSWORD", "postgres"),
		DBName:   getEnvOrDefault("DB_NAME", "bennwallet"),
		SSLMode:  getEnvOrDefault("DB_SSL_MODE", "disable"),
	}
}

// ConnectionString builds a PostgreSQL connection string
func (cfg PostgresConfig) ConnectionString() string {
	// If DATABASE_URL is set (Fly.io or other cloud provider), use it directly
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		return dbURL
	}

	// Otherwise build from components
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName, cfg.SSLMode,
	)
}

// CreatePostgresDB creates a new PostgreSQL database connection
func CreatePostgresDB() (*sql.DB, error) {
	config := GetPostgresConfigFromEnv()
	connectionString := config.ConnectionString()

	log.Printf("Connecting to PostgreSQL: %s", MaskPassword(connectionString))

	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open PostgreSQL connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	log.Println("Successfully connected to PostgreSQL")
	return db, nil
}

// MaskPassword masks the password in a connection string for logging
func MaskPassword(connStr string) string {
	// Simple regex-free approach to mask password
	result := ""
	inPassword := false

	for i := 0; i < len(connStr); i++ {
		if inPassword {
			if connStr[i] == '@' {
				inPassword = false
				result += "@"
			} else {
				result += "*"
			}
		} else if i+1 < len(connStr) && connStr[i:i+2] == ":" && connStr[i-1] != '/' {
			result += ":"
			inPassword = true
		} else {
			result += string(connStr[i])
		}
	}

	return result
}

// Helper function to get environment variable with default
func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

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

// CreatePostgresSchema creates all the tables needed for PostgreSQL
func CreatePostgresSchema(db *sql.DB) error {
	// Create migrations table first to track schema versions
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS migrations (
			id SERIAL PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Create base tables in correct order to handle dependencies
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			role TEXT NOT NULL,
			status TEXT DEFAULT 'approved',
			is_admin BOOLEAN DEFAULT FALSE
		);

		CREATE TABLE IF NOT EXISTS permissions (
			id SERIAL PRIMARY KEY,
			granted_user_id TEXT NOT NULL REFERENCES users(id),
			owner_user_id TEXT NOT NULL REFERENCES users(id),
			resource_type TEXT NOT NULL,
			permission_type TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP WITH TIME ZONE,
			UNIQUE(granted_user_id, owner_user_id, resource_type, permission_type)
		);

		CREATE TABLE IF NOT EXISTS ynab_category_groups (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			category_group_id TEXT NOT NULL,
			hidden BOOLEAN DEFAULT false,
			user_id TEXT NOT NULL REFERENCES users(id),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			last_updated TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS ynab_categories (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			category_group_id TEXT NOT NULL,
			hidden BOOLEAN DEFAULT false,
			budget_amount DECIMAL(15,2),
			user_id TEXT NOT NULL REFERENCES users(id),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			last_updated TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (category_group_id) REFERENCES ynab_category_groups(id)
		);

		CREATE TABLE IF NOT EXISTS transactions (
			id TEXT PRIMARY KEY,
			amount NUMERIC(15,2) NOT NULL,
			description TEXT NOT NULL,
			date TIMESTAMP NOT NULL,
			transaction_date TIMESTAMP,
			type TEXT NOT NULL,
			pay_to TEXT,
			paid BOOLEAN NOT NULL DEFAULT FALSE,
			paid_date TEXT,
			optional BOOLEAN NOT NULL DEFAULT FALSE,
			entered_by TEXT NOT NULL,
			user_id TEXT NOT NULL REFERENCES users(id),
			note TEXT
		);

		CREATE TABLE IF NOT EXISTS transaction_categories (
			id SERIAL PRIMARY KEY,
			transaction_id TEXT NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
			category_id TEXT NOT NULL REFERENCES ynab_categories(id) ON DELETE CASCADE,
			amount NUMERIC(15,2) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(transaction_id, category_id)
		);

		CREATE TABLE IF NOT EXISTS ynab_config (
			id SERIAL PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id),
			encrypted_api_token TEXT,
			encrypted_budget_id TEXT,
			encrypted_account_id TEXT,
			api_token TEXT,
			budget_id TEXT,
			account_id TEXT,
			last_sync_time TIMESTAMP,
			sync_frequency INTEGER DEFAULT 60,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			has_credentials BOOLEAN DEFAULT FALSE,
			UNIQUE(user_id)
		);

		CREATE TABLE IF NOT EXISTS user_ynab_settings (
			user_id TEXT PRIMARY KEY REFERENCES users(id),
			token TEXT,
			budget_id TEXT,
			account_id TEXT,
			auto_import BOOLEAN DEFAULT false,
			sync_enabled BOOLEAN DEFAULT false,
			last_synced TIMESTAMP WITH TIME ZONE
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create base tables: %w", err)
	}

	return nil
}

// SeedDefaultData inserts default data into the database
func SeedDefaultData(db *sql.DB) error {
	log.Println("Database tables created. No default data will be seeded.")

	// Create migrations table if it doesn't exist yet
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS migrations (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Record that we've run the initial setup
	_, err = db.Exec(`
		INSERT INTO migrations (name) 
		VALUES ('initial_setup')
		ON CONFLICT (name) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("failed to record initial setup migration: %w", err)
	}

	log.Println("Database setup completed. No data seeded as per application requirements.")
	return nil
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
		t.Fatalf("Failed to drop existing tables: %v", err)
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
		ON CONFLICT (id) DO UPDATE SET 
		username = EXCLUDED.username,
		name = EXCLUDED.name,
		role = EXCLUDED.role,
		status = EXCLUDED.status,
		is_admin = EXCLUDED.is_admin
	`)
	if err != nil {
		t.Fatalf("Failed to insert test users: %v", err)
	}

	// Insert test category groups
	_, err = db.Exec(`
		INSERT INTO ynab_category_groups (id, name, category_group_id, user_id, hidden)
		VALUES 
		('group-1', 'Food', 'group-1', 'test-user-id', false),
		('group-2', 'Housing', 'group-2', 'test-user-id', false),
		('group-3', 'Fun', 'group-3', 'test-user-id', false)
		ON CONFLICT (id) DO UPDATE SET
		name = EXCLUDED.name,
		category_group_id = EXCLUDED.category_group_id,
		user_id = EXCLUDED.user_id,
		hidden = EXCLUDED.hidden
	`)
	if err != nil {
		t.Fatalf("Failed to insert test category groups: %v", err)
	}

	// Insert test categories
	_, err = db.Exec(`
		INSERT INTO ynab_categories (id, name, user_id, category_group_id, hidden)
		VALUES 
		('cat-test-user-id-Food', 'Food', 'test-user-id', 'group-1', false),
		('cat-test-user-id-Housing', 'Housing', 'test-user-id', 'group-2', false),
		('cat-test-user-id-Fun', 'Fun', 'test-user-id', 'group-3', false)
		ON CONFLICT (id) DO UPDATE SET
		name = EXCLUDED.name,
		user_id = EXCLUDED.user_id,
		category_group_id = EXCLUDED.category_group_id,
		hidden = EXCLUDED.hidden
	`)
	if err != nil {
		t.Fatalf("Failed to insert test categories: %v", err)
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
