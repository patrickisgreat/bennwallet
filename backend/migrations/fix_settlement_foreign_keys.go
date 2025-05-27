package migrations

import (
	"database/sql"
	"log"
)

// FixSettlementForeignKeys adds ON DELETE CASCADE to settlement_items transaction_id foreign key
func FixSettlementForeignKeys(db *sql.DB) error {
	log.Println("Fixing settlement foreign key constraints...")

	// Drop the existing foreign key constraint
	_, err := db.Exec(`
		ALTER TABLE settlement_items 
		DROP CONSTRAINT IF EXISTS settlement_items_transaction_id_fkey;
	`)
	if err != nil {
		log.Printf("Error dropping existing constraint: %v", err)
		// Continue anyway, it might not exist
	}

	// Add the constraint back with ON DELETE CASCADE
	_, err = db.Exec(`
		ALTER TABLE settlement_items 
		ADD CONSTRAINT settlement_items_transaction_id_fkey 
		FOREIGN KEY (transaction_id) 
		REFERENCES transactions(id) 
		ON DELETE CASCADE;
	`)
	if err != nil {
		log.Printf("Error adding new constraint: %v", err)
		return err
	}

	// Also fix settlement_history to cascade delete
	_, err = db.Exec(`
		ALTER TABLE settlement_history 
		DROP CONSTRAINT IF EXISTS settlement_history_transaction_id_fkey;
	`)
	if err != nil {
		log.Printf("Error dropping history constraint: %v", err)
		// Continue anyway
	}

	_, err = db.Exec(`
		ALTER TABLE settlement_history 
		ADD CONSTRAINT settlement_history_transaction_id_fkey 
		FOREIGN KEY (transaction_id) 
		REFERENCES transactions(id) 
		ON DELETE CASCADE;
	`)
	if err != nil {
		log.Printf("Error adding history constraint: %v", err)
		return err
	}

	log.Println("Settlement foreign key constraints fixed successfully")
	return nil
}
