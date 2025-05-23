package testutil

import (
	"database/sql"
	"testing"

	"bennwallet/backend/database"
	"bennwallet/backend/migrations"
)

// SetupTestDB creates a test database and seeds it with test data
func SetupTestDB(t testing.TB) (*sql.DB, func()) {
	// Create a test PostgreSQL database
	testConfig := database.PostgresConfig{
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
	if err := database.CreatePostgresSchema(db); err != nil {
		t.Fatalf("Failed to create test schema: %v", err)
	}

	// Run migrations to create all necessary tables
	if err := migrations.RunMigrations(db, true); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Seed test data
	if err := SeedTestData(db); err != nil {
		t.Fatalf("Failed to seed test data: %v", err)
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
