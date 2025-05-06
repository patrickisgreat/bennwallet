package migrations

import (
	"database/sql"
	"fmt"
	"log"
	"os"
)

// SeedTestData seeds test data for development and PR environments
// This should only be called in non-production environments
func SeedTestData(db *sql.DB) error {
	// Check if we're in production - we should NEVER run this in production
	if os.Getenv("APP_ENV") == "production" || os.Getenv("NODE_ENV") == "production" {
		log.Println("Refusing to seed test data in production environment")
		return nil
	}

	// Only seed if explicitly requested or in dev/PR environment
	if os.Getenv("RESET_DB") != "true" &&
		os.Getenv("APP_ENV") != "development" &&
		os.Getenv("PR_DEPLOYMENT") != "true" {
		log.Println("Skipping test data seeding - not explicitly requested and not in dev/PR environment")
		return nil
	}

	log.Println("Seeding test data for development/PR environment...")

	// Start a transaction for all operations
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Clear existing data (make sure this is only done in dev)
	tables := []string{"transactions", "categories", "permissions", "ynab_config", "user_ynab_settings", "ynab_category_groups", "ynab_categories"}
	for _, table := range tables {
		// Check if table exists first
		var exists int
		err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check if table %s exists: %w", table, err)
		}

		if exists > 0 {
			_, err = tx.Exec("DELETE FROM " + table)
			if err != nil {
				return fmt.Errorf("failed to clear table %s: %w", table, err)
			}
		}
	}

	// 1. Make sure we have our default users
	// First check if we need to clear the users table too
	if os.Getenv("RESET_DB") == "true" {
		_, err = tx.Exec("DELETE FROM users")
		if err != nil {
			return fmt.Errorf("failed to clear users table: %w", err)
		}
	}

	// Insert default users if they don't exist
	defaultUsers := []struct {
		id       string
		username string
		name     string
		role     string
	}{
		{id: "1", username: "sarah", name: "Sarah", role: "user"},
		{id: "2", username: "patrick", name: "Patrick", role: "user"},
		{id: "admin", username: "admin", name: "Admin", role: "admin"},
	}

	for _, user := range defaultUsers {
		// Check if user exists
		var count int
		err = tx.QueryRow("SELECT COUNT(*) FROM users WHERE id = ?", user.id).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check if user exists: %w", err)
		}

		if count == 0 {
			_, err = tx.Exec("INSERT INTO users (id, username, name, role) VALUES (?, ?, ?, ?)",
				user.id, user.username, user.name, user.role)
			if err != nil {
				return fmt.Errorf("failed to insert user %s: %w", user.username, err)
			}
		}
	}

	// 2. Seed sample transaction data
	sampleTransactions := []struct {
		id          string
		amount      float64
		description string
		date        string
		txType      string
		payTo       string
		paid        bool
		enteredBy   string
		userId      string
	}{
		{
			id:          "tx_1",
			amount:      42.50,
			description: "Groceries",
			date:        "2023-08-15",
			txType:      "expense",
			payTo:       "Supermarket",
			paid:        true,
			enteredBy:   "1",
			userId:      "1",
		},
		{
			id:          "tx_2",
			amount:      1200.00,
			description: "Rent",
			date:        "2023-08-01",
			txType:      "expense",
			payTo:       "Landlord",
			paid:        true,
			enteredBy:   "1",
			userId:      "1",
		},
		{
			id:          "tx_3",
			amount:      85.99,
			description: "Internet bill",
			date:        "2023-08-10",
			txType:      "expense",
			payTo:       "ISP",
			paid:        false,
			enteredBy:   "2",
			userId:      "2",
		},
		{
			id:          "tx_4",
			amount:      2500.00,
			description: "Salary",
			date:        "2023-08-25",
			txType:      "income",
			payTo:       "Employer",
			paid:        true,
			enteredBy:   "2",
			userId:      "2",
		},
	}

	for _, tx := range sampleTransactions {
		_, err = db.Exec(`
			INSERT INTO transactions 
			(id, amount, description, date, type, payTo, paid, enteredBy, userId) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, tx.id, tx.amount, tx.description, tx.date, tx.txType, tx.payTo, tx.paid, tx.enteredBy, tx.userId)

		if err != nil {
			return fmt.Errorf("failed to insert transaction %s: %w", tx.id, err)
		}
	}

	// 3. Add some sample categories
	sampleCategories := []struct {
		name        string
		description string
		user_id     string
		color       string
	}{
		{name: "Food", description: "Groceries and dining out", user_id: "1", color: "#4CAF50"},
		{name: "Housing", description: "Rent and utilities", user_id: "1", color: "#2196F3"},
		{name: "Transportation", description: "Car and public transit", user_id: "1", color: "#FFC107"},
		{name: "Entertainment", description: "Movies and fun", user_id: "2", color: "#9C27B0"},
		{name: "Utilities", description: "Bills and services", user_id: "2", color: "#F44336"},
	}

	for _, cat := range sampleCategories {
		_, err = db.Exec(`
			INSERT INTO categories (name, description, user_id, color) 
			VALUES (?, ?, ?, ?)
		`, cat.name, cat.description, cat.user_id, cat.color)

		if err != nil {
			return fmt.Errorf("failed to insert category %s: %w", cat.name, err)
		}
	}

	// 4. Add sample permissions
	samplePermissions := []struct {
		id              string
		owner_user_id   string
		granted_user_id string
		permission_type string
		resource_type   string
	}{
		{id: "perm_1", owner_user_id: "1", granted_user_id: "2", permission_type: "read", resource_type: "transactions"},
		{id: "perm_2", owner_user_id: "2", granted_user_id: "1", permission_type: "write", resource_type: "transactions"},
	}

	for _, perm := range samplePermissions {
		_, err = db.Exec(`
			INSERT INTO permissions (id, owner_user_id, granted_user_id, permission_type, resource_type) 
			VALUES (?, ?, ?, ?, ?)
		`, perm.id, perm.owner_user_id, perm.granted_user_id, perm.permission_type, perm.resource_type)

		if err != nil {
			return fmt.Errorf("failed to insert permission %s: %w", perm.id, err)
		}
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Println("Test data seeded successfully")
	return nil
}
