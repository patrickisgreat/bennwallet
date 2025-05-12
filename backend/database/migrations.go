package database

import (
	"log"
)

// RunMigrations executes necessary database migrations
// Now simplified to create required tables for backward compatibility
func RunMigrations() error {
	log.Println("Schema changes are now applied during database initialization")
	log.Println("All migrations have been consolidated into the base schema")
	log.Println("No migrations will be performed - database is created fresh")

	return nil
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
