package testutil

import (
	"bennwallet/backend/migrations"
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// Define the same context keys your auth middleware uses
type contextKey string

const UserIDKey contextKey = "user_id"
const UserRoleKey contextKey = "user_role"
const TestUserID string = "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2"

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

// ForceResetTestDatabase completely drops and recreates the test database
func ForceResetTestDatabase() error {
	config := GetTestDBConfig()
	testDBName := config.DBName // Use the unique database name

	// Connect to postgres database to drop/create the test database
	config.DBName = "postgres"
	db, err := sql.Open("postgres", config.ConnectionString())
	if err != nil {
		return fmt.Errorf("failed to connect to postgres: %w", err)
	}
	defer db.Close()

	// Terminate all connections to the test database
	_, err = db.Exec(fmt.Sprintf(`
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
	_, err = db.Exec(fmt.Sprintf(`
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

func MockAuthContext(req *http.Request, userID string) *http.Request {
	// Set the exact same context keys your middleware uses
	ctx := context.WithValue(req.Context(), UserIDKey, userID)
	ctx = context.WithValue(ctx, UserRoleKey, "admin")

	// Also set as string keys in case your handlers expect that
	ctx = context.WithValue(ctx, "user_id", userID)
	ctx = context.WithValue(ctx, "userID", userID)
	ctx = context.WithValue(ctx, "userId", userID)

	return req.WithContext(ctx)
}

// SeedYNABTestData seeds additional test data for YNAB categories and groups
// Only use this if you need EXTRA test data beyond what migrations provide
func SeedYNABTestData(db *sql.DB) error {
	// Insert additional test YNAB category groups
	_, err := db.Exec(`
		INSERT INTO ynab_category_groups (id, name, category_group_id, user_id, hidden)
		VALUES 
			('test-group1', 'Test Group 1', 'test-group1', $1, false),
			('test-group2', 'Test Group 2', 'test-group2', $1, false)
		ON CONFLICT (id) DO UPDATE SET 
			name = EXCLUDED.name,
			category_group_id = EXCLUDED.category_group_id,
			user_id = EXCLUDED.user_id,
			hidden = EXCLUDED.hidden;
	`, TestUserID)
	if err != nil {
		return fmt.Errorf("error inserting test category groups: %v", err)
	}

	// Insert additional test YNAB categories
	_, err = db.Exec(`
		INSERT INTO ynab_categories (id, name, group_id, category_group_id, user_id, hidden, budget_amount)
		VALUES 
			('test-cat1', 'Test Category 1', 'test-group1', 'test-group1', $1, false, 100.00),
			('test-cat2', 'Test Category 2', 'test-group1', 'test-group1', $1, false, 200.00),
			('test-cat3', 'Test Category 3', 'test-group2', 'test-group2', $1, false, 300.00)
		ON CONFLICT (id) DO UPDATE SET 
			name = EXCLUDED.name,
			group_id = EXCLUDED.group_id,
			category_group_id = EXCLUDED.category_group_id,
			user_id = EXCLUDED.user_id,
			hidden = EXCLUDED.hidden,
			budget_amount = EXCLUDED.budget_amount;
	`, TestUserID)
	if err != nil {
		return fmt.Errorf("error inserting test categories: %v", err)
	}

	return nil
}

// SeedSpecificYNABTestData seeds specific test data for individual tests
// Use this when a test needs specific category groups/categories
func SeedSpecificYNABTestData(db *sql.DB, userID string, groupCount int) error {
	// Clear existing test data for this user
	_, err := db.Exec(`DELETE FROM ynab_categories WHERE user_id = $1 AND id LIKE 'test-%'`, userID)
	if err != nil {
		return fmt.Errorf("failed to clear existing test categories: %w", err)
	}

	_, err = db.Exec(`DELETE FROM ynab_category_groups WHERE user_id = $1 AND id LIKE 'test-%'`, userID)
	if err != nil {
		return fmt.Errorf("failed to clear existing test category groups: %w", err)
	}

	// Insert the requested number of test groups
	for i := 1; i <= groupCount; i++ {
		groupID := fmt.Sprintf("test-group-%d", i)
		groupName := fmt.Sprintf("Test Group %d", i)

		_, err = db.Exec(`
			INSERT INTO ynab_category_groups (id, name, category_group_id, user_id, hidden)
			VALUES ($1, $2, $1, $3, false)
		`, groupID, groupName, userID)
		if err != nil {
			return fmt.Errorf("failed to insert test group %d: %w", i, err)
		}

		// Insert a test category for each group
		categoryID := fmt.Sprintf("test-cat-%d", i)
		categoryName := fmt.Sprintf("Test Category %d", i)

		_, err = db.Exec(`
			INSERT INTO ynab_categories (id, name, group_id, category_group_id, user_id, hidden, budget_amount)
			VALUES ($1, $2, $3, $3, $4, false, $5)
		`, categoryID, categoryName, groupID, userID, float64(i*100))
		if err != nil {
			return fmt.Errorf("failed to insert test category %d: %w", i, err)
		}
	}

	return nil
}

// CleanupTestDB performs cleanup operations on the test database
func CleanupTestDB(db *sql.DB) error {
	// Clear test data but keep schema
	tables := []string{
		"transaction_categories",
		"transactions",
		"permissions",
		"ynab_categories",
		"ynab_category_groups",
		"user_ynab_settings",
		"ynab_config",
	}

	for _, table := range tables {
		_, err := db.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
		if err != nil {
			return fmt.Errorf("failed to truncate %s: %w", table, err)
		}
	}

	return nil
}

// CreateTestUser creates a test user in the database
func CreateTestUser(db *sql.DB, userID, username, name, role string) error {
	_, err := db.Exec(`
		INSERT INTO users (id, username, name, role)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE SET
			username = EXCLUDED.username,
			name = EXCLUDED.name,
			role = EXCLUDED.role
	`, userID, username, name, role)

	if err != nil {
		return fmt.Errorf("failed to create test user: %w", err)
	}

	return nil
}

// CreateTestTransaction creates a test transaction in the database
func CreateTestTransaction(db *sql.DB, id, userID, description, txType string, amount float64) error {
	now := time.Now()

	_, err := db.Exec(`
		INSERT INTO transactions (id, amount, description, date, transaction_date, type, pay_to, paid, entered_by, user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO NOTHING
	`, id, amount, description, now, now, txType, "test-payee", true, userID, userID)

	if err != nil {
		return fmt.Errorf("failed to create test transaction: %w", err)
	}

	return nil
}

// WaitForDB waits for the database to be ready
func WaitForDB(db *sql.DB, maxAttempts int) error {
	for i := 0; i < maxAttempts; i++ {
		if err := db.Ping(); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("database not ready after %d attempts", maxAttempts)
}
