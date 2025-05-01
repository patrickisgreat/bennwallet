package migrations

import (
	"database/sql"
	"log"
)

// AddTransactionDateColumn adds a transaction_date column to the transactions table
func AddTransactionDateColumn(db *sql.DB) error {
	log.Println("Adding transaction_date column to transactions table...")

	// Add the column
	_, err := db.Exec(`
		ALTER TABLE transactions 
		ADD COLUMN transaction_date DATETIME;
	`)
	if err != nil {
		log.Printf("Error adding transaction_date column: %v", err)
		return err
	}

	// Initialize the column with date values
	_, err = db.Exec(`
		UPDATE transactions 
		SET transaction_date = date 
		WHERE transaction_date IS NULL;
	`)
	if err != nil {
		log.Printf("Error initializing transaction_date values: %v", err)
		return err
	}

	log.Println("Transaction date column added successfully")
	return nil
}
