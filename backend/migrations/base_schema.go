package migrations

import (
	"database/sql"
	"fmt"
	"log"
	"os"
)

// CreateBaseSchema creates all the base tables needed for the application
func CreateBaseSchema(db *sql.DB) error {
	// Check if we're in a test environment
	isTest := os.Getenv("GO_ENV") == "test"

	// For tests, we want to drop and recreate tables
	if isTest {
		if err := DropAllTables(db); err != nil {
			return fmt.Errorf("failed to drop tables for test: %w", err)
		}
	}

	// Create UUID extension if it doesn't exist
	_, err := db.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`)
	if err != nil {
		return fmt.Errorf("failed to create uuid-ossp extension: %w", err)
	}

	// Create base tables
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			role TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			is_admin BOOLEAN NOT NULL DEFAULT false
		);

		CREATE TABLE IF NOT EXISTS transactions (
			id TEXT PRIMARY KEY,
			amount NUMERIC(15,2) NOT NULL,
			description TEXT NOT NULL,
			date TEXT NOT NULL,
			transaction_date TEXT,
			type TEXT NOT NULL,
			pay_to TEXT,
			paid BOOLEAN NOT NULL DEFAULT FALSE,
			paid_date TEXT,
			optional BOOLEAN NOT NULL DEFAULT FALSE,
			entered_by TEXT NOT NULL,
			user_id TEXT NOT NULL REFERENCES users(id),
			note TEXT,
			entered TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			paid_by TEXT REFERENCES users(id),
			owed_by TEXT REFERENCES users(id)
		);

		CREATE TABLE IF NOT EXISTS settlements (
			id TEXT PRIMARY KEY,
			creator_id TEXT NOT NULL REFERENCES users(id),
			recipient_id TEXT NOT NULL REFERENCES users(id),
			total_amount NUMERIC(15,2) NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			notes TEXT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			completed_at TIMESTAMP WITH TIME ZONE
		);

		CREATE TABLE IF NOT EXISTS settlement_items (
			id SERIAL PRIMARY KEY,
			settlement_id TEXT NOT NULL REFERENCES settlements(id) ON DELETE CASCADE,
			transaction_id TEXT NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
			applied_amount NUMERIC(15,2) NOT NULL,
			created_by TEXT NOT NULL REFERENCES users(id),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(settlement_id, transaction_id)
		);

		CREATE INDEX IF NOT EXISTS idx_settlement_items_settlement_id ON settlement_items(settlement_id);
		CREATE INDEX IF NOT EXISTS idx_settlement_items_transaction_id ON settlement_items(transaction_id);
		CREATE INDEX IF NOT EXISTS idx_settlement_items_created_by ON settlement_items(created_by);

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

		CREATE INDEX IF NOT EXISTS idx_settlement_history_settlement_id ON settlement_history(settlement_id);
		CREATE INDEX IF NOT EXISTS idx_settlement_history_actor_id ON settlement_history(actor_id);
		CREATE INDEX IF NOT EXISTS idx_settlement_history_transaction_id ON settlement_history(transaction_id);

		CREATE TABLE IF NOT EXISTS settlement_transactions (
			id SERIAL PRIMARY KEY,
			settlement_id TEXT NOT NULL REFERENCES settlements(id) ON DELETE CASCADE,
			transaction_id TEXT NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
			amount NUMERIC(15,2) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(settlement_id, transaction_id)
		);

		CREATE TABLE IF NOT EXISTS permissions (
			id SERIAL PRIMARY KEY,
			granted_user_id TEXT NOT NULL REFERENCES users(id),
			owner_user_id TEXT NOT NULL REFERENCES users(id),
			resource_type TEXT NOT NULL,
			permission_type TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP WITH TIME ZONE,
			UNIQUE(granted_user_id, owner_user_id, resource_type, permission_type)
		);

		CREATE TABLE IF NOT EXISTS ynab_config (
			id SERIAL PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id),
			encrypted_api_token TEXT,
			encrypted_budget_id TEXT,
			encrypted_account_id TEXT,
			api_token TEXT,
			budget_id TEXT,
			account_id TEXT,
			last_sync_time TIMESTAMP,
			sync_frequency INTEGER DEFAULT 60,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			has_credentials BOOLEAN DEFAULT FALSE,
			UNIQUE(user_id)
		);

		CREATE TABLE IF NOT EXISTS user_ynab_settings (
			user_id TEXT PRIMARY KEY REFERENCES users(id),
			token TEXT,
			budget_id TEXT,
			account_id TEXT,
			auto_import BOOLEAN DEFAULT false,
			sync_enabled BOOLEAN DEFAULT false,
			last_synced TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS ynab_category_groups (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			category_group_id TEXT NOT NULL,
			hidden BOOLEAN DEFAULT false,
			user_id TEXT NOT NULL REFERENCES users(id),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			last_updated TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS ynab_categories (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			group_id TEXT REFERENCES ynab_category_groups(id),
			category_group_id TEXT REFERENCES ynab_category_groups(id),
			hidden BOOLEAN DEFAULT false,
			budget_amount DECIMAL(15,2),
			user_id TEXT NOT NULL REFERENCES users(id),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			last_updated TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS transaction_categories (
			id SERIAL PRIMARY KEY,
			transaction_id TEXT NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
			category_id TEXT NOT NULL REFERENCES ynab_categories(id) ON DELETE CASCADE,
			amount NUMERIC(15,2) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(transaction_id, category_id)
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create base schema: %w", err)
	}

	log.Println("Base schema created successfully")
	return nil
}

// DropAllTables drops all tables in the database
func DropAllTables(db *sql.DB) error {
	_, err := db.Exec(`
		DO $$ 
		DECLARE
			r RECORD;
		BEGIN
			-- Disable foreign key checks during table deletion
			EXECUTE 'SET CONSTRAINTS ALL DEFERRED';
			
			-- Drop all tables in the public schema
			FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = 'public') LOOP
				EXECUTE 'DROP TABLE IF EXISTS ' || quote_ident(r.tablename) || ' CASCADE';
			END LOOP;
			
			-- Re-enable foreign key checks
			EXECUTE 'SET CONSTRAINTS ALL IMMEDIATE';
		END $$;
	`)
	if err != nil {
		return fmt.Errorf("failed to drop tables: %w", err)
	}

	log.Println("All tables dropped successfully")
	return nil
}
