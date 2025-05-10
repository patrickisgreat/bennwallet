package database

import (
	"log"
)

// RunMigrations executes necessary database migrations
// Now simplified to create required tables for backward compatibility
func RunMigrations() error {
	log.Println("Migration system has been replaced with consolidated schema approach")
	log.Println("Schema changes are now applied during database initialization")

	// Ensure YNAB tables exist for backward compatibility with tests
	createYNABTables := `
	CREATE TABLE IF NOT EXISTS ynab_config (
		user_id TEXT PRIMARY KEY,
		encrypted_api_token TEXT NOT NULL
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
		hidden BOOLEAN DEFAULT false,
		user_id TEXT NOT NULL REFERENCES users(id)
	);
	
	CREATE TABLE IF NOT EXISTS ynab_categories (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		group_id TEXT REFERENCES ynab_category_groups(id),
		hidden BOOLEAN DEFAULT false,
		budget_amount DECIMAL(15,2),
		user_id TEXT NOT NULL REFERENCES users(id)
	);
	`

	_, err := DB.Exec(createYNABTables)
	return err
}

// MigrationFailed is a helper to determine if a migration command failed
// Kept for backward compatibility
func MigrationFailed(err error) bool {
	if err == nil {
		return false
	}
	log.Printf("Error: %v", err)
	return true
}

// MigrateFromSQLiteToPostgres provides guidance for manual migration
// This is needed only for existing installations transitioning from SQLite
func MigrateFromSQLiteToPostgres() error {
	log.Println("Migration from SQLite to PostgreSQL is a manual process.")
	log.Println("Since you're now using PostgreSQL exclusively, follow these steps:")
	log.Println("1. Export your existing SQLite data:")
	log.Println("   $ sqlite3 your-sqlite-db.db .dump > dump.sql")
	log.Println("2. Convert the SQL syntax to PostgreSQL format")
	log.Println("3. Import the data into your PostgreSQL database:")
	log.Println("   $ psql -h hostname -U username -d dbname -f converted_dump.sql")
	log.Println("4. Update your environment variables to use PostgreSQL")
	log.Println("   $ export DB_HOST=your_postgres_host")
	log.Println("   $ export DB_PORT=5432")
	log.Println("   $ export DB_USER=your_postgres_user")
	log.Println("   $ export DB_PASSWORD=your_postgres_password")
	log.Println("   $ export DB_NAME=your_postgres_db_name")

	return nil
}
