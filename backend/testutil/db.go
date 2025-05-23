package testutil

import (
	"bennwallet/backend/models"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

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

// GetTestDBConfig returns PostgreSQL test database configuration
func GetTestDBConfig() PostgresConfig {
	return PostgresConfig{
		Host:     getEnvOrDefault("TEST_DB_HOST", "localhost"),
		Port:     getEnvOrDefault("TEST_DB_PORT", "5432"),
		User:     getEnvOrDefault("TEST_DB_USER", "postgres"),
		Password: getEnvOrDefault("TEST_DB_PASSWORD", "postgres"),
		DBName:   getEnvOrDefault("TEST_DB_NAME", "bennwallet_test"),
		SSLMode:  "disable",
	}
}

// SetupPostgresTestDB creates a new test database connection and returns it
func SetupPostgresTestDB() (*sql.DB, error) {
	config := GetTestDBConfig()
	db, err := sql.Open("postgres", config.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pooling
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

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
		return nil, fmt.Errorf("failed to drop existing tables: %w", err)
	}

	// Create base tables needed for tests
	err = createTestTables(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create test tables: %w", err)
	}

	// Insert test user
	err = seedTestUsers(db)
	if err != nil {
		return nil, fmt.Errorf("failed to seed test users: %w", err)
	}

	// Insert test data
	_, err = db.Exec(`
		-- Insert test user
		INSERT INTO users (id, email, role) VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET email = EXCLUDED.email, role = EXCLUDED.role;
	`, TestUserID, "test@example.com", models.RoleUser)
	if err != nil {
		return nil, fmt.Errorf("error inserting test user: %v", err)
	}

	// Insert test YNAB category groups
	_, err = db.Exec(`
		INSERT INTO ynab_category_groups (id, name, user_id, hidden)
		VALUES 
			('group1', 'Test Group 1', $1, false),
			('group2', 'Test Group 2', $1, false)
		ON CONFLICT (id) DO UPDATE SET 
			name = EXCLUDED.name,
			user_id = EXCLUDED.user_id,
			hidden = EXCLUDED.hidden;
	`, TestUserID)
	if err != nil {
		return nil, fmt.Errorf("error inserting test category groups: %v", err)
	}

	// Insert test YNAB categories
	_, err = db.Exec(`
		INSERT INTO ynab_categories (id, name, group_id, user_id, hidden, budget_amount, color)
		VALUES 
			('cat1', 'Test Category 1', 'group1', $1, false, 100.00, '#FF0000'),
			('cat2', 'Test Category 2', 'group1', $1, false, 200.00, '#00FF00'),
			('cat3', 'Test Category 3', 'group2', $1, false, 300.00, '#0000FF')
		ON CONFLICT (id) DO UPDATE SET 
			name = EXCLUDED.name,
			group_id = EXCLUDED.group_id,
			user_id = EXCLUDED.user_id,
			hidden = EXCLUDED.hidden,
			budget_amount = EXCLUDED.budget_amount,
			color = EXCLUDED.color;
	`, TestUserID)
	if err != nil {
		return nil, fmt.Errorf("error inserting test categories: %v", err)
	}

	return db, nil
}

// createTestTables creates the necessary tables for testing
func createTestTables(db *sql.DB) error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			name TEXT,
			email TEXT,
			status TEXT,
			is_admin BOOLEAN DEFAULT FALSE,
			role TEXT DEFAULT 'user',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS permissions (
			id SERIAL PRIMARY KEY,
			granted_user_id TEXT NOT NULL,
			owner_user_id TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			permission_type TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP WITH TIME ZONE,
			UNIQUE(granted_user_id, owner_user_id, resource_type, permission_type)
		)`,
		`CREATE TABLE IF NOT EXISTS user_ynab_settings (
			id SERIAL PRIMARY KEY,
			user_id TEXT NOT NULL UNIQUE,
			token TEXT,
			budget_id TEXT,
			account_id TEXT,
			sync_enabled BOOLEAN DEFAULT false,
			last_synced TIMESTAMP WITH TIME ZONE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS ynab_category_groups (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			category_group_id TEXT,
			user_id TEXT NOT NULL,
			hidden BOOLEAN DEFAULT false,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS ynab_categories (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			group_id TEXT,
			category_group_id TEXT,
			user_id TEXT NOT NULL,
			hidden BOOLEAN DEFAULT false,
			budget_amount DECIMAL(15,2),
			color TEXT DEFAULT '#3498DB',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (group_id) REFERENCES ynab_category_groups(id) ON DELETE CASCADE
		)`,
	}

	for _, createSQL := range tables {
		if _, err := db.Exec(createSQL); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	return nil
}

// seedTestUsers inserts default test users
func seedTestUsers(db *sql.DB) error {
	users := []struct {
		id       string
		username string
		name     string
		email    string
		status   string
		isAdmin  bool
		role     string
	}{
		{TestUserID, "testuser", "Test User", "testuser@example.com", "approved", true, "admin"},
		{"1", "sarah", "Sarah", "sarah@example.com", "approved", true, "admin"},
		{"2", "patrick", "Patrick", "patrick@example.com", "approved", true, "admin"},
		{"admin1", "admin1", "Admin One", "admin1@example.com", "approved", true, "admin"},
	}

	for _, user := range users {
		_, err := db.Exec(`
			INSERT INTO users (id, username, name, email, status, is_admin, role)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (id) DO UPDATE SET
				username = EXCLUDED.username,
				name = EXCLUDED.name,
				email = EXCLUDED.email,
				status = EXCLUDED.status,
				is_admin = EXCLUDED.is_admin,
				role = EXCLUDED.role
		`, user.id, user.username, user.name, user.email, user.status, user.isAdmin, user.role)
		if err != nil {
			return fmt.Errorf("failed to insert test user: %w", err)
		}
	}

	return nil
}
