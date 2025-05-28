package migrations

import (
	"database/sql"
	"log"
)

// RestructureTransactionDebtTracking adds owed_by field and renames pay_to to paid_by
func RestructureTransactionDebtTracking(db *sql.DB) error {
	log.Println("Starting transaction debt tracking restructure migration...")

	// Add the new owed_by column
	_, err := db.Exec(`
		ALTER TABLE transactions 
		ADD COLUMN IF NOT EXISTS owed_by TEXT
	`)
	if err != nil {
		log.Printf("Error adding owed_by column: %v", err)
		return err
	}
	log.Println("Added owed_by column to transactions table")

	// Rename pay_to to paid_by
	// First check if pay_to exists and paid_by doesn't
	var hasPayTo bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'transactions' AND column_name = 'pay_to'
		)
	`).Scan(&hasPayTo)
	if err != nil {
		log.Printf("Error checking for pay_to column: %v", err)
		return err
	}

	var hasPaidBy bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'transactions' AND column_name = 'paid_by'
		)
	`).Scan(&hasPaidBy)
	if err != nil {
		log.Printf("Error checking for paid_by column: %v", err)
		return err
	}

	if hasPayTo && !hasPaidBy {
		// PostgreSQL syntax for renaming column
		_, err = db.Exec(`ALTER TABLE transactions RENAME COLUMN pay_to TO paid_by`)
		if err != nil {
			log.Printf("Error renaming pay_to to paid_by: %v", err)
			return err
		}
		log.Println("Renamed pay_to column to paid_by")
	}

	// Migrate existing data - set owed_by to the opposite of paid_by for existing transactions
	// This assumes that in the old system, pay_to meant who should pay (which is backwards)
	// So we'll swap the logic: paid_by = entered_by, owed_by = old pay_to value
	_, err = db.Exec(`
		UPDATE transactions 
		SET owed_by = paid_by,
		    paid_by = entered_by
		WHERE owed_by IS NULL
	`)
	if err != nil {
		log.Printf("Error migrating existing data: %v", err)
		return err
	}
	log.Println("Migrated existing transaction data to new structure")

	log.Println("Transaction debt tracking restructure migration completed successfully")
	return nil
}
