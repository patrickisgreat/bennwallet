package testutil

import (
	"bennwallet/backend/migrations"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

const TestUserID = "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2"

// PostgresConfig holds configuration for a PostgreSQL database connection
type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// ConnectionString returns a PostgreSQL connection string
func (c PostgresConfig) ConnectionString() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}

// GetTestDBConfig returns test database configuration with a unique test database name for parallel execution
func GetTestDBConfig() PostgresConfig {
	// Generate unique database name for parallel test execution
	dbName := fmt.Sprintf("bennwallet_test_%d", time.Now().UnixNano())

	return PostgresConfig{
		Host:     getEnvOrDefault("POSTGRES_HOST", "localhost"),
		Port:     getEnvOrDefault("POSTGRES_PORT", "5432"),
		User:     getEnvOrDefault("POSTGRES_USER", "postgres"),
		Password: getEnvOrDefault("POSTGRES_PASSWORD", "postgres"),
		DBName:   dbName,
		SSLMode:  "disable",
	}
}

// getEnvOrDefault gets environment variable or returns default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// ForceResetTestDatabaseWithConfig completely drops and recreates the test database using provided config
func ForceResetTestDatabaseWithConfig(config PostgresConfig) error {
	testDBName := config.DBName // Use the database name from config

	// Connect to postgres database to drop/create the test database
	configCopy := config
	configCopy.DBName = "postgres"
	db, err := sql.Open("postgres", configCopy.ConnectionString())
	if err != nil {
		return fmt.Errorf("failed to connect to postgres: %w", err)
	}
	defer db.Close()

	// Terminate all connections to the test database
	_, _ = db.Exec(fmt.Sprintf(`
		SELECT pg_terminate_backend(pg_stat_activity.pid)
		FROM pg_stat_activity
		WHERE pg_stat_activity.datname = '%s'
		AND pid <> pg_backend_pid()
	`, testDBName))
	// Ignore error as database might not exist

	// Drop the test database if it exists
	_, err = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", testDBName))
	if err != nil && !strings.Contains(err.Error(), "does not exist") {
		return fmt.Errorf("failed to drop test database: %w", err)
	}

	// Create a fresh test database
	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", testDBName))
	if err != nil {
		return fmt.Errorf("failed to create test database: %w", err)
	}

	return nil
}

// SetupPostgresTestDB creates a new test database connection and returns it
// This is the ONLY function you need to call to set up a complete test database
func SetupPostgresTestDB() (*sql.DB, error) {
	// Clear Firebase env vars to ensure dev mode auth bypass
	os.Unsetenv("FIREBASE_SERVICE_ACCOUNT_JSON")
	os.Unsetenv("FIREBASE_SERVICE_ACCOUNT_BASE64")
	os.Unsetenv("FIREBASE_SERVICE_ACCOUNT")

	// Get config once and reuse it to ensure same database name
	config := GetTestDBConfig()

	// Force reset the entire database first for clean state
	if err := ForceResetTestDatabaseWithConfig(config); err != nil {
		return nil, fmt.Errorf("failed to force reset database: %w", err)
	}

	db, err := sql.Open("postgres", config.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pooling
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Set environment to ensure test data is seeded
	os.Setenv("APP_ENV", "development")
	os.Setenv("RESET_DB", "true")

	// Use the migration system to create the complete schema
	// This creates ALL tables (users, transactions, categories, permissions, etc.)
	// AND seeds them with test data automatically
	err = migrations.RunMigrations(db, true) // true = reset database
	if err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Ensure test user exists
	_, err = db.Exec(`
		INSERT INTO users (id, username, name, role)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO NOTHING`,
		TestUserID, "Patrick Bennett", "Patrick Bennett", "admin",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert test user: %w", err)
	}

	// Ensure user_ynab_settings row exists for test user with sync_enabled = true
	_, err = db.Exec(`
		INSERT INTO user_ynab_settings (user_id, token, budget_id, account_id, sync_enabled)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE SET sync_enabled = EXCLUDED.sync_enabled`,
		TestUserID, "test-token", "test-budget", "test-account", true,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert user_ynab_settings: %w", err)
	}

	return db, nil
}
