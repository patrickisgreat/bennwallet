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

func GetTransactions(w http.ResponseWriter, r *http.Request) {
	// Get the user ID from the authentication context
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	// Build a query specifically for PostgreSQL
	query := `
		SELECT id, amount, description, date, transaction_date, type, 
		pay_to, paid, paid_date, entered_by, optional, user_id
		FROM transactions 
		WHERE 1=1
	`

	args := []interface{}{}
	paramCounter := 1

	// Add user ID filter
	// Get list of user IDs the current user can access using the permissions system
	accessibleUsers, err := middleware.GetUserAccessibleResources(userID, models.ResourceTransactions, models.PermissionRead)
	if err != nil {
		log.Printf("Error getting accessible resources: %v", err)
		// Fallback to showing only the user's own transactions
		query += fmt.Sprintf(" AND user_id = $%d", paramCounter)
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
		query += fmt.Sprintf(" AND (user_id IN (%s) OR user_id IS NULL)",
			strings.Join(placeholders, ","))
		log.Printf("Fetching transactions for user %s and %d other accessible users", userID, len(accessibleUsers)-1)
	} else {
		// Fallback to showing only the user's own transactions
		query += fmt.Sprintf(" AND user_id = $%d", paramCounter)
		args = append(args, userID)
		paramCounter++
		log.Printf("Fetching only personal transactions for user %s (no permissions found)", userID)
	}

	// Parse query parameters
	payTo := r.URL.Query().Get("payTo")
	if payTo != "" {
		query += fmt.Sprintf(" AND pay_to LIKE $%d", paramCounter)
		search := "%" + payTo + "%"
		args = append(args, search)
		paramCounter++
		log.Printf("Added PayTo LIKE filter: '%s' (as %s)", payTo, search)
	}

	enteredBy := r.URL.Query().Get("enteredBy")
	if enteredBy != "" {
		query += fmt.Sprintf(" AND entered_by LIKE $%d", paramCounter)
		search := "%" + enteredBy + "%"
		args = append(args, search)
		paramCounter++
		log.Printf("Added EnteredBy LIKE filter: '%s' (as %s)", enteredBy, search)
	}

	paid := r.URL.Query().Get("paid")
	if paid != "" {
		query += fmt.Sprintf(" AND paid = $%d", paramCounter)
		args = append(args, paid == "true")
		paramCounter++
	}

	query += " ORDER BY date DESC"

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
		var transactionDate sql.NullTime
		var userId sql.NullString

		err = rows.Scan(&t.ID, &t.Amount, &t.Description, &t.Date, &transactionDate, &t.Type, &t.PayTo, &t.Paid, &paidDate, &t.EnteredBy, &t.Optional, &userId)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if paidDate.Valid {
			t.PaidDate = paidDate.String
		}
		if transactionDate.Valid {
			t.TransactionDate = transactionDate.Time
		} else {
			t.TransactionDate = t.Date // Fall back to entered date if transaction date not available
		}
		if userId.Valid {
			t.UserID = userId.String
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
				SELECT c.id, c.name, c.description, c.color, c.user_id
				FROM categories c
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

	// First check if the optional column exists using PostgreSQL information_schema
	var hasOptionalColumn bool
	err := database.DB.QueryRow(`
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
	var transactionDate sql.NullTime
	var userId sql.NullString

	var query string
	if hasOptionalColumn && hasUserIdColumn {
		query = `
			SELECT id, amount, description, date, transaction_date, type, pay_to, paid, paid_date, entered_by, optional, user_id 
			FROM transactions 
			WHERE id = $1
		`
	} else if hasOptionalColumn {
		query = `
			SELECT id, amount, description, date, transaction_date, type, pay_to, paid, paid_date, entered_by, optional 
			FROM transactions 
			WHERE id = $1
		`
	} else {
		query = `
			SELECT id, amount, description, date, transaction_date, type, pay_to, paid, paid_date, entered_by 
			FROM transactions 
			WHERE id = $1
		`
	}

	var row *sql.Row
	row = database.DB.QueryRow(query, id)

	if hasOptionalColumn && hasUserIdColumn {
		err = row.Scan(&t.ID, &t.Amount, &t.Description, &t.Date, &transactionDate, &t.Type, &t.PayTo, &t.Paid, &paidDate, &t.EnteredBy, &t.Optional, &userId)
	} else if hasOptionalColumn {
		err = row.Scan(&t.ID, &t.Amount, &t.Description, &t.Date, &transactionDate, &t.Type, &t.PayTo, &t.Paid, &paidDate, &t.EnteredBy, &t.Optional)
	} else {
		err = row.Scan(&t.ID, &t.Amount, &t.Description, &t.Date, &transactionDate, &t.Type, &t.PayTo, &t.Paid, &paidDate, &t.EnteredBy)
	}

	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	if paidDate.Valid {
		t.PaidDate = paidDate.String
	}
	if transactionDate.Valid {
		t.TransactionDate = transactionDate.Time
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
			SELECT c.id, c.name, c.description, c.color, c.user_id
			FROM categories c
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
		INSERT INTO transactions (id, amount, description, date, transaction_date, type, pay_to, paid, paid_date, entered_by`

	paramCount := 10
	insertValues := fmt.Sprintf("$1, $2, $3, $4, $5, $6, $7, $8, $9, $10")
	insertArgs := []interface{}{t.ID, t.Amount, t.Description, t.Date, t.TransactionDate, t.Type, t.PayTo, t.Paid, t.PaidDate, t.EnteredBy}

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
			var categoryID int
			err = tx.QueryRow(`
				SELECT id FROM categories 
				WHERE name = $1 AND user_id = $2
			`, category.Name, userID).Scan(&categoryID)

			if err != nil {
				if err == sql.ErrNoRows {
					// Category doesn't exist, create it
					log.Printf("Category %s not found, creating it", category.Name)
					err = tx.QueryRow(`
						INSERT INTO categories (name, description, user_id, color)
						VALUES ($1, $2, $3, $4)
						RETURNING id
					`, category.Name, category.Description, userID, category.Color).Scan(&categoryID)

					if err != nil {
						log.Printf("Error creating category %s: %v", category.Name, err)
						http.Error(w, "Error creating category: "+err.Error(), http.StatusInternalServerError)
						return
					}
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

			log.Printf("Associated transaction %s with category %s (ID: %d)", t.ID, category.Name, categoryID)
		}
	} else if t.Type != "" {
		// If we have a 'type' field but no explicit categories, try to use it as a category
		// This is for backward compatibility with the previous approach
		var categoryID int
		err = tx.QueryRow(`
			SELECT id FROM categories 
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
				log.Printf("Associated transaction %s with category derived from type: %s (ID: %d)", t.ID, t.Type, categoryID)
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
			SELECT c.id, c.name, c.description, c.color, c.user_id
			FROM categories c
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
	err = database.DB.QueryRow(
		"SELECT user_id FROM transactions WHERE id = $1", id,
	).Scan(&originalOwnerID)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Transaction not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Check if the user has permission to update this transaction
	if originalOwnerID.Valid && originalOwnerID.String != userID {
		hasPermission := middleware.CheckUserPermission(userID, originalOwnerID.String, models.ResourceTransactions, models.PermissionWrite)
		if !hasPermission {
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
			tx.Rollback()
		}
	}()

	// Build the update query based on available columns
	updateQuery := `
		UPDATE transactions SET 
		amount = $1, 
		description = $2, 
		date = $3, 
		type = $4, 
		pay_to = $5, 
		paid = $6, 
		paid_date = $7, 
		entered_by = $8
	`
	updateArgs := []interface{}{
		t.Amount, t.Description, t.Date,
		t.Type, t.PayTo, t.Paid, t.PaidDate, t.EnteredBy,
	}
	paramCount := 8

	if hasTransactionDateColumn {
		updateQuery += fmt.Sprintf(", transaction_date = $%d", paramCount+1)
		updateArgs = append(updateArgs, t.TransactionDate)
		paramCount++
	}

	if hasOptionalColumn {
		updateQuery += fmt.Sprintf(", optional = $%d", paramCount+1)
		updateArgs = append(updateArgs, t.Optional)
		paramCount++
	}

	updateQuery += fmt.Sprintf(" WHERE id = $%d", paramCount+1)
	updateArgs = append(updateArgs, id)

	_, err = tx.Exec(updateQuery, updateArgs...)
	if err != nil {
		log.Printf("Error updating transaction: %v", err)
		http.Error(w, "Error updating transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Update category associations if the transaction_categories table exists
	if hasTransactionCategoriesTable {
		// Remove existing category associations
		_, err = tx.Exec(`DELETE FROM transaction_categories WHERE transaction_id = $1`, id)
		if err != nil {
			log.Printf("Error removing existing category associations: %v", err)
			http.Error(w, "Error updating category associations: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Add new category associations if present
		if len(t.Categories) > 0 {
			for _, category := range t.Categories {
				// Ensure the category exists and get its ID
				var categoryID int
				err = tx.QueryRow(`
					SELECT id FROM categories 
					WHERE name = $1 AND user_id = $2
				`, category.Name, userID).Scan(&categoryID)

				if err != nil {
					if err == sql.ErrNoRows {
						// Category doesn't exist, create it
						log.Printf("Category %s not found, creating it", category.Name)
						err = tx.QueryRow(`
							INSERT INTO categories (name, description, user_id, color)
							VALUES ($1, $2, $3, $4)
							RETURNING id
						`, category.Name, category.Description, userID, category.Color).Scan(&categoryID)

						if err != nil {
							log.Printf("Error creating category %s: %v", category.Name, err)
							http.Error(w, "Error creating category: "+err.Error(), http.StatusInternalServerError)
							return
						}
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
				`, id, categoryID, t.Amount)

				if err != nil {
					log.Printf("Error associating transaction with category: %v", err)
					http.Error(w, "Error associating transaction with category: "+err.Error(), http.StatusInternalServerError)
					return
				}

				log.Printf("Associated transaction %s with category %s (ID: %d)", id, category.Name, categoryID)
			}
		} else if t.Type != "" {
			// Backward compatibility: if no explicit categories but Type field is set, use it as a category
			var categoryID int
			err = tx.QueryRow(`
				SELECT id FROM categories 
				WHERE name = $1 AND user_id = $2
			`, t.Type, userID).Scan(&categoryID)

			if err == nil {
				// We found a category matching the 'type' field
				_, err = tx.Exec(`
					INSERT INTO transaction_categories (transaction_id, category_id, amount)
					VALUES ($1, $2, $3)
				`, id, categoryID, t.Amount)

				if err != nil {
					log.Printf("Error associating transaction with type-derived category: %v", err)
					// Not a critical error, just log it
				} else {
					log.Printf("Associated transaction %s with category derived from type: %s (ID: %d)", id, t.Type, categoryID)
				}
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

	// Check if the user_id column exists
	var hasUserIdColumn bool
	err := database.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'transactions' AND column_name = 'user_id'
		)
	`).Scan(&hasUserIdColumn)

	if err != nil {
		log.Printf("Error checking for user_id column: %v", err)
		hasUserIdColumn = false
	}

	// Build delete query
	deleteQuery := "DELETE FROM transactions WHERE id = $1"
	deleteArgs := []interface{}{id}

	// If userId column exists, also check that user owns this transaction
	if hasUserIdColumn {
		deleteQuery += " AND (user_id = $2 OR user_id IS NULL)"
		deleteArgs = append(deleteArgs, userID)
	}

	log.Printf("Executing delete query: %s", deleteQuery)
	result, err := database.DB.Exec(deleteQuery, deleteArgs...)

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
		log.Printf("No transaction found with id %s for user %s", id, userID)
		http.Error(w, "Transaction not found or you don't have permission to delete it", http.StatusNotFound)
		return
	}

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

	// First, let's check if we have any transactions at all
	var transactionCount int
	err := database.DB.QueryRow("SELECT COUNT(*) FROM transactions").Scan(&transactionCount)
	if err != nil {
		log.Printf("Error checking transaction count: %v", err)
	} else {
		log.Printf("Total transactions in database: %d", transactionCount)
	}

	// Check if the user_id column exists using PostgreSQL information_schema
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

	log.Printf("Has userId column: %v", hasUserIdColumn)

	// Build separate queries for pay_to and entered_by (using snake_case for PostgreSQL)
	payToQuery := `
		SELECT DISTINCT pay_to 
		FROM transactions 
		WHERE pay_to IS NOT NULL
	`

	enteredByQuery := `
		SELECT DISTINCT entered_by 
		FROM transactions 
		WHERE entered_by IS NOT NULL
	`

	args := []interface{}{}

	// Add user ID filter if the column exists
	if hasUserIdColumn {
		// Get list of user IDs the current user can access using the permissions system
		accessibleUsers, err := middleware.GetUserAccessibleResources(userID, models.ResourceTransactions, models.PermissionRead)
		if err != nil {
			log.Printf("Error getting accessible resources: %v", err)
			// Fallback to showing only the user's own transactions
			payToQuery += " AND (user_id = $1)"
			enteredByQuery += " AND (user_id = $1)"
			args = append(args, userID) // Only add once for PostgreSQL
			log.Printf("Fetching only personal unique fields for user %s", userID)
		} else if len(accessibleUsers) > 0 {
			// Create placeholders for the SQL IN clause
			payToPlaceholders := make([]string, len(accessibleUsers))
			enteredByPlaceholders := make([]string, len(accessibleUsers))
			payToArgs := make([]interface{}, len(accessibleUsers))
			enteredByArgs := make([]interface{}, len(accessibleUsers))

			for i, userId := range accessibleUsers {
				payToPlaceholders[i] = fmt.Sprintf("$%d", i+1)
				payToArgs[i] = userId
				enteredByPlaceholders[i] = fmt.Sprintf("$%d", i+1)
				enteredByArgs[i] = userId
			}

			// Build query with IN clause and also include NULL userIds for backward compatibility
			payToInClause := fmt.Sprintf("(%s)", strings.Join(payToPlaceholders, ","))
			enteredByInClause := fmt.Sprintf("(%s)", strings.Join(enteredByPlaceholders, ","))

			payToQuery += fmt.Sprintf(" AND (user_id IN %s OR user_id IS NULL)", payToInClause)
			enteredByQuery += fmt.Sprintf(" AND (user_id IN %s OR user_id IS NULL)", enteredByInClause)

			// Add args separately for each query
			payToArgs = append(payToArgs, payToArgs...)             // Duplicate for consistency with placeholders
			enteredByArgs = append(enteredByArgs, enteredByArgs...) // Duplicate for consistency with placeholders

			log.Printf("Fetching unique fields for user %s and %d other accessible users", userID, len(accessibleUsers)-1)
		} else {
			// Fallback to showing only the user's own transactions
			payToQuery += " AND (user_id = $1)"
			enteredByQuery += " AND (user_id = $1)"
			args = append(args, userID) // Only add once for PostgreSQL
			log.Printf("Fetching only personal unique fields for user %s (no permissions found)", userID)
		}
	}

	// Add ORDER BY to make the results more predictable
	payToQuery += " ORDER BY pay_to"
	enteredByQuery += " ORDER BY entered_by"

	log.Printf("PayTo query: %s", payToQuery)
	log.Printf("EnteredBy query: %s", enteredByQuery)
	log.Printf("Query args: %v", args)

	// Query for unique payTo values
	payToRows, err := database.DB.Query(payToQuery, args...)
	if err != nil {
		log.Printf("Error querying unique payTo fields: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer payToRows.Close()

	// Query for unique enteredBy values
	enteredByRows, err := database.DB.Query(enteredByQuery, args...)
	if err != nil {
		log.Printf("Error querying unique enteredBy fields: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer enteredByRows.Close()

	// Use maps to store unique values
	payToValues := make(map[string]bool)
	enteredByValues := make(map[string]bool)

	// Process payTo values
	for payToRows.Next() {
		var payTo sql.NullString
		err := payToRows.Scan(&payTo)
		if err != nil {
			log.Printf("Error scanning payTo row: %v", err)
			continue
		}
		if payTo.Valid {
			payToValues[payTo.String] = true
			log.Printf("Found payTo value: %s", payTo.String)
		}
	}

	// Process enteredBy values
	for enteredByRows.Next() {
		var enteredBy sql.NullString
		err := enteredByRows.Scan(&enteredBy)
		if err != nil {
			log.Printf("Error scanning enteredBy row: %v", err)
			continue
		}
		if enteredBy.Valid {
			enteredByValues[enteredBy.String] = true
			log.Printf("Found enteredBy value: %s", enteredBy.String)
		}
	}

	// Convert maps to slices
	payToSlice := make([]string, 0, len(payToValues))
	for k := range payToValues {
		payToSlice = append(payToSlice, k)
	}

	enteredBySlice := make([]string, 0, len(enteredByValues))
	for k := range enteredByValues {
		enteredBySlice = append(enteredBySlice, k)
	}

	log.Printf("Final payTo values: %v", payToSlice)
	log.Printf("Final enteredBy values: %v", enteredBySlice)

	// Create response
	response := struct {
		PayTo     []string `json:"payTo"`
		EnteredBy []string `json:"enteredBy"`
	}{
		PayTo:     payToSlice,
		EnteredBy: enteredBySlice,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
