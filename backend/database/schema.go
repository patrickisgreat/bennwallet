package database

import (
	"database/sql"
	"fmt"
	"log"
)

// AddTransactionNotesColumn adds the note column to the transactions table if it doesn't exist
func AddTransactionNotesColumn(db *sql.DB) error {
	log.Printf("Adding note column to transactions table if it doesn't exist")

	// Check if the column already exists
	exists, err := CheckColumnExists(db, "transactions", "note")
	if err != nil {
		return fmt.Errorf("failed to check if note column exists: %w", err)
	}

	if !exists {
		// Add the note column
		_, err = db.Exec(`ALTER TABLE transactions ADD COLUMN note TEXT`)
		if err != nil {
			return fmt.Errorf("failed to add note column to transactions table: %w", err)
		}
		log.Printf("Added note column to transactions table")
	} else {
		log.Printf("Note column already exists in transactions table")
	}

	return nil
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
