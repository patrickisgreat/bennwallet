package database

import (
	"database/sql"
	"fmt"
	"log"
)

// CreateSchema creates the database schema for PostgreSQL
func CreateSchema(db *sql.DB) error {
	log.Printf("Creating schema for PostgreSQL database")

	err := createPostgresSchema(db)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Record the schema creation in the migrations table
	if err := recordMigration(db, "base_schema", "postgres"); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	return nil
}

// createPostgresSchema creates all the tables needed for PostgreSQL
func createPostgresSchema(db *sql.DB) error {
	// This is now available in postgres_schema.go
	return CreatePostgresSchema(db)
}

// recordMigration records a migration in the migrations table
func recordMigration(db *sql.DB, name string, dbType string) error {
	query := `
		INSERT INTO migrations (name) 
		VALUES ($1)
		ON CONFLICT (name) DO NOTHING
	`
	_, err := db.Exec(query, name)
	return err
}

// GetColumnNames returns all column names for a given table
func GetColumnNames(db *sql.DB, tableName string) ([]string, error) {
	query := `
		SELECT column_name 
		FROM information_schema.columns 
		WHERE table_name = $1
		ORDER BY ordinal_position
	`

	rows, err := db.Query(query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var colName string
		if err := rows.Scan(&colName); err != nil {
			return nil, err
		}
		columns = append(columns, colName)
	}

	return columns, nil
}

// CheckColumnExists checks if a column exists in a table
func CheckColumnExists(db *sql.DB, tableName, columnName string) (bool, error) {
	columns, err := GetColumnNames(db, tableName)
	if err != nil {
		return false, err
	}

	for _, col := range columns {
		if col == columnName {
			return true, nil
		}
	}

	return false, nil
}

// AddColumn adds a column to a table if it doesn't exist
func AddColumn(db *sql.DB, tableName, columnName, columnType string) error {
	exists, err := CheckColumnExists(db, tableName, columnName)
	if err != nil {
		return err
	}

	if !exists {
		query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", tableName, columnName, columnType)
		_, err = db.Exec(query)
		if err != nil {
			return err
		}
	}

	return nil
}
