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

// CreatePostgresSchema creates all tables needed for the application
// This is the complete schema definition in one place
func CreatePostgresSchema(db *sql.DB) error {
	log.Println("Creating complete PostgreSQL schema...")

	// Create schema_versions table to track schema updates
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_versions (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			version TEXT NOT NULL,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create schema_versions table: %w", err)
	}

	// Create users table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'user',
			status TEXT DEFAULT 'approved',
			is_admin BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	// Create transactions table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS transactions (
			id TEXT PRIMARY KEY,
			amount NUMERIC(15,2) NOT NULL,
			description TEXT NOT NULL,
			date TEXT NOT NULL,
			transaction_date TIMESTAMP WITH TIME ZONE,
			type TEXT NOT NULL,
			pay_to TEXT,
			paid BOOLEAN NOT NULL DEFAULT FALSE,
			paid_date TEXT,
			optional BOOLEAN NOT NULL DEFAULT FALSE,
			entered_by TEXT NOT NULL,
			user_id TEXT NOT NULL REFERENCES users(id),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create transactions table: %w", err)
	}

	// Create categories table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS categories (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			user_id TEXT NOT NULL REFERENCES users(id),
			color TEXT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(name, user_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create categories table: %w", err)
	}

	// Create transaction_categories join table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS transaction_categories (
			id SERIAL PRIMARY KEY,
			transaction_id TEXT NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
			category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
			amount NUMERIC(15,2) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(transaction_id, category_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create transaction_categories table: %w", err)
	}

	// Create permissions table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS permissions (
			id SERIAL PRIMARY KEY,
			granted_user_id TEXT NOT NULL REFERENCES users(id),
			owner_user_id TEXT NOT NULL REFERENCES users(id),
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

	// Create YNAB config table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ynab_config (
			id SERIAL PRIMARY KEY,
			user_id TEXT NOT NULL UNIQUE REFERENCES users(id),
			api_token TEXT NOT NULL,
			budget_id TEXT NOT NULL,
			account_id TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create ynab_config table: %w", err)
	}

	// Create YNAB settings table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_ynab_settings (
			id SERIAL PRIMARY KEY,
			user_id TEXT NOT NULL UNIQUE REFERENCES users(id),
			last_sync TIMESTAMP WITH TIME ZONE,
			sync_enabled BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create user_ynab_settings table: %w", err)
	}

	// Create YNAB category groups table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ynab_category_groups (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			category_group_id TEXT NOT NULL,
			user_id TEXT NOT NULL REFERENCES users(id),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create ynab_category_groups table: %w", err)
	}

	// Create YNAB categories table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ynab_categories (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			category_group_id TEXT NOT NULL REFERENCES ynab_category_groups(id),
			user_id TEXT NOT NULL REFERENCES users(id),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create ynab_categories table: %w", err)
	}

	// Create saved filters table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS saved_filters (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			resource_type TEXT NOT NULL,
			filter_config TEXT NOT NULL,
			is_default BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create saved_filters table: %w", err)
	}

	// Create custom reports table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS custom_reports (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			description TEXT,
			report_config TEXT NOT NULL,
			is_public BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create custom_reports table: %w", err)
	}

	log.Println("Successfully created complete PostgreSQL schema")
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
