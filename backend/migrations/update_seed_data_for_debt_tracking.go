package migrations

import (
	"database/sql"
	"log"
)

// UpdateSeedDataForDebtTracking updates the seed data to use the new debt tracking structure
func UpdateSeedDataForDebtTracking(db *sql.DB) error {
	// Check if we're in production - never run seed data in production
	if isProduction() {
		log.Println("Skipping seed data update in production environment")
		return nil
	}

	log.Println("Updating seed data for new debt tracking structure...")

	// Clear ALL test transactions for a fresh start
	_, err := db.Exec(`DELETE FROM transactions WHERE id LIKE 'test-tx-%' OR id LIKE 'settlement_test_%'`)
	if err != nil {
		log.Printf("Error clearing test transactions: %v", err)
		// Continue anyway
	}

	// First ensure our 3 users exist
	users := []struct {
		id       string
		username string
		name     string
		role     string
	}{
		{"UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", "Patrick Bennett", "Patrick Bennett", "superadmin"},
		{"sarah-wallis-id", "sarah", "Sarah Wallis", "user"},
		{"kim-donaldson-id", "kim", "Kim Donaldson", "user"},
	}

	for _, user := range users {
		_, err := db.Exec(`
			INSERT INTO users (id, username, name, role) 
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (id) DO UPDATE SET 
			    name = EXCLUDED.name,
			    role = EXCLUDED.role
		`, user.id, user.username, user.name, user.role)
		if err != nil {
			log.Printf("Error creating/updating user %s: %v", user.name, err)
		}
	}

	// Simplified test transactions with just 3 users
	testTransactions := []struct {
		id          string
		amount      float64
		description string
		paidBy      string // User ID who paid
		owedBy      string // User ID who owes money
		note        string
		paid        bool
	}{
		// === TRANSACTIONS WHERE PATRICK BENNETT OWES MONEY ===
		// Sarah paid, Patrick Bennett owes
		{"test-tx-1", 120.00, "Dinner", "sarah-wallis-id", "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", "Italian restaurant - Sarah paid, Patrick owes half", false},
		{"test-tx-2", 80.00, "Groceries", "sarah-wallis-id", "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", "Weekly groceries - Sarah paid, Patrick owes half", false},
		{"test-tx-3", 200.00, "Airbnb", "sarah-wallis-id", "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", "Weekend trip accommodation - Sarah paid, Patrick owes half", false},

		// Kim paid, Patrick Bennett owes
		{"test-tx-4", 60.00, "Uber rides", "kim-donaldson-id", "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", "Airport trips - Kim paid, Patrick owes", false},
		{"test-tx-5", 45.00, "Lunch", "kim-donaldson-id", "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", "Team lunch - Kim paid, Patrick owes his portion", false},
		{"test-tx-6", 150.00, "Concert tickets", "kim-donaldson-id", "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", "Taylor Swift concert - Kim paid, Patrick owes", false},

		// === TRANSACTIONS WHERE OTHERS OWE PATRICK BENNETT ===
		// Patrick Bennett paid, Sarah owes
		{"test-tx-7", 90.00, "Gas", "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", "sarah-wallis-id", "Road trip gas - Patrick paid, Sarah owes half", false},
		{"test-tx-8", 110.00, "Movie tickets", "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", "sarah-wallis-id", "IMAX movie for the group - Patrick paid, Sarah owes", false},
		{"test-tx-9", 75.00, "Breakfast", "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", "sarah-wallis-id", "Sunday brunch - Patrick paid, Sarah owes half", false},

		// Patrick Bennett paid, Kim owes
		{"test-tx-10", 140.00, "Hotel", "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", "kim-donaldson-id", "Conference hotel - Patrick paid, Kim owes half", false},
		{"test-tx-11", 85.00, "Dinner", "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", "kim-donaldson-id", "Sushi dinner - Patrick paid, Kim owes", false},
		{"test-tx-12", 50.00, "Office supplies", "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", "kim-donaldson-id", "Printer ink and paper - Patrick paid, Kim owes", false},

		// === SOME PAID TRANSACTIONS FOR HISTORY ===
		{"test-tx-13", 100.00, "Old dinner", "sarah-wallis-id", "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", "Mexican restaurant - Sarah paid, Patrick paid back", true},
		{"test-tx-14", 75.00, "Old groceries", "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", "kim-donaldson-id", "Costco run - Patrick paid, Kim paid back", true},
	}

	for _, tx := range testTransactions {
		query := `
			INSERT INTO transactions (
				id, amount, description, date, transaction_date, type, 
				paid_by, owed_by, paid, paid_date, entered_by, optional, note, user_id
			) VALUES (
				$1, $2, $3, CURRENT_DATE, CURRENT_DATE, $3,
				$4, $5, $6, $7, $4, false, $8, $4
			) ON CONFLICT (id) DO UPDATE SET
				paid_by = EXCLUDED.paid_by,
				owed_by = EXCLUDED.owed_by,
				note = EXCLUDED.note
		`

		var paidDate *string
		if tx.paid {
			pd := "CURRENT_DATE"
			paidDate = &pd
		}

		// Handle empty owedBy
		var owedBy *string
		if tx.owedBy != "" {
			owedBy = &tx.owedBy
		}

		_, err := db.Exec(query, tx.id, tx.amount, tx.description, tx.paidBy, owedBy, tx.paid, paidDate, tx.note)
		if err != nil {
			log.Printf("Error inserting transaction %s: %v", tx.id, err)
			// Continue with other transactions
		}
	}

	log.Println("Updated seed data for debt tracking")
	log.Println("")
	log.Println("=== DEBT SUMMARY ===")
	log.Println("Patrick Bennett OWES:")
	log.Println("  - Sarah Wallis: $400 (Dinner $120 + Groceries $80 + Airbnb $200)")
	log.Println("  - Kim Donaldson: $255 (Uber $60 + Lunch $45 + Concert $150)")
	log.Println("")
	log.Println("Patrick Bennett is OWED:")
	log.Println("  - From Sarah Wallis: $275 (Gas $90 + Movies $110 + Breakfast $75)")
	log.Println("  - From Kim Donaldson: $275 (Hotel $140 + Dinner $85 + Supplies $50)")
	log.Println("")
	log.Println("Net balances:")
	log.Println("  - Patrick owes Sarah: $125")
	log.Println("  - Kim owes Patrick: $20")

	return nil
}
