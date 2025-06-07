package migrations

import (
	"database/sql"
	"fmt"
	"log"
)

// FixTransactionUserReferences converts user names to user IDs in transactions table
func FixTransactionUserReferences(db *sql.DB) error {

	// Start a transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Create a mapping of user names to IDs
	userMap := make(map[string]string)
	rows, err := tx.Query("SELECT id, name FROM users")
	if err != nil {
		return fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return fmt.Errorf("failed to scan user row: %w", err)
		}
		userMap[name] = id
		log.Printf("User mapping: %s -> %s", name, id)
	}

	// Update owed_by field where it contains names instead of IDs
	for name, id := range userMap {
		result, err := tx.Exec(`
			UPDATE transactions 
			SET owed_by = $1 
			WHERE owed_by = $2
		`, id, name)
		if err != nil {
			return fmt.Errorf("failed to update owed_by for %s: %w", name, err)
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected > 0 {
			log.Printf("Updated %d transactions where owed_by was '%s' to '%s'", rowsAffected, name, id)
		}
	}

	// Update paid_by field where it contains names instead of IDs
	for name, id := range userMap {
		result, err := tx.Exec(`
			UPDATE transactions 
			SET paid_by = $1 
			WHERE paid_by = $2
		`, id, name)
		if err != nil {
			return fmt.Errorf("failed to update paid_by for %s: %w", name, err)
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected > 0 {
			log.Printf("Updated %d transactions where paid_by was '%s' to '%s'", rowsAffected, name, id)
		}
	}

	// Update entered_by field where it contains names instead of IDs
	for name, id := range userMap {
		result, err := tx.Exec(`
			UPDATE transactions 
			SET entered_by = $1 
			WHERE entered_by = $2
		`, id, name)
		if err != nil {
			return fmt.Errorf("failed to update entered_by for %s: %w", name, err)
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected > 0 {
			log.Printf("Updated %d transactions where entered_by was '%s' to '%s'", rowsAffected, name, id)
		}
	}

	// Verify the updates
	log.Println("Verifying updates...")

	// Check if any names remain in owed_by
	var countOwedBy int
	err = tx.QueryRow(`
		SELECT COUNT(*) FROM transactions 
		WHERE owed_by IN (SELECT name FROM users)
	`).Scan(&countOwedBy)
	if err != nil {
		log.Printf("Warning: Could not verify owed_by updates: %v", err)
	} else if countOwedBy > 0 {
		log.Printf("Warning: %d transactions still have names in owed_by field", countOwedBy)
	}

	// Check if any names remain in paid_by
	var countPaidBy int
	err = tx.QueryRow(`
		SELECT COUNT(*) FROM transactions 
		WHERE paid_by IN (SELECT name FROM users)
	`).Scan(&countPaidBy)
	if err != nil {
		log.Printf("Warning: Could not verify paid_by updates: %v", err)
	} else if countPaidBy > 0 {
		log.Printf("Warning: %d transactions still have names in paid_by field", countPaidBy)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Println("Successfully updated all user references from names to IDs")
	return nil
}
