package migrations

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"
)

// SeedTestData seeds test data for development and testing environments ONLY.
// This function is NEVER executed in production and exists solely to populate
// test environments with sample data.
//
// IMPORTANT: Production data should only be created by users through the application
// or synced from external services like YNAB. This function is strictly for local
// development and PR testing environments.
func SeedTestData(db *sql.DB) error {
	// Check environment variables to ensure we NEVER run this in production
	if isProduction() {
		log.Println("⛔ REFUSING to seed test data in production environment")
		log.Println("SeedTestData is designed for development and testing environments only.")
		return nil
	}

	// Only seed if explicitly requested via RESET_DB, or in development/PR environments
	if !shouldSeedTestData() {
		log.Println("Skipping test data seeding - not explicitly requested and not in dev/PR environment")
		log.Println("To seed test data, set APP_ENV=development or PR_DEPLOYMENT=true or RESET_DB=true")
		return nil
	}

	log.Println("🧪 Seeding TEST DATA for development/PR environment...")
	log.Println("WARNING: This data is for testing purposes only and should not be used in production.")

	// 1. Make sure we have our default users
	defaultUsers := []struct {
		id       string
		username string
		name     string
		role     string
	}{
		{id: "1", username: "sarah", name: "Sarah", role: "superadmin"},
		{id: "2", username: "patrick", name: "Patrick", role: "superadmin"},
		{id: "admin", username: "admin", name: "Admin", role: "admin"},
	}

	for _, user := range defaultUsers {
		// Check if user exists
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM users WHERE id = $1", user.id).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check if user exists: %w", err)
		}

		if count == 0 {
			_, err = db.Exec(`
				INSERT INTO users (id, username, name, role) 
				VALUES ($1, $2, $3, $4)`,
				user.id, user.username, user.name, user.role)
			if err != nil {
				return fmt.Errorf("failed to insert user %s: %w", user.username, err)
			}
		} else {
			// Update existing user to ensure they have the correct role
			_, err = db.Exec(`
				UPDATE users SET role = $1 WHERE id = $2 AND role != $1`,
				user.role, user.id)
			if err != nil {
				return fmt.Errorf("failed to update role for user %s: %w", user.username, err)
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
		_, err := db.Exec(`
			INSERT INTO transactions 
			(id, amount, description, date, type, pay_to, paid, entered_by, user_id) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (id) DO NOTHING
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
		{name: "Housing", description: "Rent, mortgage, repairs", user_id: "1", color: "#2196F3"},
		{name: "Transportation", description: "Car, public transit, gas", user_id: "1", color: "#FFC107"},
		{name: "Entertainment", description: "Movies, games, hobbies", user_id: "2", color: "#9C27B0"},
		{name: "Utilities", description: "Bills and services", user_id: "2", color: "#F44336"},
	}

	for _, cat := range sampleCategories {
		_, err := db.Exec(`
			INSERT INTO categories (name, description, user_id, color) 
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (name, user_id) DO NOTHING
		`, cat.name, cat.description, cat.user_id, cat.color)

		if err != nil {
			return fmt.Errorf("failed to insert category %s: %w", cat.name, err)
		}
	}

	// 4. Add transaction categories (connecting transactions to categories)
	// First get category IDs
	type categoryInfo struct {
		id     int
		name   string
		userId string
	}

	var categories []categoryInfo
	rows, err := db.Query("SELECT id, name, user_id FROM categories")
	if err != nil {
		return fmt.Errorf("failed to query categories: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cat categoryInfo
		if err := rows.Scan(&cat.id, &cat.name, &cat.userId); err != nil {
			return fmt.Errorf("failed to scan category: %w", err)
		}
		categories = append(categories, cat)
	}

	// Connect transactions to categories
	transactionCategories := []struct {
		transactionId string
		categoryName  string
		userId        string
		amount        float64
	}{
		{transactionId: "tx_1", categoryName: "Food", userId: "1", amount: 42.50},
		{transactionId: "tx_2", categoryName: "Housing", userId: "1", amount: 1200.00},
		{transactionId: "tx_3", categoryName: "Utilities", userId: "2", amount: 85.99},
	}

	for _, tc := range transactionCategories {
		// Find matching category
		var categoryId int
		for _, cat := range categories {
			if cat.name == tc.categoryName && cat.userId == tc.userId {
				categoryId = cat.id
				break
			}
		}

		if categoryId > 0 {
			_, err := db.Exec(`
				INSERT INTO transaction_categories (transaction_id, category_id, amount, created_at)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (transaction_id, category_id) DO NOTHING
			`, tc.transactionId, categoryId, tc.amount, time.Now())

			if err != nil {
				return fmt.Errorf("failed to insert transaction category: %w", err)
			}
		}
	}

	// 5. Seed user permissions
	permissionsData := []struct {
		grantedUserId  string
		ownerUserId    string
		resourceType   string
		permissionType string
	}{
		{grantedUserId: "2", ownerUserId: "1", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "1", ownerUserId: "2", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "admin", ownerUserId: "1", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "admin", ownerUserId: "1", resourceType: "transactions", permissionType: "write"},
		{grantedUserId: "admin", ownerUserId: "2", resourceType: "transactions", permissionType: "read"},
	}

	for _, perm := range permissionsData {
		_, err := db.Exec(`
			INSERT INTO permissions (granted_user_id, owner_user_id, resource_type, permission_type)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (granted_user_id, owner_user_id, resource_type, permission_type) DO NOTHING
		`, perm.grantedUserId, perm.ownerUserId, perm.resourceType, perm.permissionType)

		if err != nil {
			return fmt.Errorf("failed to insert permission: %w", err)
		}
	}

	log.Println("Test data seeded successfully")
	return nil
}

// isProduction returns true if we're in a production environment
func isProduction() bool {
	return os.Getenv("APP_ENV") == "production" ||
		os.Getenv("NODE_ENV") == "production" ||
		os.Getenv("ENVIRONMENT") == "production" ||
		os.Getenv("ENV") == "production"
}

// shouldSeedTestData returns true if we should seed test data
func shouldSeedTestData() bool {
	// Explicit override with RESET_DB
	if os.Getenv("RESET_DB") == "true" {
		return true
	}

	// Development environment
	if os.Getenv("APP_ENV") == "development" ||
		os.Getenv("NODE_ENV") == "development" {
		return true
	}

	// PR testing environment
	if os.Getenv("PR_DEPLOYMENT") == "true" {
		return true
	}

	// By default, don't seed test data
	return false
}
