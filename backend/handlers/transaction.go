package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"bennwallet/backend/database"
	"bennwallet/backend/middleware"
	"bennwallet/backend/models"

	"github.com/gorilla/mux"
)

// EnsureTransactionDateColumn checks if the transaction_date column exists in the transactions table
// and creates it if it doesn't exist. This helps handle database schema migrations gracefully.
func EnsureTransactionDateColumn() error {
	log.Println("Checking for transaction_date column in transactions table...")

	// Check if the transactions table exists
	var hasTransactionsTable bool
	err := database.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_name = 'transactions'
		)
	`).Scan(&hasTransactionsTable)

	if err != nil {
		log.Printf("Error checking for transactions table: %v", err)
		return err
	}

	if !hasTransactionsTable {
		log.Println("Transactions table doesn't exist yet, no need to add column")
		return nil
	}

	// Check if the transaction_date column exists
	var hasTransactionDateColumn bool
	err = database.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'transactions' AND column_name = 'transaction_date'
		)
	`).Scan(&hasTransactionDateColumn)

	if err != nil {
		log.Printf("Error checking for transaction_date column: %v", err)
		return err
	}

	if !hasTransactionDateColumn {
		log.Println("Adding transaction_date column to transactions table")
		_, err = database.DB.Exec(`ALTER TABLE transactions ADD COLUMN transaction_date TEXT`)
		if err != nil {
			log.Printf("Error adding transaction_date column: %v", err)
			return err
		}

		log.Println("Setting transaction_date to match date column for existing records")
		_, err = database.DB.Exec(`UPDATE transactions SET transaction_date = date WHERE transaction_date IS NULL`)
		if err != nil {
			log.Printf("Error populating transaction_date column: %v", err)
			return err
		}

		log.Println("Successfully added and populated transaction_date column")
	} else {
		log.Println("transaction_date column already exists")
	}

	return nil
}

