package migrations

import (
	"database/sql"
	"fmt"
	"log"
)

// CreateBaseSchema creates all the base tables needed for the application
func CreateBaseSchema(db *sql.DB) error {
	// Create base tables
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			role TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS transactions (
			id TEXT PRIMARY KEY,
			amount NUMERIC(15,2) NOT NULL,
			description TEXT NOT NULL,
			date TEXT NOT NULL,
			type TEXT NOT NULL,
			pay_to TEXT,
			paid BOOLEAN NOT NULL DEFAULT FALSE,
			entered_by TEXT NOT NULL,
			user_id TEXT NOT NULL REFERENCES users(id)
		);

		CREATE TABLE IF NOT EXISTS categories (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			user_id TEXT NOT NULL REFERENCES users(id),
			color TEXT,
			UNIQUE(name, user_id)
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
			user_id TEXT NOT NULL UNIQUE REFERENCES users(id),
			api_token TEXT NOT NULL,
			budget_id TEXT NOT NULL,
			account_id TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS user_ynab_settings (
			id SERIAL PRIMARY KEY,
			user_id TEXT NOT NULL UNIQUE REFERENCES users(id),
			last_sync TIMESTAMP WITH TIME ZONE
		);

		CREATE TABLE IF NOT EXISTS ynab_category_groups (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			category_group_id TEXT NOT NULL,
			user_id TEXT NOT NULL REFERENCES users(id)
		);

		CREATE TABLE IF NOT EXISTS ynab_categories (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			category_group_id TEXT NOT NULL REFERENCES ynab_category_groups(id),
			user_id TEXT NOT NULL REFERENCES users(id)
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
