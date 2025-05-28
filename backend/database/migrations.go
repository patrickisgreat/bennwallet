package database

import (
	"database/sql"
	"fmt"
	"log"
)

// RunMigrations checks if migrations are needed and runs them
func RunMigrations(db *sql.DB) error {
	// First, check if the migrations table exists
	exists, err := tableExists(db, "migrations")
	if err != nil {
		return fmt.Errorf("failed to check migrations table: %w", err)
	}

	// If the migrations table doesn't exist, create schema from scratch
	if !exists {
		log.Println("No migrations table found. Creating schema from scratch.")
		return CreateSchema(db)
	}

	// Check if base schema has been applied
	applied, err := checkMigrationApplied(db, "base_schema")
	if err != nil {
		return fmt.Errorf("failed to check if base schema was applied: %w", err)
	}

	// If base schema hasn't been applied, create it
	if !applied {
		log.Println("Base schema not found. Creating schema from scratch.")
		return CreateSchema(db)
	}

	log.Println("Database schema is up to date.")
	return nil
}

// tableExists checks if a table exists in the database
func tableExists(db *sql.DB, tableName string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = $1
		);
	`
	var exists bool
	err := db.QueryRow(query, tableName).Scan(&exists)
	return exists, err
}

// checkMigrationApplied checks if a specific migration has been applied
func checkMigrationApplied(db *sql.DB, migrationName string) (bool, error) {
	// First check if the migrations table exists
	exists, err := tableExists(db, "migrations")
	if err != nil {
		return false, err
	}

	if !exists {
		return false, nil
	}

	// Check if the migration has been applied
	query := `
		SELECT EXISTS (
			SELECT FROM migrations
			WHERE name = $1
		);
	`
	var applied bool
	err = db.QueryRow(query, migrationName).Scan(&applied)
	return applied, err
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
