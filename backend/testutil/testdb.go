package testutil

import (
	"bennwallet/backend/migrations"
	"database/sql"
	"fmt"
	"strings"
	"testing"
)

// TestDBConfig holds test database configuration
type TestDBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// ConnectionString builds a PostgreSQL connection string
func (c TestDBConfig) ConnectionString() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode)
}

// SetupTestDB creates a new test database for PostgreSQL testing
func SetupTestDB(t *testing.T) (*sql.DB, func()) {
	// Use the shared GetTestDBConfig from testutil.go
	postgresConfig := GetTestDBConfig()
	config := TestDBConfig{
		Host:     postgresConfig.Host,
		Port:     postgresConfig.Port,
		User:     postgresConfig.User,
		Password: postgresConfig.Password,
		DBName:   postgresConfig.DBName,
		SSLMode:  postgresConfig.SSLMode,
	}

	// Connect to postgres to create/drop test database
	mainDB, err := sql.Open("postgres", fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=%s",
		config.Host, config.Port, config.User, config.Password, config.SSLMode))
	if err != nil {
		t.Fatalf("Failed to connect to postgres: %v", err)
	}

	// Try to create the database, ignore error if it already exists
	_, err = mainDB.Exec(fmt.Sprintf("CREATE DATABASE %s", config.DBName))
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Close connection to postgres
	if err := mainDB.Close(); err != nil {
		t.Fatalf("Failed to close postgres connection: %v", err)
	}

	// Connect to test database
	db, err := sql.Open("postgres", config.ConnectionString())
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Run all migrations (this will create schema and apply all migrations)
	if err := migrations.RunMigrations(db, true); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Return cleanup function
	cleanup := func() {
		if err := db.Close(); err != nil {
			t.Errorf("Failed to close test database connection: %v", err)
		}

		// Reconnect to postgres to drop test database
		mainDB, err := sql.Open("postgres", fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=%s",
			config.Host, config.Port, config.User, config.Password, config.SSLMode))
		if err != nil {
			t.Errorf("Failed to connect to postgres for cleanup: %v", err)
			return
		}
		defer mainDB.Close()

		// Terminate existing connections
		_, err = mainDB.Exec(fmt.Sprintf(`
			SELECT pg_terminate_backend(pg_stat_activity.pid)
			FROM pg_stat_activity
			WHERE pg_stat_activity.datname = '%s'
			AND pid <> pg_backend_pid()
		`, config.DBName))
		if err != nil {
			t.Errorf("Failed to terminate connections during cleanup: %v", err)
			return
		}

		// Drop test database
		_, err = mainDB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", config.DBName))
		if err != nil {
			t.Errorf("Failed to drop test database during cleanup: %v", err)
		}
	}

	return db, cleanup
}