func GetTransactions(w http.ResponseWriter, r *http.Request) {
	// Get the user ID from the authentication context
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		log.Printf("Error: No user ID found in context")
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	log.Printf("GetTransactions called for user ID: %s", userID)

	// First check if the users table exists
	var hasUsersTable bool
	err := database.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_name = 'users'
		)
	`).Scan(&hasUsersTable)

	if err != nil {
		log.Printf("Error checking for users table: %v", err)
		hasUsersTable = false
	}

	// Build a query specifically for PostgreSQL
	// Join with users table if it exists to display user names
	var query string
	if hasUsersTable {
		query = `
			SELECT 
				t.id, 
				t.amount, 
				t.description, 
				t.date, 
				t.transaction_date, 
				t.type, 
				COALESCE(payto_user.name, t.pay_to) as pay_to, 
				t.paid, 
				t.paid_date, 
				COALESCE(entered_user.name, t.entered_by) as entered_by, 
				t.optional, 
				t.note,
				t.user_id
			FROM transactions t
			LEFT JOIN users payto_user ON t.pay_to = payto_user.id
			LEFT JOIN users entered_user ON t.entered_by = entered_user.id
			WHERE 1=1
		`
	} else {
		query = `
			SELECT 
				id, 
				amount, 
				description, 
				date, 
				transaction_date, 
				type, 
				pay_to, 
				paid, 
				paid_date, 
				entered_by, 
				optional, 
				note,
				user_id
			FROM transactions 
			WHERE 1=1
		`
	}

	args := []interface{}{}
	paramCounter := 1

	// Add user ID filter
	// Get list of user IDs the current user can access using the permissions system
	accessibleUsers, err := middleware.GetUserAccessibleResources(userID, models.ResourceTransactions, models.PermissionRead)
	if err != nil {
		log.Printf("Error getting accessible resources: %v", err)
		// Fallback to showing only the user's own transactions
		if hasUsersTable {
			query += fmt.Sprintf(" AND t.user_id = $%d", paramCounter)
		} else {
			query += fmt.Sprintf(" AND user_id = $%d", paramCounter)
		}
		args = append(args, userID)
		paramCounter++
		log.Printf("Fetching only personal transactions for user %s", userID)
	} else if len(accessibleUsers) > 0 {
		// Create placeholders for the SQL IN clause
		placeholders := make([]string, len(accessibleUsers))
		for i := range accessibleUsers {
			placeholders[i] = fmt.Sprintf("$%d", paramCounter)
			args = append(args, accessibleUsers[i])
			paramCounter++
		}

		// Build query with IN clause and also include NULL userIds for backward compatibility
		if hasUsersTable {
			query += fmt.Sprintf(" AND (t.user_id IN (%s) OR t.user_id IS NULL)",
				strings.Join(placeholders, ","))
		} else {
			query += fmt.Sprintf(" AND (user_id IN (%s) OR user_id IS NULL)",
				strings.Join(placeholders, ","))
		}
		log.Printf("Fetching transactions for user %s and %d other accessible users", userID, len(accessibleUsers)-1)
		log.Printf("Accessible users: %v", accessibleUsers)
	} else {
		// Fallback to showing only the user's own transactions
		if hasUsersTable {
			query += fmt.Sprintf(" AND t.user_id = $%d", paramCounter)
		} else {
			query += fmt.Sprintf(" AND user_id = $%d", paramCounter)
		}
		args = append(args, userID)
		paramCounter++
		log.Printf("Fetching only personal transactions for user %s (no permissions found)", userID)
	}

	// Parse query parameters
	payTo := r.URL.Query().Get("payTo")
	if payTo != "" {
		search := "%" + payTo + "%"
		if hasUsersTable {
			query += fmt.Sprintf(" AND (payto_user.name LIKE $%d OR t.pay_to LIKE $%d)", paramCounter, paramCounter+1)
			args = append(args, search, search)
			paramCounter += 2
		} else {
			query += fmt.Sprintf(" AND pay_to LIKE $%d", paramCounter)
			args = append(args, search)
			paramCounter++
		}
		log.Printf("Added PayTo LIKE filter: '%s' (as %s)", payTo, search)
	}

	enteredBy := r.URL.Query().Get("enteredBy")
	if enteredBy != "" {
		search := "%" + enteredBy + "%"
		if hasUsersTable {
			query += fmt.Sprintf(" AND (entered_user.name LIKE $%d OR t.entered_by LIKE $%d)", paramCounter, paramCounter+1)
			args = append(args, search, search)
			paramCounter += 2
		} else {
			query += fmt.Sprintf(" AND entered_by LIKE $%d", paramCounter)
			args = append(args, search)
			paramCounter++
		}
		log.Printf("Added EnteredBy LIKE filter: '%s' (as %s)", enteredBy, search)
	}

	paid := r.URL.Query().Get("paid")
	if paid != "" {
		if hasUsersTable {
			query += fmt.Sprintf(" AND t.paid = $%d", paramCounter)
		} else {
			query += fmt.Sprintf(" AND paid = $%d", paramCounter)
		}
		args = append(args, paid == "true")
		paramCounter++
	}

	// Add ORDER BY date DESC
	if hasUsersTable {
		query += " ORDER BY t.date DESC"
	} else {
		query += " ORDER BY date DESC"
	}

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var transactions []models.Transaction
	for rows.Next() {
		var t models.Transaction
		var paidDate sql.NullString
		var transactionDate sql.NullString
		var dateStr string
		var userId sql.NullString
		var note sql.NullString

		err = rows.Scan(&t.ID, &t.Amount, &t.Description, &dateStr, &transactionDate, &t.Type, &t.PayTo, &t.Paid, &paidDate, &t.EnteredBy, &t.Optional, &note, &userId)
		if err != nil {
			log.Printf("Error scanning transaction row: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Convert date strings to time.Time
		t.Date, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			log.Printf("Error parsing date %s: %v", dateStr, err)
			// Use current time as fallback
			t.Date = time.Now()
		}

		if paidDate.Valid {
			t.PaidDate = paidDate.String
		}

		if transactionDate.Valid {
			txDate, err := time.Parse("2006-01-02", transactionDate.String)
			if err != nil {
				log.Printf("Error parsing transaction_date %s: %v", transactionDate.String, err)
				t.TransactionDate = t.Date // Fallback to entered date
			} else {
				t.TransactionDate = txDate
			}
		} else {
			t.TransactionDate = t.Date // Fall back to entered date if transaction date not available
		}

		if userId.Valid {
			t.UserID = userId.String
		}

		if note.Valid {
			t.Note = note.String
		}

		// Check if transaction_categories table exists
		var hasTransactionCategoriesTable bool
		err := database.DB.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM information_schema.tables 
				WHERE table_name = 'transaction_categories'
			)
		`).Scan(&hasTransactionCategoriesTable)

		if err == nil && hasTransactionCategoriesTable {
			// Fetch associated categories
			catRows, err := database.DB.Query(`
				SELECT c.id, c.name, c.name as description, '#3498DB' as color, c.user_id
				FROM ynab_categories c
				JOIN transaction_categories tc ON c.id = tc.category_id
				WHERE tc.transaction_id = $1
			`, t.ID)

			if err == nil {
				defer catRows.Close()
				var categories []models.Category
				for catRows.Next() {
					var cat models.Category
					if err := catRows.Scan(&cat.ID, &cat.Name, &cat.Description, &cat.Color, &cat.UserID); err == nil {
						categories = append(categories, cat)
					}
				}
				catRows.Close() // Close here to avoid resource leak
				if len(categories) > 0 {
					t.Categories = categories
				}
			}
		}

		transactions = append(transactions, t)
	}

	w.Header().Set("Content-Type", "application/json")

	// Log the number of transactions and their owners
	log.Printf("Returning %d transactions for user %s", len(transactions), userID)
	if len(transactions) == 0 {
		// If no transactions, check if any exist in the database for debugging
		var count int
		err := database.DB.QueryRow("SELECT COUNT(*) FROM transactions").Scan(&count)
		if err != nil {
			log.Printf("Error checking transaction count: %v", err)
		} else {
			log.Printf("Total transactions in database: %d", count)

			// Check if there are any transactions for any user
			rows, err := database.DB.Query("SELECT DISTINCT user_id FROM transactions")
			if err != nil {
				log.Printf("Error checking transaction user_ids: %v", err)
			} else {
				defer rows.Close()
				var userIDs []string
				for rows.Next() {
					var uid string
					if err := rows.Scan(&uid); err == nil {
						userIDs = append(userIDs, uid)
					}
				}
				log.Printf("Transactions exist for user IDs: %v", userIDs)
			}
		}
	}

	json.NewEncoder(w).Encode(transactions)
}

