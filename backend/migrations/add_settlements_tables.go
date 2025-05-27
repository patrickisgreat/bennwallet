package migrations

import (
	"database/sql"
	"fmt"
	"log"
)

// AddSettlementsTables creates tables for transaction offsetting/settlements
func AddSettlementsTables(db *sql.DB) error {
	log.Println("Creating settlements tables")

	_, err := db.Exec(`
		-- Main settlements table
		CREATE TABLE IF NOT EXISTS settlements (
			id TEXT PRIMARY KEY,
			created_by TEXT NOT NULL REFERENCES users(id),
			created_for TEXT NOT NULL REFERENCES users(id),
			total_amount NUMERIC(15,2) NOT NULL,
			remaining_amount NUMERIC(15,2) NOT NULL,
			status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'completed', 'cancelled')),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			completed_at TIMESTAMP WITH TIME ZONE,
			notes TEXT
		);

		-- Settlement items - tracks which transactions are applied to which settlements
		CREATE TABLE IF NOT EXISTS settlement_items (
			id SERIAL PRIMARY KEY,
			settlement_id TEXT NOT NULL REFERENCES settlements(id) ON DELETE CASCADE,
			transaction_id TEXT NOT NULL REFERENCES transactions(id),
			applied_amount NUMERIC(15,2) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			created_by TEXT NOT NULL REFERENCES users(id),
			UNIQUE(settlement_id, transaction_id)
		);

		-- Settlement history for audit trail
		CREATE TABLE IF NOT EXISTS settlement_history (
			id SERIAL PRIMARY KEY,
			settlement_id TEXT NOT NULL REFERENCES settlements(id) ON DELETE CASCADE,
			action TEXT NOT NULL CHECK (action IN ('created', 'transaction_applied', 'transaction_removed', 'completed', 'cancelled')),
			actor_id TEXT NOT NULL REFERENCES users(id),
			transaction_id TEXT REFERENCES transactions(id),
			amount NUMERIC(15,2),
			details JSONB,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		-- Indexes for performance
		CREATE INDEX IF NOT EXISTS idx_settlements_created_by ON settlements(created_by);
		CREATE INDEX IF NOT EXISTS idx_settlements_created_for ON settlements(created_for);
		CREATE INDEX IF NOT EXISTS idx_settlements_status ON settlements(status);
		CREATE INDEX IF NOT EXISTS idx_settlement_items_transaction ON settlement_items(transaction_id);
		CREATE INDEX IF NOT EXISTS idx_settlement_history_settlement ON settlement_history(settlement_id);
		CREATE INDEX IF NOT EXISTS idx_settlement_history_actor ON settlement_history(actor_id);
	`)

	if err != nil {
		return fmt.Errorf("failed to create settlements tables: %w", err)
	}

	log.Println("Settlements tables created successfully")
	return nil
}