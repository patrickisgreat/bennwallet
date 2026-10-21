package migrations

import (
	"database/sql"
	"fmt"
	"log"

	"bennwallet/backend/database"
)

// migration is a named, idempotent schema or data change applied once and recorded in the migrations table.
type migration struct {
	name string
	fn   func(*sql.DB) error
}

// registry lists every migration in the order it must be applied.
var registry = []migration{
	{"add_transaction_notes_column", database.AddTransactionNotesColumn},
	{"add_ynab_categories_table", AddYNABCategoriesTable},
	{"fix_ynab_categories_column", FixYNABCategoriesColumn},
	{"add_settlements_tables", AddSettlementsTables},
	{"restructure_transaction_debt_tracking", RestructureTransactionDebtTracking},
	{"seed_test_data", SeedTestData},
	{"seed_settlement_test_data", AddSettlementTestData},
	{"update_seed_data_for_debt_tracking", UpdateSeedDataForDebtTracking},
	{"fix_settlement_foreign_keys", FixSettlementForeignKeys},
	{"fix_transaction_user_references", FixTransactionUserReferences},
}

// seedMigrations are development-only data seeds that must never run in production.
var seedMigrations = map[string]bool{
	"seed_test_data":                     true,
	"seed_settlement_test_data":          true,
	"update_seed_data_for_debt_tracking": true,
}

// RunMigrations applies every unapplied migration in order.
// When isResetDB is true, all tables are dropped and the base schema recreated first.
func RunMigrations(db *sql.DB, isResetDB bool) error {
	if isResetDB {
		log.Println("Resetting database before migrations...")
		if err := DropAllTables(db); err != nil {
			return fmt.Errorf("failed to drop tables: %w", err)
		}
		if err := CreateBaseSchema(db); err != nil {
			return fmt.Errorf("failed to create base schema: %w", err)
		}
	}

	if err := ensureMigrationsTable(db); err != nil {
		return err
	}

	applied, err := appliedMigrations(db)
	if err != nil {
		return err
	}

	production := isProduction()
	for _, m := range registry {
		if production && seedMigrations[m.name] {
			log.Printf("Skipping %s in production", m.name)
			continue
		}
		if applied[m.name] {
			continue
		}

		log.Printf("Applying migration: %s", m.name)
		if err := m.fn(db); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", m.name, err)
		}
		if _, err := db.Exec("INSERT INTO migrations (name) VALUES ($1)", m.name); err != nil {
			return fmt.Errorf("failed to record migration %s: %w", m.name, err)
		}
	}

	log.Println("All migrations completed successfully")
	return nil
}

// GetPendingMigrations returns the names of registered migrations not yet applied.
func GetPendingMigrations(db *sql.DB) ([]string, error) {
	if err := ensureMigrationsTable(db); err != nil {
		return nil, err
	}

	applied, err := appliedMigrations(db)
	if err != nil {
		return nil, err
	}

	var pending []string
	for _, m := range registry {
		if !applied[m.name] {
			pending = append(pending, m.name)
		}
	}
	return pending, nil
}

func ensureMigrationsTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS migrations (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}
	return nil
}

func appliedMigrations(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query("SELECT name FROM migrations")
	if err != nil {
		return nil, fmt.Errorf("failed to query migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan migration name: %w", err)
		}
		applied[name] = true
	}
	return applied, rows.Err()
}