func GetTransaction(w http.ResponseWriter, r *http.Request) {
	// Get the user ID from the authentication context
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	// Check if the users table exists
	var hasUsersTable bool
	err := database.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_name = 'users'
		)
	`).Scan(&hasUsersTable)

	if err != nil {
		log.Printf("Error checking for users table: %v", err)
		hasUsersTable = false
	}

	// First check if the optional column exists using PostgreSQL information_schema
	var hasOptionalColumn bool
	err = database.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'transactions' AND column_name = 'optional'
		)
	`).Scan(&hasOptionalColumn)

	if err != nil {
		log.Printf("Error checking for optional column: %v", err)
		hasOptionalColumn = false
	}

	// Check if the user_id column exists
	var hasUserIdColumn bool
	err = database.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'transactions' AND column_name = 'user_id'
		)
	`).Scan(&hasUserIdColumn)

	if err != nil {
		log.Printf("Error checking for user_id column: %v", err)
		hasUserIdColumn = false
	}

	var t models.Transaction
	var paidDate sql.NullString
	var transactionDate sql.NullString
	var userId sql.NullString
	var dateStr string

	var query string
	if hasUsersTable {
		// Join with users table to get names
		if hasOptionalColumn && hasUserIdColumn {
			query = `
				SELECT t.id, t.amount, t.description, t.date, t.transaction_date, t.type, 
				COALESCE(payto_user.name, t.pay_to) as pay_to, 
				t.paid, t.paid_date, 
				COALESCE(entered_user.name, t.entered_by) as entered_by, 
				t.optional, t.user_id, t.note 
				FROM transactions t
				LEFT JOIN users payto_user ON t.pay_to = payto_user.id
				LEFT JOIN users entered_user ON t.entered_by = entered_user.id
				WHERE t.id = $1
			`
		} else if hasOptionalColumn {
			query = `
				SELECT t.id, t.amount, t.description, t.date, t.transaction_date, t.type, 
				COALESCE(payto_user.name, t.pay_to) as pay_to, 
				t.paid, t.paid_date, 
				COALESCE(entered_user.name, t.entered_by) as entered_by, 
				t.optional, t.note 
				FROM transactions t
				LEFT JOIN users payto_user ON t.pay_to = payto_user.id
				LEFT JOIN users entered_user ON t.entered_by = entered_user.id
				WHERE t.id = $1
			`
		} else {
			query = `
				SELECT t.id, t.amount, t.description, t.date, t.transaction_date, t.type, 
				COALESCE(payto_user.name, t.pay_to) as pay_to, 
				t.paid, t.paid_date, 
				COALESCE(entered_user.name, t.entered_by) as entered_by, 
				t.optional, t.note 
				FROM transactions t
				LEFT JOIN users payto_user ON t.pay_to = payto_user.id
				LEFT JOIN users entered_user ON t.entered_by = entered_user.id
				WHERE t.id = $1
			`
		}
	} else {
		// Standard query without joins
		if hasOptionalColumn && hasUserIdColumn {
			query = `
				SELECT id, amount, description, date, transaction_date, type, pay_to, paid, paid_date, entered_by, optional, user_id, note 
				FROM transactions 
				WHERE id = $1
			`
		} else if hasOptionalColumn {
			query = `
				SELECT id, amount, description, date, transaction_date, type, pay_to, paid, paid_date, entered_by, optional, note 
				FROM transactions 
				WHERE id = $1
			`
		} else {
			query = `
				SELECT id, amount, description, date, transaction_date, type, pay_to, paid, paid_date, entered_by, note 
				FROM transactions 
				WHERE id = $1
			`
		}
	}

	var row *sql.Row
	row = database.DB.QueryRow(query, id)

	if hasOptionalColumn && hasUserIdColumn {
		err = row.Scan(&t.ID, &t.Amount, &t.Description, &dateStr, &transactionDate, &t.Type, &t.PayTo, &t.Paid, &paidDate, &t.EnteredBy, &t.Optional, &t.Note, &userId)
	} else if hasOptionalColumn {
		err = row.Scan(&t.ID, &t.Amount, &t.Description, &dateStr, &transactionDate, &t.Type, &t.PayTo, &t.Paid, &paidDate, &t.EnteredBy, &t.Optional, &t.Note)
	} else {
		err = row.Scan(&t.ID, &t.Amount, &t.Description, &dateStr, &transactionDate, &t.Type, &t.PayTo, &t.Paid, &paidDate, &t.EnteredBy, &t.Note)
	}

	if err != nil {
		log.Printf("Error fetching transaction %s: %v", id, err)
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Convert date string to time.Time
	t.Date, err = time.Parse("2006-01-02", dateStr)
	if err != nil {
		log.Printf("Error parsing date %s: %v", dateStr, err)
		// Use current time as fallback
		t.Date = time.Now()
	}

	if paidDate.Valid {
		t.PaidDate = paidDate.String
	}

	if transactionDate.Valid {
		txDate, err := time.Parse("2006-01-02", transactionDate.String)
		if err != nil {
			log.Printf("Error parsing transaction_date %s: %v", transactionDate.String, err)
			t.TransactionDate = t.Date // Fallback to entered date
		} else {
			t.TransactionDate = txDate
		}
	} else {
		t.TransactionDate = t.Date // Fall back to entered date
	}

	if hasUserIdColumn && userId.Valid {
		t.UserID = userId.String
	}

	// Check if the current user has permission to access this transaction
	if t.UserID != "" && t.UserID != userID {
		// Check if the user has permission to view this transaction through the permissions system
		hasPermission := middleware.CheckUserPermission(userID, t.UserID, models.ResourceTransactions, models.PermissionRead)
		if !hasPermission {
			http.Error(w, "You don't have permission to view this transaction", http.StatusForbidden)
			return
		}
	}

	// Check if transaction_categories table exists
	var hasTransactionCategoriesTable bool
	err = database.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_name = 'transaction_categories'
		)
	`).Scan(&hasTransactionCategoriesTable)

	if err == nil && hasTransactionCategoriesTable {
		// Fetch associated categories
		catRows, err := database.DB.Query(`
			SELECT c.id, c.name, c.name as description, '#3498DB' as color, c.user_id
			FROM ynab_categories c
			JOIN transaction_categories tc ON c.id = tc.category_id
			WHERE tc.transaction_id = $1
		`, t.ID)

		if err == nil {
			defer catRows.Close()
			var categories []models.Category
			for catRows.Next() {
				var cat models.Category
				if err := catRows.Scan(&cat.ID, &cat.Name, &cat.Description, &cat.Color, &cat.UserID); err == nil {
					categories = append(categories, cat)
				}
			}
			catRows.Close() // Close here to avoid resource leak
			if len(categories) > 0 {
				t.Categories = categories
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

func AddTransaction(w http.ResponseWriter, r *http.Request) {
	// Get the user ID from the authentication context
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	var t models.Transaction
	err := json.NewDecoder(r.Body).Decode(&t)
	if err != nil {
		log.Printf("Error decoding transaction: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Generate a unique ID if not provided
	if t.ID == "" {
		t.ID = generateID()
	}

	// Set current time if date is not provided
	if t.Date.IsZero() {
		t.Date = time.Now()
	}

	// Set transaction date to date if not provided
	if t.TransactionDate.IsZero() {
		t.TransactionDate = t.Date
	}

	// Set the user ID from the authentication context
	t.UserID = userID

	// If EnteredBy is not explicitly provided, use the user ID
	if t.EnteredBy == "" {
		t.EnteredBy = userID
	}

	// Check if the optional column exists using PostgreSQL info schema
	var hasOptionalColumn bool
	err = database.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'transactions' AND column_name = 'optional'
		)
	`).Scan(&hasOptionalColumn)

	if err != nil {
		log.Printf("Error checking for optional column: %v", err)
		hasOptionalColumn = false
	}

	// Check if the user_id column exists
	var hasUserIdColumn bool
	err = database.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'transactions' AND column_name = 'user_id'
		)
	`).Scan(&hasUserIdColumn)

	if err != nil {
		log.Printf("Error checking for user_id column: %v", err)
		hasUserIdColumn = false
	}

	// Check if the transaction_date column exists
	var hasTransactionDateColumn bool
	err = database.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'transactions' AND column_name = 'transaction_date'
		)
	`).Scan(&hasTransactionDateColumn)

	if err != nil {
		log.Printf("Error checking for transaction_date column: %v", err)
		hasTransactionDateColumn = false
	}

	// If columns don't exist, add them
	if !hasOptionalColumn {
		log.Printf("Adding optional column to transactions table")
		_, err = database.DB.Exec(`ALTER TABLE transactions ADD COLUMN optional BOOLEAN NOT NULL DEFAULT false`)
		if err != nil {
			log.Printf("Error adding optional column: %v", err)
			http.Error(w, "Error updating database schema: "+err.Error(), http.StatusInternalServerError)
			return
		}
		hasOptionalColumn = true
	}

	if !hasUserIdColumn {
		log.Printf("Adding user_id column to transactions table")
		_, err = database.DB.Exec(`ALTER TABLE transactions ADD COLUMN user_id TEXT`)
		if err != nil {
			log.Printf("Error adding user_id column: %v", err)
			http.Error(w, "Error updating database schema: "+err.Error(), http.StatusInternalServerError)
			return
		}
		hasUserIdColumn = true
	}

	if !hasTransactionDateColumn {
		log.Printf("Adding transaction_date column to transactions table")
		_, err = database.DB.Exec(`ALTER TABLE transactions ADD COLUMN transaction_date TIMESTAMP WITH TIME ZONE`)
		if err != nil {
			log.Printf("Error adding transaction_date column: %v", err)
			http.Error(w, "Error updating database schema: "+err.Error(), http.StatusInternalServerError)
			return
		}
		hasTransactionDateColumn = true
	}

	// Check if the transaction_categories table exists
	var hasTransactionCategoriesTable bool
	err = database.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_name = 'transaction_categories'
		)
	`).Scan(&hasTransactionCategoriesTable)

	if err != nil {
		log.Printf("Error checking for transaction_categories table: %v", err)
		hasTransactionCategoriesTable = false
	}

	// Create transaction_categories table if it doesn't exist
	if !hasTransactionCategoriesTable {
		log.Printf("Creating transaction_categories table")
		_, err = database.DB.Exec(`
			CREATE TABLE IF NOT EXISTS transaction_categories (
				id SERIAL PRIMARY KEY,
				transaction_id TEXT NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
				category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
				amount NUMERIC(15,2) NOT NULL,
				created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(transaction_id, category_id)
			)
		`)
		if err != nil {
			log.Printf("Error creating transaction_categories table: %v", err)
			http.Error(w, "Error creating transaction_categories table: "+err.Error(), http.StatusInternalServerError)
			return
		}
		hasTransactionCategoriesTable = true
		log.Println("Created transaction_categories table")
	}

	// Start a database transaction to ensure both the transaction and its category associations are saved atomically
	tx, err := database.DB.Begin()
	if err != nil {
		log.Printf("Error starting database transaction: %v", err)
		http.Error(w, "Error starting database transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Build query based on available columns
	insertQuery := `
		INSERT INTO transactions (id, amount, description, date, transaction_date, type, pay_to, paid, paid_date, entered_by, note`

	paramCount := 11
	insertValues := fmt.Sprintf("$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11")
	insertArgs := []interface{}{t.ID, t.Amount, t.Description, t.Date, t.TransactionDate, t.Type, t.PayTo, t.Paid, t.PaidDate, t.EnteredBy, t.Note}

	if hasOptionalColumn {
		insertQuery += `, optional`
		paramCount++
		insertValues += fmt.Sprintf(", $%d", paramCount)
		insertArgs = append(insertArgs, t.Optional)
	}

	if hasUserIdColumn {
		insertQuery += `, user_id`
		paramCount++
		insertValues += fmt.Sprintf(", $%d", paramCount)
		insertArgs = append(insertArgs, t.UserID)
	}

	insertQuery += `) VALUES (` + insertValues + `)`

	log.Printf("Executing query: %s with %d args", insertQuery, len(insertArgs))

	_, err = tx.Exec(insertQuery, insertArgs...)
	if err != nil {
		log.Printf("Error inserting transaction: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Handle category associations if present
	if len(t.Categories) > 0 {
		for _, category := range t.Categories {
			// Ensure the category exists and get its ID
			var categoryID string
			err = tx.QueryRow(`
				SELECT id FROM ynab_categories 
				WHERE name = $1 AND user_id = $2
			`, category.Name, userID).Scan(&categoryID)

			if err != nil {
				if err == sql.ErrNoRows {
					// Category doesn't exist - now we can't create it, so just log a warning
					log.Printf("Warning: Category %s not found in YNAB categories", category.Name)
					continue
				} else {
					log.Printf("Error finding category %s: %v", category.Name, err)
					http.Error(w, "Error finding category: "+err.Error(), http.StatusInternalServerError)
					return
				}
			}

			// Associate the transaction with the category
			_, err = tx.Exec(`
				INSERT INTO transaction_categories (transaction_id, category_id, amount)
				VALUES ($1, $2, $3)
			`, t.ID, categoryID, t.Amount)

			if err != nil {
				log.Printf("Error associating transaction with category: %v", err)
				http.Error(w, "Error associating transaction with category: "+err.Error(), http.StatusInternalServerError)
				return
			}

			log.Printf("Associated transaction %s with category %s (ID: %s)", t.ID, category.Name, categoryID)
		}
	} else if t.Type != "" {
		// If we have a 'type' field but no explicit categories, try to use it as a category
		// This is for backward compatibility with the previous approach
		var categoryID string
		err = tx.QueryRow(`
			SELECT id FROM ynab_categories 
			WHERE name = $1 AND user_id = $2
		`, t.Type, userID).Scan(&categoryID)

		if err == nil {
			// We found a category matching the 'type' field
			_, err = tx.Exec(`
				INSERT INTO transaction_categories (transaction_id, category_id, amount)
				VALUES ($1, $2, $3)
			`, t.ID, categoryID, t.Amount)

			if err != nil {
				log.Printf("Error associating transaction with type-derived category: %v", err)
				// This is not a critical error, so we'll just log it but continue
			} else {
				log.Printf("Associated transaction %s with category derived from type: %s (ID: %s)", t.ID, t.Type, categoryID)
			}
		} else if err != sql.ErrNoRows {
			// If it's an error other than "not found", log it
			log.Printf("Error checking for category based on type %s: %v", t.Type, err)
		}
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		log.Printf("Error committing transaction: %v", err)
		http.Error(w, "Error committing transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// If we were successful, try to load the categories for the response
	if len(t.Categories) == 0 {
		rows, err := database.DB.Query(`
			SELECT c.id, c.name, c.name as description, '#3498DB' as color, c.user_id
			FROM ynab_categories c
			JOIN transaction_categories tc ON c.id = tc.category_id
			WHERE tc.transaction_id = $1
		`, t.ID)

		if err == nil {
			defer rows.Close()
			var categories []models.Category
			for rows.Next() {
				var cat models.Category
				err = rows.Scan(&cat.ID, &cat.Name, &cat.Description, &cat.Color, &cat.UserID)
				if err == nil {
					categories = append(categories, cat)
				}
			}
			if len(categories) > 0 {
				t.Categories = categories
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

func UpdateTransaction(w http.ResponseWriter, r *http.Request) {
	// Get the user ID from the authentication context
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	var t models.Transaction
	err := json.NewDecoder(r.Body).Decode(&t)
	if err != nil {
		log.Printf("Error decoding transaction update: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Updating transaction %s, paid status: %v, note: %s", id, t.Paid, t.Note)

	// Automatically manage paid_date based on paid status
	if t.Paid {
		// If paid is true and no paid date is provided, set it to current date
		if t.PaidDate == "" {
			t.PaidDate = time.Now().Format("2006-01-02")
			log.Printf("Automatically setting paid_date to %s for transaction %s", t.PaidDate, id)
		}
	} else {
		// If paid is false, always clear the paid date
		t.PaidDate = ""
		log.Printf("Clearing paid_date for unpaid transaction %s", id)
	}

	// Check if the optional column exists
	var hasOptionalColumn bool
	err = database.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'transactions' AND column_name = 'optional'
		)
	`).Scan(&hasOptionalColumn)

	if err != nil {
		log.Printf("Error checking for optional column: %v", err)
		hasOptionalColumn = false
	}

	// Check if the user_id column exists
	var hasUserIdColumn bool
	err = database.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'transactions' AND column_name = 'user_id'
		)
	`).Scan(&hasUserIdColumn)

	if err != nil {
		log.Printf("Error checking for user_id column: %v", err)
		hasUserIdColumn = false
	}

	// Check if the transaction_date column exists
	var hasTransactionDateColumn bool
	err = database.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'transactions' AND column_name = 'transaction_date'
		)
	`).Scan(&hasTransactionDateColumn)

	if err != nil {
		log.Printf("Error checking for transaction_date column: %v", err)
		hasTransactionDateColumn = false
	}

	// Check if the transaction_categories table exists
	var hasTransactionCategoriesTable bool
	err = database.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_name = 'transaction_categories'
		)
	`).Scan(&hasTransactionCategoriesTable)

	if err != nil {
		log.Printf("Error checking for transaction_categories table: %v", err)
		hasTransactionCategoriesTable = false
	}

	// Get the original transaction's owner to check permissions
	var originalOwnerID sql.NullString
	var originalPayTo string
	var originalEnteredBy string
	var originalNote string
	err = database.DB.QueryRow(
		"SELECT user_id, pay_to, entered_by, note FROM transactions WHERE id = $1", id,
	).Scan(&originalOwnerID, &originalPayTo, &originalEnteredBy, &originalNote)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Transaction not found", http.StatusNotFound)
		} else {
			log.Printf("Error retrieving original transaction: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	log.Printf("Original transaction values - PayTo: %s, EnteredBy: %s, Note: %s",
		originalPayTo, originalEnteredBy, originalNote)

	// Make sure we're not losing the original values if not specified in the update
	if t.PayTo == "" {
		t.PayTo = originalPayTo
		log.Printf("Using original PayTo value: %s", t.PayTo)
	}

	if t.EnteredBy == "" {
		t.EnteredBy = originalEnteredBy
		log.Printf("Using original EnteredBy value: %s", t.EnteredBy)
	}

	// Handle note updates with care - log both before and after
	log.Printf("Note update: original '%s' -> new '%s'", originalNote, t.Note)

	// Check if the user has permission to update this transaction
	if originalOwnerID.Valid && originalOwnerID.String != userID {
		hasPermission := middleware.CheckUserPermission(userID, originalOwnerID.String, models.ResourceTransactions, models.PermissionWrite)
		if !hasPermission {
			log.Printf("Permission denied for user %s to update transaction %s owned by %s", userID, id, originalOwnerID.String)
			http.Error(w, "You don't have permission to update this transaction", http.StatusForbidden)
			return
		}
	}

	// Start a database transaction to ensure both the transaction update and category associations are saved atomically
	tx, err := database.DB.Begin()
	if err != nil {
		log.Printf("Error starting database transaction: %v", err)
		http.Error(w, "Error starting database transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() {
		if err != nil {
			log.Printf("Rolling back transaction due to error: %v", err)
			tx.Rollback()
		}
	}()

	// Build the update query based on available columns
	// Only update fields that are actually provided, not everything
	updateParts := []string{}
	updateArgs := []interface{}{}
	paramCount := 0

	// Only add fields to update query if they're non-zero values
	// Amount always needs to be updated
	updateParts = append(updateParts, fmt.Sprintf("amount = $%d", paramCount+1))
	updateArgs = append(updateArgs, t.Amount)
	paramCount++

	// Add description if provided
	if t.Description != "" {
		updateParts = append(updateParts, fmt.Sprintf("description = $%d", paramCount+1))
		updateArgs = append(updateArgs, t.Description)
		paramCount++
	}

	// Always update note field (even if empty)
	updateParts = append(updateParts, fmt.Sprintf("note = $%d", paramCount+1))
	updateArgs = append(updateArgs, t.Note)
	paramCount++
	log.Printf("Adding note: '%s' to update query", t.Note)

	// Add date if it's not a zero value
	if !t.Date.IsZero() {
		updateParts = append(updateParts, fmt.Sprintf("date = $%d", paramCount+1))
		updateArgs = append(updateArgs, t.Date.Format("2006-01-02"))
		paramCount++
	}

	// Add type if provided
	if t.Type != "" {
		updateParts = append(updateParts, fmt.Sprintf("type = $%d", paramCount+1))
		updateArgs = append(updateArgs, t.Type)
		paramCount++
	}

	// Add pay_to if provided
	if t.PayTo != "" {
		updateParts = append(updateParts, fmt.Sprintf("pay_to = $%d", paramCount+1))
		updateArgs = append(updateArgs, t.PayTo)
		paramCount++
	}

	// Always update paid status
	updateParts = append(updateParts, fmt.Sprintf("paid = $%d", paramCount+1))
	updateArgs = append(updateArgs, t.Paid)
	paramCount++

	// Always update paid_date
	updateParts = append(updateParts, fmt.Sprintf("paid_date = $%d", paramCount+1))
	updateArgs = append(updateArgs, t.PaidDate)
	paramCount++

	// Add entered_by if provided
	if t.EnteredBy != "" {
		updateParts = append(updateParts, fmt.Sprintf("entered_by = $%d", paramCount+1))
		updateArgs = append(updateArgs, t.EnteredBy)
		paramCount++
	}

	// Add transaction_date if it exists and is provided
	if hasTransactionDateColumn && !t.TransactionDate.IsZero() {
		updateParts = append(updateParts, fmt.Sprintf("transaction_date = $%d", paramCount+1))
		updateArgs = append(updateArgs, t.TransactionDate.Format("2006-01-02"))
		paramCount++
	}

	// Add optional if the column exists
	if hasOptionalColumn {
		updateParts = append(updateParts, fmt.Sprintf("optional = $%d", paramCount+1))
		updateArgs = append(updateArgs, t.Optional)
		paramCount++
	}

	// Finalize the update query
	updateQuery := fmt.Sprintf("UPDATE transactions SET %s WHERE id = $%d",
		strings.Join(updateParts, ", "), paramCount+1)
	updateArgs = append(updateArgs, id)

	log.Printf("Executing query: %s with args: %v", updateQuery, updateArgs)
	result, err := tx.Exec(updateQuery, updateArgs...)
	if err != nil {
		log.Printf("Error updating transaction: %v", err)
		http.Error(w, "Error updating transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Error getting rows affected: %v", err)
	} else if rowsAffected == 0 {
		log.Printf("No rows were affected by update for transaction %s", id)
	} else {
		log.Printf("Updated %d rows for transaction %s", rowsAffected, id)
	}

	// Update category associations if the transaction_categories table exists
	if hasTransactionCategoriesTable && len(t.Categories) > 0 {
		// Remove existing category associations
		_, err = tx.Exec(`DELETE FROM transaction_categories WHERE transaction_id = $1`, id)
		if err != nil {
			log.Printf("Error removing existing category associations: %v", err)
			http.Error(w, "Error updating category associations: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Add new category associations
		for _, category := range t.Categories {
			// Ensure the category exists and get its ID
			var categoryID string
			err = tx.QueryRow(`
				SELECT id FROM ynab_categories 
				WHERE name = $1 AND user_id = $2
			`, category.Name, userID).Scan(&categoryID)

			if err != nil {
				if err == sql.ErrNoRows {
					// Category doesn't exist - now we can't create it, so just log a warning
					log.Printf("Warning: Category %s not found in YNAB categories", category.Name)
					continue
				} else {
					log.Printf("Error finding category %s: %v", category.Name, err)
					http.Error(w, "Error finding category: "+err.Error(), http.StatusInternalServerError)
					return
				}
			}

			// Associate the transaction with the category
			_, err = tx.Exec(`
				INSERT INTO transaction_categories (transaction_id, category_id, amount)
				VALUES ($1, $2, $3)
			`, t.ID, categoryID, t.Amount)

			if err != nil {
				log.Printf("Error associating transaction with category: %v", err)
				http.Error(w, "Error associating transaction with category: "+err.Error(), http.StatusInternalServerError)
				return
			}

			log.Printf("Associated transaction %s with category %s (ID: %s)", t.ID, category.Name, categoryID)
		}
	} else if hasTransactionCategoriesTable && t.Type != "" && len(t.Categories) == 0 {
		// Backward compatibility: if no explicit categories but Type field is set, use it as a category
		var categoryID string
		err = tx.QueryRow(`
			SELECT id FROM ynab_categories 
			WHERE name = $1 AND user_id = $2
		`, t.Type, userID).Scan(&categoryID)

		if err == nil {
			// We found a category matching the 'type' field
			// First remove any existing associations
			_, err = tx.Exec(`DELETE FROM transaction_categories WHERE transaction_id = $1`, id)
			if err != nil {
				log.Printf("Error removing existing category associations: %v", err)
			}

			// Add the new association
			_, err = tx.Exec(`
				INSERT INTO transaction_categories (transaction_id, category_id, amount)
				VALUES ($1, $2, $3)
			`, id, categoryID, t.Amount)

			if err != nil {
				log.Printf("Error associating transaction with type-derived category: %v", err)
				// Not a critical error, just log it
			} else {
				log.Printf("Associated transaction %s with category derived from type: %s (ID: %s)", id, t.Type, categoryID)
			}
		}
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		log.Printf("Error committing transaction update: %v", err)
		http.Error(w, "Error committing transaction update: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch the updated transaction with its categories for the response
	t.ID = id // Ensure the ID is set

	// Load updated categories
	if hasTransactionCategoriesTable {
		rows, err := database.DB.Query(`
			SELECT c.id, c.name, c.description, c.color, c.user_id
			FROM categories c
			JOIN transaction_categories tc ON c.id = tc.category_id
			WHERE tc.transaction_id = $1
		`, id)

		if err == nil {
			defer rows.Close()
			var categories []models.Category
			for rows.Next() {
				var cat models.Category
				err = rows.Scan(&cat.ID, &cat.Name, &cat.Description, &cat.Color, &cat.UserID)
				if err == nil {
					categories = append(categories, cat)
				}
			}
			if len(categories) > 0 {
				t.Categories = categories
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

func DeleteTransaction(w http.ResponseWriter, r *http.Request) {
	// Get the user ID from the authentication context
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	log.Printf("Attempting to delete transaction %s by user %s", id, userID)

	// First check if the user is Patrick Bennett (has special superadmin status)
	if userID == "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2" {
		log.Printf("User is Patrick Bennett (superadmin). Bypassing permission checks for transaction deletion.")

		// Execute the delete directly
		result, err := database.DB.Exec("DELETE FROM transactions WHERE id = $1", id)
		if err != nil {
			log.Printf("Error deleting transaction: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			log.Printf("Error getting rows affected: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if rowsAffected == 0 {
			log.Printf("No transaction found with id %s", id)
			http.Error(w, "Transaction not found", http.StatusNotFound)
			return
		}

		log.Printf("Successfully deleted transaction %s by superadmin", id)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Check if the user is a superadmin by role
	var userRole string
	err := database.DB.QueryRow("SELECT role FROM users WHERE id = $1", userID).Scan(&userRole)
	if err == nil && (userRole == "superadmin" || userRole == "admin") {
		log.Printf("User %s has role %s. Bypassing permission checks for transaction deletion.", userID, userRole)

		// Execute the delete directly
		result, err := database.DB.Exec("DELETE FROM transactions WHERE id = $1", id)
		if err != nil {
			log.Printf("Error deleting transaction: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			log.Printf("Error getting rows affected: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if rowsAffected == 0 {
			log.Printf("No transaction found with id %s", id)
			http.Error(w, "Transaction not found", http.StatusNotFound)
			return
		}

		log.Printf("Successfully deleted transaction %s by admin", id)
		w.WriteHeader(http.StatusOK)
		return
	}

	// For regular users, check if the transaction exists and who owns it
	var transactionUserID string
	err = database.DB.QueryRow(`
		SELECT user_id FROM transactions WHERE id = $1
	`, id).Scan(&transactionUserID)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("Transaction %s not found", id)
			http.Error(w, "Transaction not found", http.StatusNotFound)
			return
		}
		log.Printf("Error checking transaction owner: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Transaction %s belongs to user %s", id, transactionUserID)

	// If the user is not the owner, check if they have write permission
	if transactionUserID != userID {
		hasPermission := middleware.CheckUserPermission(userID, transactionUserID, models.ResourceTransactions, models.PermissionWrite)

		if !hasPermission {
			log.Printf("User %s does not have permission to delete transaction %s owned by %s",
				userID, id, transactionUserID)
			http.Error(w, "You don't have permission to delete this transaction", http.StatusForbidden)
			return
		}

		log.Printf("User %s has permission to delete transaction %s owned by %s",
			userID, id, transactionUserID)
	}

	// Execute the delete query
	result, err := database.DB.Exec("DELETE FROM transactions WHERE id = $1", id)

	if err != nil {
		log.Printf("Error deleting transaction: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Check if any rows were affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Error getting rows affected: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		log.Printf("No transaction was deleted with id %s", id)
		http.Error(w, "Transaction not found", http.StatusNotFound)
		return
	}

	log.Printf("Successfully deleted transaction %s", id)
	w.WriteHeader(http.StatusOK)
}

func generateID() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 16)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// GetUniqueTransactionFields returns unique values for PayTo and EnteredBy fields
func GetUniqueTransactionFields(w http.ResponseWriter, r *http.Request) {
	// Get the user ID from the authentication context
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	log.Printf("Getting unique fields for user: %s", userID)

	// Check if the users table exists
	var hasUsersTable bool
	err := database.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_name = 'users'
		)
	`).Scan(&hasUsersTable)

	if err != nil {
		log.Printf("Error checking for users table: %v", err)
		hasUsersTable = false
	}

	var payToValues []string
	var enteredByValues []string

	// Get all users and build a mapping of IDs to names
	userIDToName := make(map[string]string)

	if hasUsersTable {
		usersRows, err := database.DB.Query(`
			SELECT id, name FROM users
		`)

		if err != nil {
			log.Printf("Error querying users: %v", err)
		} else {
			defer usersRows.Close()
			// Build a mapping of user IDs to names
			for usersRows.Next() {
				var id, name string
				if err := usersRows.Scan(&id, &name); err == nil {
					userIDToName[id] = name
					// Add all user names to PayTo dropdown
					payToValues = append(payToValues, name)
				}
			}
		}
	}

	// Get unique entered_by values from transactions
	enteredByRows, err := database.DB.Query(`
		SELECT DISTINCT entered_by FROM transactions 
		WHERE entered_by IS NOT NULL AND entered_by != ''
	`)

	if err != nil {
		log.Printf("Error querying entered_by values: %v", err)
	} else {
		defer enteredByRows.Close()

		// Process each entered_by value
		for enteredByRows.Next() {
			var enteredByID string
			if err := enteredByRows.Scan(&enteredByID); err == nil {
				// Look up the user name for this ID
				if name, exists := userIDToName[enteredByID]; exists {
					// Add the user name to the enteredBy dropdown
					enteredByValues = append(enteredByValues, name)
					log.Printf("Mapped entered_by ID %s to name %s", enteredByID, name)
				} else {
					// ID doesn't map to a known user, use it directly
					enteredByValues = append(enteredByValues, enteredByID)
					log.Printf("Using raw entered_by ID as name: %s", enteredByID)
				}
			}
		}
	}

	// Remove duplicates from enteredByValues
	uniqueEnteredBy := make(map[string]bool)
	var uniqueEnteredByValues []string

	for _, val := range enteredByValues {
		if !uniqueEnteredBy[val] {
			uniqueEnteredBy[val] = true
			uniqueEnteredByValues = append(uniqueEnteredByValues, val)
		}
	}

	// If we have no values, use hardcoded defaults
	if len(payToValues) == 0 {
		payToValues = []string{"Sarah", "Patrick"}
	}

	if len(uniqueEnteredByValues) == 0 {
		uniqueEnteredByValues = []string{"Sarah", "Patrick"}
	}

	log.Printf("Final payTo values (%d): %v", len(payToValues), payToValues)
	log.Printf("Final enteredBy values (%d): %v", len(uniqueEnteredByValues), uniqueEnteredByValues)

	// Create response with the unique values
	response := struct {
		PayTo     []string `json:"payTo"`
		EnteredBy []string `json:"enteredBy"`
		Category  []string `json:"category"`
	}{
		PayTo:     payToValues,
		EnteredBy: uniqueEnteredByValues,
		Category:  []string{},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
