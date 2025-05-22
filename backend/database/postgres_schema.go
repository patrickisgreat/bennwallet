package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq" // PostgreSQL driver
)

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

// CreatePostgresSchema creates all the tables needed for PostgreSQL
// This is a clean schema definition for first-time database creation
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

	// Create base tables
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			role TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS transactions (
			id TEXT PRIMARY KEY,
			amount NUMERIC(15,2) NOT NULL,
			description TEXT NOT NULL,
			date TEXT NOT NULL,
			transaction_date TEXT,
			type TEXT NOT NULL,
			pay_to TEXT,
			paid BOOLEAN NOT NULL DEFAULT FALSE,
			paid_date TEXT,
			optional BOOLEAN NOT NULL DEFAULT FALSE,
			entered_by TEXT NOT NULL,
			user_id TEXT NOT NULL REFERENCES users(id),
			note TEXT
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
			last_synced TIMESTAMP
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
			category_group_id TEXT REFERENCES ynab_category_groups(id),
			hidden BOOLEAN DEFAULT false,
			budget_amount DECIMAL(15,2),
			user_id TEXT NOT NULL REFERENCES users(id),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			last_updated TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
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

// All other seeding functions are removed as we don't want to create any data automatically.
// Users, categories, and other data will be created by the users through the application
// or synced from external services like YNAB.
