package database

import (
	"database/sql"
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

// SeedDefaultUsers seeds default users (proxy for backward compatibility)
func SeedDefaultUsers() error {
	// This function is kept for backward compatibility
	// Default users are now seeded as part of SeedDefaultData
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

	// Create schema
	if err := CreatePostgresSchema(db); err != nil {
		t.Fatalf("Failed to create test schema: %v", err)
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
