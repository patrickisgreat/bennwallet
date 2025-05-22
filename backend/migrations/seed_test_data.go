package migrations

import (
	"database/sql"
	"fmt"
	"log"
	"os"
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
		{id: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", username: "Patrick Bennett", name: "Patrick Bennett", role: "superadmin"},
		{id: "regular1", username: "regularuser1", name: "Regular User 1", role: "user"},
		{id: "regular2", username: "regularuser2", name: "Regular User 2", role: "user"},
		{id: "regular3", username: "regularuser3", name: "Regular User 3", role: "user"},
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

	// 1b. Add a test admin user for development - MOVED BEFORE CATEGORIES
	adminUser := struct {
		id       string
		username string
		name     string
		role     string
	}{
		id:       "admin-user-1",
		username: "admin-user",
		name:     "Admin User",
		role:     "superadmin",
	}

	// Insert admin user with ON CONFLICT DO NOTHING
	_, err := db.Exec(`
		INSERT INTO users (id, username, name, role) 
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE SET 
		    role = 'superadmin'`,
		adminUser.id, adminUser.username, adminUser.name, adminUser.role)
	if err != nil {
		return fmt.Errorf("failed to insert admin user %s: %w", adminUser.username, err)
	}
	log.Printf("Created or updated admin test user with ID: %s", adminUser.id)

	// 2. Add category groups first
	categoryGroups := []struct {
		id      string
		name    string
		user_id string
	}{
		{id: "group-1", name: "Essentials", user_id: "1"},
		{id: "group-2", name: "Lifestyle", user_id: "1"},
		{id: "group-3", name: "Essentials", user_id: "2"},
		{id: "group-4", name: "Lifestyle", user_id: "2"},
		{id: "group-5", name: "Essentials", user_id: "regular1"},
		{id: "group-6", name: "Essentials", user_id: "regular2"},
		{id: "group-7", name: "Essentials", user_id: "regular3"},
		{id: "group-8", name: "Essentials", user_id: "admin-user-1"},
		{id: "group-9", name: "Essentials", user_id: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2"},
		{id: "group-10", name: "Lifestyle", user_id: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2"},
	}

	for _, group := range categoryGroups {
		_, err := db.Exec(`
			INSERT INTO ynab_category_groups (id, name, category_group_id, user_id, hidden)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (id) DO NOTHING
		`, group.id, group.name, group.id, group.user_id, false)

		if err != nil {
			return fmt.Errorf("failed to insert category group %s: %w", group.name, err)
		}
	}

	// 3. Add sample categories with proper group associations
	sampleCategories := []struct {
		name           string
		description    string
		user_id        string
		color          string
		category_group string
	}{
		{name: "Food", description: "Groceries and dining out", user_id: "1", color: "#4CAF50", category_group: "group-1"},
		{name: "Housing", description: "Rent, mortgage, repairs", user_id: "1", color: "#2196F3", category_group: "group-1"},
		{name: "Transportation", description: "Car, public transit, gas", user_id: "1", color: "#FFC107", category_group: "group-1"},
		{name: "Entertainment", description: "Movies, games, hobbies", user_id: "1", color: "#9C27B0", category_group: "group-2"},
		{name: "Utilities", description: "Bills and services", user_id: "1", color: "#F44336", category_group: "group-1"},
		{name: "Healthcare", description: "Medical expenses", user_id: "1", color: "#3F51B5", category_group: "group-1"},
		{name: "Personal Care", description: "Haircuts, gym", user_id: "1", color: "#009688", category_group: "group-2"},
		{name: "Education", description: "Tuition, books", user_id: "1", color: "#FF5722", category_group: "group-2"},
		{name: "Pets", description: "Pet food, vet", user_id: "1", color: "#795548", category_group: "group-2"},
		{name: "Gifts", description: "Presents, donations", user_id: "1", color: "#E91E63", category_group: "group-2"},

		{name: "Food", description: "Groceries and dining out", user_id: "2", color: "#4CAF50", category_group: "group-3"},
		{name: "Housing", description: "Rent, mortgage, repairs", user_id: "2", color: "#2196F3", category_group: "group-3"},
		{name: "Transportation", description: "Car, public transit, gas", user_id: "2", color: "#FFC107", category_group: "group-3"},
		{name: "Entertainment", description: "Movies, games, hobbies", user_id: "2", color: "#9C27B0", category_group: "group-4"},
		{name: "Utilities", description: "Bills and services", user_id: "2", color: "#F44336", category_group: "group-3"},
		{name: "Healthcare", description: "Medical expenses", user_id: "2", color: "#3F51B5", category_group: "group-3"},
		{name: "Shopping", description: "Clothes, electronics", user_id: "2", color: "#607D8B", category_group: "group-4"},
		{name: "Travel", description: "Vacations, trips", user_id: "2", color: "#8BC34A", category_group: "group-4"},

		// Regular user 1 categories
		{name: "Food", description: "Groceries and dining out", user_id: "regular1", color: "#4CAF50", category_group: "group-5"},
		{name: "Rent", description: "Monthly rent", user_id: "regular1", color: "#2196F3", category_group: "group-5"},
		{name: "Transportation", description: "Car, bus, train", user_id: "regular1", color: "#FFC107", category_group: "group-5"},
		{name: "Entertainment", description: "Movies, games, hobbies", user_id: "regular1", color: "#9C27B0", category_group: "group-5"},

		// Regular user 2 categories
		{name: "Food", description: "Groceries and dining out", user_id: "regular2", color: "#4CAF50", category_group: "group-6"},
		{name: "Utilities", description: "Bills and services", user_id: "regular2", color: "#F44336", category_group: "group-6"},
		{name: "Transportation", description: "Car, public transit, gas", user_id: "regular2", color: "#FFC107", category_group: "group-6"},
		{name: "Shopping", description: "Clothes, electronics", user_id: "regular2", color: "#607D8B", category_group: "group-6"},

		// Regular user 3 categories
		{name: "Food", description: "Groceries and dining out", user_id: "regular3", color: "#4CAF50", category_group: "group-7"},
		{name: "Healthcare", description: "Medical expenses", user_id: "regular3", color: "#3F51B5", category_group: "group-7"},
		{name: "Entertainment", description: "Movies, games, hobbies", user_id: "regular3", color: "#9C27B0", category_group: "group-7"},
		{name: "Travel", description: "Vacations, trips", user_id: "regular3", color: "#8BC34A", category_group: "group-7"},

		// Admin user categories
		{name: "Rent", description: "Monthly rent", user_id: "admin-user-1", color: "#2196F3", category_group: "group-8"},
		{name: "Groceries", description: "Food and household items", user_id: "admin-user-1", color: "#4CAF50", category_group: "group-8"},
		{name: "Utilities", description: "Electricity, water, internet", user_id: "admin-user-1", color: "#F44336", category_group: "group-8"},
		{name: "Transportation", description: "Bus, train, Uber", user_id: "admin-user-1", color: "#FFC107", category_group: "group-8"},
		{name: "Entertainment", description: "Movies, games", user_id: "admin-user-1", color: "#9C27B0", category_group: "group-8"},
		{name: "Health", description: "Medical, dental", user_id: "admin-user-1", color: "#3F51B5", category_group: "group-8"},
		{name: "Test", description: "Test Category", user_id: "admin-user-1", color: "#FF0000", category_group: "group-8"},

		// Patrick Bennett categories
		{name: "Housing", description: "Rent, mortgage, repairs", user_id: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", color: "#2196F3", category_group: "group-9"},
		{name: "Food", description: "Groceries and dining out", user_id: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", color: "#4CAF50", category_group: "group-9"},
		{name: "Transportation", description: "Car, public transit, gas", user_id: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", color: "#FFC107", category_group: "group-9"},
		{name: "Entertainment", description: "Movies, games, hobbies", user_id: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", color: "#9C27B0", category_group: "group-10"},
		{name: "Utilities", description: "Bills and services", user_id: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", color: "#F44336", category_group: "group-9"},
		{name: "Healthcare", description: "Medical expenses", user_id: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", color: "#3F51B5", category_group: "group-9"},
		{name: "Shopping", description: "Clothes, electronics", user_id: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", color: "#607D8B", category_group: "group-10"},
		{name: "Travel", description: "Vacations, trips", user_id: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", color: "#8BC34A", category_group: "group-10"},
	}

	categoryIds := make(map[string]string) // Map to store category IDs by name and user

	for _, cat := range sampleCategories {
		// Create a unique ID for the category
		categoryID := fmt.Sprintf("cat-%s-%s", cat.user_id, cat.name)

		// Insert category into ynab_categories if it doesn't exist
		_, err := db.Exec(`
			INSERT INTO ynab_categories (id, name, user_id, category_group_id, hidden, budget_amount) 
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (id) DO NOTHING
		`, categoryID, cat.name, cat.user_id, cat.category_group, false, 0.0)

		if err != nil {
			return fmt.Errorf("failed to insert ynab_category %s: %w", cat.name, err)
		}

		// Store category ID in the map using composite key of name + user_id
		categoryIds[cat.name+"-"+cat.user_id] = categoryID
	}

	// First get user IDs to use for pay_to and entered_by fields
	userIds := make([]string, 0, len(defaultUsers))
	for _, user := range defaultUsers {
		userIds = append(userIds, user.id)
	}

	// 3. Seed sample transaction data
	sampleTransactions := []struct {
		id               string
		amount           float64
		description      string
		date             string
		transaction_date string
		txType           string
		payTo            string // This should be a valid user ID
		paid             bool
		paid_date        string
		optional         bool
		enteredBy        string // This should be a valid user ID
		userId           string
		categoryName     string
		note             string
	}{
		{
			id:               "tx_1",
			amount:           43.50,
			description:      "Groceries",
			date:             "2025-04-15",
			transaction_date: "2025-04-15",
			txType:           "expense",
			payTo:            "1", // Sarah
			paid:             true,
			paid_date:        "2025-04-16",
			optional:         false,
			enteredBy:        "2", // Patrick
			userId:           "2",
			categoryName:     "Food",
			note:             "Weekly grocery shopping for essentials",
		},
		{
			id:               "tx_2",
			amount:           1200.00,
			description:      "Rent",
			date:             "2025-04-01",
			transaction_date: "2025-04-01",
			txType:           "expense",
			payTo:            "2", // Patrick
			paid:             true,
			paid_date:        "2025-04-02",
			optional:         false,
			enteredBy:        "1", // Sarah
			userId:           "1",
			categoryName:     "Housing",
			note:             "Monthly apartment rental payment",
		},
		{
			id:               "tx_3",
			amount:           85.99,
			description:      "Internet bill",
			date:             "2025-04-10",
			transaction_date: "2025-04-10",
			txType:           "expense",
			payTo:            "regular1", // Regular User 1
			paid:             false,
			paid_date:        "",
			optional:         false,
			enteredBy:        "2", // Patrick
			userId:           "2",
			categoryName:     "Utilities",
			note:             "Fiber internet monthly subscription",
		},
		{
			id:               "tx_4",
			amount:           2500.00,
			description:      "Salary",
			date:             "2025-04-25",
			transaction_date: "2025-04-25",
			txType:           "income",
			payTo:            "regular2", // Regular User 2
			paid:             true,
			paid_date:        "2025-04-25",
			optional:         true,
			enteredBy:        "1", // Sarah
			userId:           "1",
			categoryName:     "Entertainment",
			note:             "Monthly paycheck from Acme Corp",
		},
		{
			id:               "tx_5",
			amount:           175.50,
			description:      "Shared Dinner",
			date:             "2025-05-01",
			transaction_date: "2025-05-01",
			txType:           "expense",
			payTo:            "Regular User 3", // Display name instead of ID for consistency with UI
			paid:             false,
			paid_date:        "",
			optional:         false,
			enteredBy:        "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", // Patrick Bennett
			userId:           "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2",
			categoryName:     "Food",
			note:             "Birthday dinner at Fancy Restaurant with friends",
		},
		{
			id:               "tx_6",
			amount:           60.00,
			description:      "Regular user transaction",
			date:             "2025-05-05",
			transaction_date: "2025-05-05",
			txType:           "expense",
			payTo:            "1", // Sarah
			paid:             true,
			paid_date:        "2025-05-06",
			optional:         false,
			enteredBy:        "regular1", // Regular User 1
			userId:           "regular1",
			categoryName:     "Entertainment",
			note:             "Movie tickets and popcorn with friends",
		},
		{
			id:               "tx_7",
			amount:           120.75,
			description:      "Another regular user transaction",
			date:             "2025-05-10",
			transaction_date: "2025-05-10",
			txType:           "expense",
			payTo:            "2", // Patrick
			paid:             false,
			paid_date:        "",
			optional:         false,
			enteredBy:        "regular2", // Regular User 2
			userId:           "regular2",
			categoryName:     "Transportation",
			note:             "Uber rides and bus fares for the week",
		},
	}

	// Check if transaction_categories table exists
	var hasTransactionCategoriesTable bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_name = 'transaction_categories'
		)
	`).Scan(&hasTransactionCategoriesTable)

	if err != nil {
		return fmt.Errorf("failed to check for transaction_categories table: %w", err)
	}

	// Create transaction_categories table if it doesn't exist
	if !hasTransactionCategoriesTable {
		_, err = db.Exec(`
			CREATE TABLE IF NOT EXISTS transaction_categories (
				id SERIAL PRIMARY KEY,
				transaction_id TEXT NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
				category_id TEXT NOT NULL,
				amount NUMERIC(15,2) NOT NULL,
				created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(transaction_id, category_id)
			)
		`)
		if err != nil {
			return fmt.Errorf("failed to create transaction_categories table: %w", err)
		}
		log.Println("Created transaction_categories table")
	}

	// Add an explicit index to improve joining performance
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_transaction_categories_category_id ON transaction_categories(category_id)
	`)
	if err != nil {
		log.Printf("Warning: Failed to create index on transaction_categories: %v", err)
	}

	// Clear out existing transaction_categories
	_, err = db.Exec(`TRUNCATE TABLE transaction_categories`)
	if err != nil {
		log.Printf("Warning: Failed to truncate transaction_categories: %v", err)
	}

	// 2. Check if the transactions table is empty
	var transactionCount int
	err = db.QueryRow("SELECT COUNT(*) FROM transactions").Scan(&transactionCount)
	if err != nil {
		log.Printf("Error checking transaction count: %v", err)
		transactionCount = 0 // Assume empty if error
	}

	if transactionCount == 0 {
		log.Println("Seeding transactions...")

		// Insert the transactions
		for _, tx := range sampleTransactions {
			_, err := db.Exec(`
				INSERT INTO transactions 
				(id, amount, description, date, transaction_date, type, pay_to, paid, paid_date, optional, entered_by, user_id, note) 
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			`,
				tx.id, tx.amount, tx.description, tx.date, tx.transaction_date,
				tx.txType, tx.payTo, tx.paid, tx.paid_date, tx.optional,
				tx.enteredBy, tx.userId, tx.note)

			if err != nil {
				return fmt.Errorf("error inserting transaction %s: %w", tx.id, err)
			}

			log.Printf("Inserted transaction %s for user %s with category %s", tx.id, tx.userId, tx.categoryName)

			// Get the category ID
			categoryId, exists := categoryIds[tx.categoryName+"-"+tx.userId]
			if !exists {
				log.Printf("WARNING: Category %s not found for user %s, skipping association", tx.categoryName, tx.userId)
				// Try creating the category on the fly
				categoryId = fmt.Sprintf("cat-%s-%s", tx.userId, tx.categoryName)
				_, err = db.Exec(`
					INSERT INTO ynab_categories (id, name, user_id, hidden) 
					VALUES ($1, $2, $3, $4)
					ON CONFLICT (id) DO NOTHING
				`, categoryId, tx.categoryName, tx.userId, false)

				if err != nil {
					log.Printf("Error creating fallback category: %v", err)
					continue
				}
				log.Printf("Created fallback category %s for transaction %s", categoryId, tx.id)
			}

			// Associate the transaction with the category
			log.Printf("Associating transaction %s with category %s (ID: %s)", tx.id, tx.categoryName, categoryId)
			_, err = db.Exec(`
				INSERT INTO transaction_categories 
				(transaction_id, category_id, amount) 
				VALUES ($1, $2, $3)
				ON CONFLICT (transaction_id, category_id) DO NOTHING
			`, tx.id, categoryId, tx.amount)

			if err != nil {
				return fmt.Errorf("failed to associate transaction %s with category %s: %w", tx.id, tx.categoryName, err)
			}

			// Verify the association was created
			var count int
			err = db.QueryRow(`
				SELECT COUNT(*) FROM transaction_categories 
				WHERE transaction_id = $1 AND category_id = $2
			`, tx.id, categoryId).Scan(&count)

			if err != nil {
				log.Printf("Error verifying transaction-category association: %v", err)
			} else if count == 0 {
				log.Printf("WARNING: Failed to create transaction-category association for %s and %s", tx.id, categoryId)
			} else {
				log.Printf("VERIFIED: Associated transaction %s with category %s (ID: %s)", tx.id, tx.categoryName, categoryId)
			}
		}
	}

	// 4. Seed user permissions
	permissionsData := []struct {
		grantedUserId  string
		ownerUserId    string
		resourceType   string
		permissionType string
	}{
		// Admin permissions
		{grantedUserId: "2", ownerUserId: "1", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "1", ownerUserId: "2", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "admin", ownerUserId: "1", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "admin", ownerUserId: "1", resourceType: "transactions", permissionType: "write"},
		{grantedUserId: "admin", ownerUserId: "2", resourceType: "transactions", permissionType: "read"},

		// Regular user permissions
		{grantedUserId: "1", ownerUserId: "regular1", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "2", ownerUserId: "regular1", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "regular1", ownerUserId: "1", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "regular1", ownerUserId: "2", resourceType: "transactions", permissionType: "read"},

		{grantedUserId: "1", ownerUserId: "regular2", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "2", ownerUserId: "regular2", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "regular2", ownerUserId: "1", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "regular2", ownerUserId: "2", resourceType: "transactions", permissionType: "read"},

		{grantedUserId: "1", ownerUserId: "regular3", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "2", ownerUserId: "regular3", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "regular3", ownerUserId: "1", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "regular3", ownerUserId: "2", resourceType: "transactions", permissionType: "read"},

		// Cross-permissions between regular users
		{grantedUserId: "regular1", ownerUserId: "regular2", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "regular2", ownerUserId: "regular1", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "regular1", ownerUserId: "regular3", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "regular3", ownerUserId: "regular1", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "regular2", ownerUserId: "regular3", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "regular3", ownerUserId: "regular2", resourceType: "transactions", permissionType: "read"},

		// Add permissions for Patrick Bennett to access everyone's transactions
		{grantedUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", ownerUserId: "1", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", ownerUserId: "1", resourceType: "transactions", permissionType: "write"},
		{grantedUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", ownerUserId: "2", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", ownerUserId: "2", resourceType: "transactions", permissionType: "write"},
		{grantedUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", ownerUserId: "admin", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", ownerUserId: "admin", resourceType: "transactions", permissionType: "write"},
		{grantedUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", ownerUserId: "regular1", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", ownerUserId: "regular1", resourceType: "transactions", permissionType: "write"},
		{grantedUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", ownerUserId: "regular2", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", ownerUserId: "regular2", resourceType: "transactions", permissionType: "write"},
		{grantedUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", ownerUserId: "regular3", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", ownerUserId: "regular3", resourceType: "transactions", permissionType: "write"},

		// Add reciprocal permissions for other users to see Patrick's transactions
		{grantedUserId: "1", ownerUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "2", ownerUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "admin", ownerUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "regular1", ownerUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "regular2", ownerUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", resourceType: "transactions", permissionType: "read"},
		{grantedUserId: "regular3", ownerUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", resourceType: "transactions", permissionType: "read"},

		// Add permissions for categories
		{grantedUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", ownerUserId: "1", resourceType: "categories", permissionType: "read"},
		{grantedUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", ownerUserId: "1", resourceType: "categories", permissionType: "write"},
		{grantedUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", ownerUserId: "2", resourceType: "categories", permissionType: "read"},
		{grantedUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", ownerUserId: "2", resourceType: "categories", permissionType: "write"},
		{grantedUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", ownerUserId: "admin", resourceType: "categories", permissionType: "read"},
		{grantedUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", ownerUserId: "admin", resourceType: "categories", permissionType: "write"},
		{grantedUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", ownerUserId: "regular1", resourceType: "categories", permissionType: "read"},
		{grantedUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", ownerUserId: "regular1", resourceType: "categories", permissionType: "write"},
		{grantedUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", ownerUserId: "regular2", resourceType: "categories", permissionType: "read"},
		{grantedUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", ownerUserId: "regular2", resourceType: "categories", permissionType: "write"},
		{grantedUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", ownerUserId: "regular3", resourceType: "categories", permissionType: "read"},
		{grantedUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", ownerUserId: "regular3", resourceType: "categories", permissionType: "write"},

		// Add reciprocal permissions for categories
		{grantedUserId: "1", ownerUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", resourceType: "categories", permissionType: "read"},
		{grantedUserId: "2", ownerUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", resourceType: "categories", permissionType: "read"},
		{grantedUserId: "admin", ownerUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", resourceType: "categories", permissionType: "read"},
		{grantedUserId: "regular1", ownerUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", resourceType: "categories", permissionType: "read"},
		{grantedUserId: "regular2", ownerUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", resourceType: "categories", permissionType: "read"},
		{grantedUserId: "regular3", ownerUserId: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", resourceType: "categories", permissionType: "read"},
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

	// 5. Add a test transaction for the admin user
	adminTransaction := struct {
		id               string
		amount           float64
		description      string
		date             string
		transaction_date string
		txType           string
		payTo            string
		paid             bool
		paid_date        string
		optional         bool
		enteredBy        string
		userId           string
		categoryName     string
		note             string
	}{
		id:               "admin_tx_1",
		amount:           99.99,
		description:      "Admin Test Transaction",
		date:             "2025-04-15",
		transaction_date: "2025-04-15",
		txType:           "expense",
		payTo:            "regular1", // Regular User 1 instead of Sarah
		paid:             true,
		paid_date:        "2025-04-16",
		optional:         false,
		enteredBy:        adminUser.id, // Admin user ID
		userId:           adminUser.id,
		categoryName:     "Test",
		note:             "This is a test transaction for admin user testing",
	}

	// Create a test category for admin if it doesn't already exist in ynab_categories
	adminCategoryID := fmt.Sprintf("cat-%s-Test", adminUser.id)
	_, err = db.Exec(`
		INSERT INTO ynab_categories (id, name, user_id, category_group_id, hidden, budget_amount) 
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO NOTHING
	`, adminCategoryID, "Test", adminUser.id, "group-8", false, 0.0)
	if err != nil {
		return fmt.Errorf("failed to insert test category for admin: %w", err)
	}

	// Insert admin test transaction
	_, err = db.Exec(`
		INSERT INTO transactions 
		(id, amount, description, date, transaction_date, type, pay_to, paid, paid_date, optional, entered_by, user_id, note) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (id) DO NOTHING
	`, adminTransaction.id, adminTransaction.amount, adminTransaction.description,
		adminTransaction.date, adminTransaction.transaction_date, adminTransaction.txType,
		adminTransaction.payTo, adminTransaction.paid, adminTransaction.paid_date,
		adminTransaction.optional, adminTransaction.enteredBy, adminTransaction.userId, adminTransaction.note)

	if err != nil {
		return fmt.Errorf("failed to insert admin test transaction: %w", err)
	}

	// Associate the transaction with the category
	log.Printf("Associating admin transaction %s with category %s (ID: %s)",
		adminTransaction.id, adminTransaction.categoryName, adminCategoryID)

	_, err = db.Exec(`
		INSERT INTO transaction_categories 
		(transaction_id, category_id, amount) 
		VALUES ($1, $2, $3)
		ON CONFLICT (transaction_id, category_id) DO NOTHING
	`, adminTransaction.id, adminCategoryID, adminTransaction.amount)

	if err != nil {
		return fmt.Errorf("failed to associate admin transaction with category: %w", err)
	}

	// Verify the association was created
	var count int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM transaction_categories 
		WHERE transaction_id = $1 AND category_id = $2
	`, adminTransaction.id, adminCategoryID).Scan(&count)

	if err != nil {
		log.Printf("Error verifying admin transaction-category association: %v", err)
	} else if count == 0 {
		log.Printf("WARNING: Failed to create admin transaction-category association for %s and %s",
			adminTransaction.id, adminCategoryID)
	} else {
		log.Printf("VERIFIED: Associated admin transaction %s with category %s (ID: %s)",
			adminTransaction.id, adminTransaction.categoryName, adminCategoryID)
	}

	// Make sure Patrick Bennett is always a superadmin
	_, err = db.Exec(`
		UPDATE users SET role = 'superadmin' 
		WHERE id = 'UgwzWuP8iHNF8nhqDHMwFFcg8Sc2'
	`)
	if err != nil {
		log.Printf("Warning: Failed to ensure Patrick Bennett is superadmin: %v", err)
	} else {
		log.Printf("Ensured Patrick Bennett (UgwzWuP8iHNF8nhqDHMwFFcg8Sc2) is set as superadmin")
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
