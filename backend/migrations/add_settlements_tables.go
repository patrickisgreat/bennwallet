package migrations

import (
	"database/sql"
	"fmt"
	"log"
)

// AddSettlementsTables creates tables for transaction offsetting/settlements
func AddSettlementsTables(db *sql.DB) error {
	log.Println("Creating settlements tables")

	// Check if settlements table already exists with the base schema
	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'settlements'
		)
	`).Scan(&exists)

	if err != nil {
		return fmt.Errorf("failed to check if settlements table exists: %w", err)
	}

	if exists {
		// Tables already exist from base schema, check if we need to add any missing columns
		log.Println("Settlements tables already exist, checking for missing columns")

		// Add remaining_amount column if it doesn't exist
		_, err = db.Exec(`
			ALTER TABLE settlements 
			ADD COLUMN IF NOT EXISTS remaining_amount NUMERIC(15,2);
		`)
		if err != nil {
			return fmt.Errorf("failed to add remaining_amount column: %w", err)
		}

		// Update remaining_amount to match total_amount for existing rows
		_, err = db.Exec(`
			UPDATE settlements 
			SET remaining_amount = total_amount 
			WHERE remaining_amount IS NULL;
		`)
		if err != nil {
			return fmt.Errorf("failed to update remaining_amount values: %w", err)
		}

		// Make remaining_amount NOT NULL after setting values
		_, err = db.Exec(`
			ALTER TABLE settlements 
			ALTER COLUMN remaining_amount SET NOT NULL;
		`)
		if err != nil {
			return fmt.Errorf("failed to set remaining_amount NOT NULL: %w", err)
		}

		log.Println("Settlements tables updated successfully")
		return nil
	}

	// If tables don't exist, this shouldn't happen as base schema should create them
	// But let's create them anyway with the correct column names matching the base schema
	_, err = db.Exec(`
		-- Main settlements table
		CREATE TABLE IF NOT EXISTS settlements (
			id TEXT PRIMARY KEY,
			creator_id TEXT NOT NULL REFERENCES users(id),
			recipient_id TEXT NOT NULL REFERENCES users(id),
			total_amount NUMERIC(15,2) NOT NULL,
			remaining_amount NUMERIC(15,2) NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			notes TEXT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			completed_at TIMESTAMP WITH TIME ZONE
		);

		-- Settlement items - tracks which transactions are applied to which settlements
		CREATE TABLE IF NOT EXISTS settlement_items (
			id SERIAL PRIMARY KEY,
			settlement_id TEXT NOT NULL REFERENCES settlements(id) ON DELETE CASCADE,
			transaction_id TEXT NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
			applied_amount NUMERIC(15,2) NOT NULL,
			created_by TEXT NOT NULL REFERENCES users(id),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(settlement_id, transaction_id)
		);

		-- Settlement history for audit trail
		CREATE TABLE IF NOT EXISTS settlement_history (
			id SERIAL PRIMARY KEY,
			settlement_id TEXT NOT NULL REFERENCES settlements(id) ON DELETE CASCADE,
			action TEXT NOT NULL,
			actor_id TEXT NOT NULL REFERENCES users(id),
			transaction_id TEXT REFERENCES transactions(id),
			amount NUMERIC(15,2),
			details JSONB,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		-- Settlement transactions junction table
		CREATE TABLE IF NOT EXISTS settlement_transactions (
			id SERIAL PRIMARY KEY,
			settlement_id TEXT NOT NULL REFERENCES settlements(id) ON DELETE CASCADE,
			transaction_id TEXT NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
			amount NUMERIC(15,2) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(settlement_id, transaction_id)
		);

		-- Indexes for performance
		CREATE INDEX IF NOT EXISTS idx_settlements_creator_id ON settlements(creator_id);
		CREATE INDEX IF NOT EXISTS idx_settlements_recipient_id ON settlements(recipient_id);
		CREATE INDEX IF NOT EXISTS idx_settlements_status ON settlements(status);
		CREATE INDEX IF NOT EXISTS idx_settlement_items_transaction_id ON settlement_items(transaction_id);
		CREATE INDEX IF NOT EXISTS idx_settlement_history_settlement_id ON settlement_history(settlement_id);
		CREATE INDEX IF NOT EXISTS idx_settlement_history_actor_id ON settlement_history(actor_id);
	`)

	if err != nil {
		return fmt.Errorf("failed to create settlements tables: %w", err)
	}

	log.Println("Settlements tables created successfully")
	return nil
}
