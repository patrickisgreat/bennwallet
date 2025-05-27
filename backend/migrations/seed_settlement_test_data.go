package migrations

import (
	"database/sql"
	"fmt"
	"log"
)

// AddSettlementTestData adds test transactions that create realistic debt scenarios for testing settlements
func AddSettlementTestData(db *sql.DB) error {
	// Only run in development/test environments
	if isProduction() {
		log.Println("⛔ REFUSING to seed settlement test data in production environment")
		return nil
	}

	log.Println("🧪 Adding settlement test data...")

	// Clear existing test transactions to avoid duplicates
	_, err := db.Exec(`
		DELETE FROM transactions 
		WHERE id LIKE 'settlement_test_%'
	`)
	if err != nil {
		log.Printf("Warning: Failed to clear existing settlement test data: %v", err)
	}

	// Create settlement test transactions
	settlementTransactions := []struct {
		id               string
		amount           float64
		description      string
		date             string
		transaction_date string
		txType           string
		payTo            string // User name as displayed in UI
		paid             bool
		paid_date        string
		optional         bool
		enteredBy        string // User ID who entered the transaction
		userId           string
		categoryName     string
		note             string
	}{
		// Scenario 1: Admin user owes Sarah $150 for groceries
		{
			id:               "settlement_test_1",
			amount:           150.00,
			description:      "Groceries for the week",
			date:             "2025-05-25",
			transaction_date: "2025-05-25",
			txType:           "expense",
			payTo:            "Admin User", // Admin owes this
			paid:             false,
			paid_date:        "",
			optional:         false,
			enteredBy:        "1", // Sarah entered it
			userId:           "1",
			categoryName:     "Food",
			note:             "Admin's share of weekly groceries",
		},
		// Scenario 2: Sarah owes Admin $75 for utilities
		{
			id:               "settlement_test_2",
			amount:           75.00,
			description:      "Electric bill",
			date:             "2025-05-24",
			transaction_date: "2025-05-24",
			txType:           "expense",
			payTo:            "Sarah", // Sarah owes this
			paid:             false,
			paid_date:        "",
			optional:         false,
			enteredBy:        "admin-user-1", // Admin entered it
			userId:           "admin-user-1",
			categoryName:     "Utilities",
			note:             "Sarah's half of electric bill",
		},
		// Scenario 3: Admin owes Sarah another $50 for dinner
		{
			id:               "settlement_test_3",
			amount:           50.00,
			description:      "Dinner at restaurant",
			date:             "2025-05-23",
			transaction_date: "2025-05-23",
			txType:           "expense",
			payTo:            "Admin User", // Admin owes this
			paid:             false,
			paid_date:        "",
			optional:         false,
			enteredBy:        "1", // Sarah entered it
			userId:           "1",
			categoryName:     "Food",
			note:             "Admin's portion of dinner",
		},
		// Scenario 4: Patrick owes Admin $100 for concert tickets
		{
			id:               "settlement_test_4",
			amount:           100.00,
			description:      "Concert tickets",
			date:             "2025-05-22",
			transaction_date: "2025-05-22",
			txType:           "expense",
			payTo:            "Patrick", // Patrick owes this
			paid:             false,
			paid_date:        "",
			optional:         false,
			enteredBy:        "admin-user-1", // Admin entered it
			userId:           "admin-user-1",
			categoryName:     "Entertainment",
			note:             "Patrick's concert ticket",
		},
		// Scenario 5: Admin owes Patrick $80 for gas
		{
			id:               "settlement_test_5",
			amount:           80.00,
			description:      "Gas for road trip",
			date:             "2025-05-21",
			transaction_date: "2025-05-21",
			txType:           "expense",
			payTo:            "Admin User", // Admin owes this
			paid:             false,
			paid_date:        "",
			optional:         false,
			enteredBy:        "2", // Patrick entered it
			userId:           "2",
			categoryName:     "Transportation",
			note:             "Admin's share of gas",
		},
	}

	// Get category IDs for each user
	for _, tx := range settlementTransactions {
		// Get or create category
		var categoryId string
		err := db.QueryRow(`
			SELECT id FROM ynab_categories 
			WHERE name = $1 AND user_id = $2 
			LIMIT 1
		`, tx.categoryName, tx.userId).Scan(&categoryId)

		if err != nil {
			// Create category if it doesn't exist
			categoryId = fmt.Sprintf("cat-%s-%s", tx.userId, tx.categoryName)
			
			// Get a group for this user
			var groupId string
			err = db.QueryRow(`
				SELECT id FROM ynab_category_groups 
				WHERE user_id = $1 
				LIMIT 1
			`, tx.userId).Scan(&groupId)

			if err != nil {
				// Create a default group
				groupId = fmt.Sprintf("group-default-%s", tx.userId)
				_, err = db.Exec(`
					INSERT INTO ynab_category_groups (id, name, category_group_id, user_id, hidden)
					VALUES ($1, $2, $1, $3, $4)
					ON CONFLICT (id) DO NOTHING
				`, groupId, "Other", tx.userId, false)
				if err != nil {
					log.Printf("Error creating group: %v", err)
					continue
				}
			}

			_, err = db.Exec(`
				INSERT INTO ynab_categories (id, name, user_id, group_id, category_group_id, hidden, budget_amount) 
				VALUES ($1, $2, $3, $4, $4, $5, $6)
				ON CONFLICT (id) DO NOTHING
			`, categoryId, tx.categoryName, tx.userId, groupId, false, 0.0)
			if err != nil {
				log.Printf("Error creating category: %v", err)
				continue
			}
		}

		// Insert transaction
		_, err = db.Exec(`
			INSERT INTO transactions 
			(id, amount, description, date, transaction_date, type, pay_to, paid, paid_date, optional, entered_by, user_id, note) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			ON CONFLICT (id) DO NOTHING
		`, tx.id, tx.amount, tx.description, tx.date, tx.transaction_date,
			tx.txType, tx.payTo, tx.paid, tx.paid_date, tx.optional,
			tx.enteredBy, tx.userId, tx.note)

		if err != nil {
			log.Printf("Error inserting settlement test transaction %s: %v", tx.id, err)
			continue
		}

		// Associate with category
		_, err = db.Exec(`
			INSERT INTO transaction_categories 
			(transaction_id, category_id, amount) 
			VALUES ($1, $2, $3)
			ON CONFLICT (transaction_id, category_id) DO NOTHING
		`, tx.id, categoryId, tx.amount)

		if err != nil {
			log.Printf("Error associating transaction with category: %v", err)
		}

		log.Printf("Added settlement test transaction: %s (%s owes %s $%.2f)", 
			tx.description, tx.payTo, tx.enteredBy, tx.amount)
	}

	log.Println("✅ Settlement test data added successfully")
	log.Println("Test scenarios created:")
	log.Println("- Admin owes Sarah: $150 + $50 = $200")
	log.Println("- Sarah owes Admin: $75")
	log.Println("- Patrick owes Admin: $100")
	log.Println("- Admin owes Patrick: $80")
	log.Println("Net debts: Admin owes Sarah $125, Patrick owes Admin $20")

	return nil
}