package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
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

	// First check if the optional column exists
	var hasOptionalColumn bool
	err := database.DB.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM pragma_table_info('transactions') 
		WHERE name = 'optional'
	`).Scan(&hasOptionalColumn)

	if err != nil {
		log.Printf("Error checking for optional column: %v", err)
		hasOptionalColumn = false
	}

	// Check if the userId column exists
	var hasUserIdColumn bool
	err = database.DB.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM pragma_table_info('transactions') 
		WHERE name = 'userId'
	`).Scan(&hasUserIdColumn)

	if err != nil {
		log.Printf("Error checking for userId column: %v", err)
		hasUserIdColumn = false
	}

	// Base query with the appropriate columns
	var query string
	if hasOptionalColumn && hasUserIdColumn {
		query = `
			SELECT id, amount, description, date, transaction_date, type, payTo, paid, paidDate, enteredBy, optional, userId 
			FROM transactions 
			WHERE 1=1
		`
	} else if hasOptionalColumn {
		query = `
			SELECT id, amount, description, date, transaction_date, type, payTo, paid, paidDate, enteredBy, optional 
			FROM transactions 
			WHERE 1=1
		`
	} else {
		query = `
			SELECT id, amount, description, date, transaction_date, type, payTo, paid, paidDate, enteredBy 
			FROM transactions 
			WHERE 1=1
		`
	}

	args := []interface{}{}

	// Add user ID filter if the column exists
	if hasUserIdColumn {
		// Check if the user is in the approved sharing group (Sarah and Patrick)
		var isApprovedUser bool
		err := database.DB.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM users 
				WHERE id = ? AND 
				(name = 'Patrick' OR name = 'Sarah' OR 
				 username = 'patrick' OR username = 'sarah' OR
				 id = 'UgwzWuP8iHNF8nhqDHMwFFcg8Sc2' OR
				 id = '4fWxBBh9NYhMlwop2SJGt1ZzzI22' OR
				 username = 'sarah.elizabeth.wallis@gmail.com')
			)
		`, userID).Scan(&isApprovedUser)

		if err != nil {
			log.Printf("Error checking if user is approved for sharing: %v", err)
			isApprovedUser = false
		}

		if isApprovedUser {
			// If the user is Patrick or Sarah, they can see all transactions
			// from both of them or any legacy transactions without userId
			query += ` AND (
				userId = ? OR 
				userId IS NULL OR 
				userId IN (
					SELECT id FROM users 
					WHERE name = 'Patrick' OR name = 'Sarah' OR 
						  username = 'patrick' OR username = 'sarah' OR
						  id = 'UgwzWuP8iHNF8nhqDHMwFFcg8Sc2' OR
						  id = '4fWxBBh9NYhMlwop2SJGt1ZzzI22' OR
						  username = 'sarah.elizabeth.wallis@gmail.com'
				)
			)`
			args = append(args, userID)
			log.Printf("Fetching ALL shared transactions for approved user %s", userID)
		} else {
			// For other users, only show their own transactions
			query += " AND (userId = ?)"
			args = append(args, userID)
			log.Printf("Fetching only personal transactions for user %s", userID)
		}
	}

	// Parse query parameters
	payTo := r.URL.Query().Get("payTo")
	if payTo != "" {
		query += " AND payTo LIKE ?"
		args = append(args, "%"+payTo+"%")
	}

	enteredBy := r.URL.Query().Get("enteredBy")
	if enteredBy != "" {
		query += " AND enteredBy LIKE ?"
		args = append(args, "%"+enteredBy+"%")
	}

	paid := r.URL.Query().Get("paid")
	if paid != "" {
		query += " AND paid = ?"
		args = append(args, paid == "true")
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

		var err error
		if hasOptionalColumn && hasUserIdColumn {
			err = rows.Scan(&t.ID, &t.Amount, &t.Description, &t.Date, &transactionDate, &t.Type, &t.PayTo, &t.Paid, &paidDate, &t.EnteredBy, &t.Optional, &userId)
			if userId.Valid {
				t.UserID = userId.String
			}
		} else if hasOptionalColumn {
			err = rows.Scan(&t.ID, &t.Amount, &t.Description, &t.Date, &transactionDate, &t.Type, &t.PayTo, &t.Paid, &paidDate, &t.EnteredBy, &t.Optional)
		} else {
			err = rows.Scan(&t.ID, &t.Amount, &t.Description, &t.Date, &transactionDate, &t.Type, &t.PayTo, &t.Paid, &paidDate, &t.EnteredBy)
			// Set default value for optional
			t.Optional = false
		}

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

	// First check if the optional column exists
	var hasOptionalColumn bool
	err := database.DB.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM pragma_table_info('transactions') 
		WHERE name = 'optional'
	`).Scan(&hasOptionalColumn)

	if err != nil {
		log.Printf("Error checking for optional column: %v", err)
		hasOptionalColumn = false
	}

	// Check if the userId column exists
	var hasUserIdColumn bool
	err = database.DB.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM pragma_table_info('transactions') 
		WHERE name = 'userId'
	`).Scan(&hasUserIdColumn)

	if err != nil {
		log.Printf("Error checking for userId column: %v", err)
		hasUserIdColumn = false
	}

	var t models.Transaction
	var paidDate sql.NullString
	var transactionDate sql.NullTime
	var userId sql.NullString

	var query string
	if hasOptionalColumn && hasUserIdColumn {
		query = `
			SELECT id, amount, description, date, transaction_date, type, payTo, paid, paidDate, enteredBy, optional, userId 
			FROM transactions 
			WHERE id = ?
		`
	} else if hasOptionalColumn {
		query = `
			SELECT id, amount, description, date, transaction_date, type, payTo, paid, paidDate, enteredBy, optional 
			FROM transactions 
			WHERE id = ?
		`
	} else {
		query = `
			SELECT id, amount, description, date, transaction_date, type, payTo, paid, paidDate, enteredBy 
			FROM transactions 
			WHERE id = ?
		`
	}

	// Add user ID check if the column exists
	if hasUserIdColumn {
		// Check if the user is in the approved sharing group (Sarah and Patrick)
		var isApprovedUser bool
		err := database.DB.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM users 
				WHERE id = ? AND 
				(name = 'Patrick' OR name = 'Sarah' OR 
				 username = 'patrick' OR username = 'sarah' OR
				 id = 'UgwzWuP8iHNF8nhqDHMwFFcg8Sc2' OR
				 id = '4fWxBBh9NYhMlwop2SJGt1ZzzI22' OR
				 username = 'sarah.elizabeth.wallis@gmail.com')
			)
		`, userID).Scan(&isApprovedUser)

		if err != nil {
			log.Printf("Error checking if user is approved for sharing: %v", err)
			isApprovedUser = false
		}

		if isApprovedUser {
			// If the user is Patrick or Sarah, they can see all transactions
			// from both of them or any legacy transactions without userId
			query += ` AND (
				userId = ? OR 
				userId IS NULL OR 
				userId IN (
					SELECT id FROM users 
					WHERE name = 'Patrick' OR name = 'Sarah' OR 
						  username = 'patrick' OR username = 'sarah' OR
						  id = 'UgwzWuP8iHNF8nhqDHMwFFcg8Sc2' OR
						  id = '4fWxBBh9NYhMlwop2SJGt1ZzzI22' OR
						  username = 'sarah.elizabeth.wallis@gmail.com'
				)
			)`

			// Create parameter list based on available columns
			args := []interface{}{id, userID}

			if hasOptionalColumn && hasUserIdColumn {
				err = database.DB.QueryRow(query, args...).Scan(
					&t.ID, &t.Amount, &t.Description, &t.Date, &transactionDate,
					&t.Type, &t.PayTo, &t.Paid, &paidDate, &t.EnteredBy, &t.Optional, &userId)
			} else if hasOptionalColumn {
				err = database.DB.QueryRow(query, args...).Scan(
					&t.ID, &t.Amount, &t.Description, &t.Date, &transactionDate,
					&t.Type, &t.PayTo, &t.Paid, &paidDate, &t.EnteredBy, &t.Optional)
			} else {
				err = database.DB.QueryRow(query, args...).Scan(
					&t.ID, &t.Amount, &t.Description, &t.Date, &transactionDate,
					&t.Type, &t.PayTo, &t.Paid, &paidDate, &t.EnteredBy)
			}

			if err != nil {
				if err == sql.ErrNoRows {
					http.Error(w, "Transaction not found", http.StatusNotFound)
				} else {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				return
			}

			if userId.Valid {
				t.UserID = userId.String
			}
		} else {
			// For other users, only show their own transactions
			query += " AND (userId = ?)"

			// Create parameter list based on available columns
			args := []interface{}{id, userID}

			if hasOptionalColumn && hasUserIdColumn {
				err = database.DB.QueryRow(query, args...).Scan(
					&t.ID, &t.Amount, &t.Description, &t.Date, &transactionDate,
					&t.Type, &t.PayTo, &t.Paid, &paidDate, &t.EnteredBy, &t.Optional, &userId)
			} else if hasOptionalColumn {
				err = database.DB.QueryRow(query, args...).Scan(
					&t.ID, &t.Amount, &t.Description, &t.Date, &transactionDate,
					&t.Type, &t.PayTo, &t.Paid, &paidDate, &t.EnteredBy, &t.Optional)
			} else {
				err = database.DB.QueryRow(query, args...).Scan(
					&t.ID, &t.Amount, &t.Description, &t.Date, &transactionDate,
					&t.Type, &t.PayTo, &t.Paid, &paidDate, &t.EnteredBy)
			}

			if err != nil {
				if err == sql.ErrNoRows {
					http.Error(w, "Transaction not found", http.StatusNotFound)
				} else {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				return
			}

			if userId.Valid {
				t.UserID = userId.String
			}
		}
	} else {
		// No userId column, just query by ID
		args := []interface{}{id}
		if hasOptionalColumn {
			err = database.DB.QueryRow(query, args...).Scan(
				&t.ID, &t.Amount, &t.Description, &t.Date, &transactionDate,
				&t.Type, &t.PayTo, &t.Paid, &paidDate, &t.EnteredBy, &t.Optional)
		} else {
			err = database.DB.QueryRow(query, args...).Scan(
				&t.ID, &t.Amount, &t.Description, &t.Date, &transactionDate,
				&t.Type, &t.PayTo, &t.Paid, &paidDate, &t.EnteredBy)
		}

		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Transaction not found", http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
	}

	if paidDate.Valid {
		t.PaidDate = paidDate.String
	}
	if transactionDate.Valid {
		t.TransactionDate = transactionDate.Time
	} else {
		t.TransactionDate = t.Date // Fall back to entered date if transaction date not available
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

	// Check if the optional column exists
	var hasOptionalColumn bool
	err = database.DB.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM pragma_table_info('transactions') 
		WHERE name = 'optional'
	`).Scan(&hasOptionalColumn)

	if err != nil {
		log.Printf("Error checking for optional column: %v", err)
		hasOptionalColumn = false
	}

	// Check if the userId column exists
	var hasUserIdColumn bool
	err = database.DB.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM pragma_table_info('transactions') 
		WHERE name = 'userId'
	`).Scan(&hasUserIdColumn)

	if err != nil {
		log.Printf("Error checking for userId column: %v", err)
		hasUserIdColumn = false
	}

	// Check if the transaction_date column exists
	var hasTransactionDateColumn bool
	err = database.DB.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM pragma_table_info('transactions') 
		WHERE name = 'transaction_date'
	`).Scan(&hasTransactionDateColumn)

	if err != nil {
		log.Printf("Error checking for transaction_date column: %v", err)
		hasTransactionDateColumn = false
	}

	// If columns don't exist, add them
	if !hasOptionalColumn {
		log.Printf("Adding optional column to transactions table")
		_, err = database.DB.Exec(`ALTER TABLE transactions ADD COLUMN optional BOOLEAN NOT NULL DEFAULT 0`)
		if err != nil {
			log.Printf("Error adding optional column: %v", err)
			http.Error(w, "Error updating database schema: "+err.Error(), http.StatusInternalServerError)
			return
		}
		hasOptionalColumn = true
	}

	if !hasUserIdColumn {
		log.Printf("Adding userId column to transactions table")
		_, err = database.DB.Exec(`ALTER TABLE transactions ADD COLUMN userId TEXT`)
		if err != nil {
			log.Printf("Error adding userId column: %v", err)
			http.Error(w, "Error updating database schema: "+err.Error(), http.StatusInternalServerError)
			return
		}
		hasUserIdColumn = true
	}

	if !hasTransactionDateColumn {
		log.Printf("Adding transaction_date column to transactions table")
		_, err = database.DB.Exec(`ALTER TABLE transactions ADD COLUMN transaction_date DATETIME`)
		if err != nil {
			log.Printf("Error adding transaction_date column: %v", err)
			http.Error(w, "Error updating database schema: "+err.Error(), http.StatusInternalServerError)
			return
		}
		hasTransactionDateColumn = true
	}

	// Build query based on available columns
	insertQuery := `
		INSERT INTO transactions (id, amount, description, date, transaction_date, type, payTo, paid, paidDate, enteredBy`

	insertValues := `?, ?, ?, ?, ?, ?, ?, ?, ?, ?`
	insertArgs := []interface{}{t.ID, t.Amount, t.Description, t.Date, t.TransactionDate, t.Type, t.PayTo, t.Paid, t.PaidDate, t.EnteredBy}

	if hasOptionalColumn {
		insertQuery += `, optional`
		insertValues += `, ?`
		insertArgs = append(insertArgs, t.Optional)
	}

	if hasUserIdColumn {
		insertQuery += `, userId`
		insertValues += `, ?`
		insertArgs = append(insertArgs, t.UserID)
	}

	insertQuery += `) VALUES (` + insertValues + `)`

	log.Printf("Executing query: %s with %d args", insertQuery, len(insertArgs))

	_, err = database.DB.Exec(insertQuery, insertArgs...)
	if err != nil {
		log.Printf("Error inserting transaction: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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
		SELECT COUNT(*) > 0 
		FROM pragma_table_info('transactions') 
		WHERE name = 'optional'
	`).Scan(&hasOptionalColumn)

	if err != nil {
		log.Printf("Error checking for optional column: %v", err)
		hasOptionalColumn = false
	}

	// Check if the userId column exists
	var hasUserIdColumn bool
	err = database.DB.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM pragma_table_info('transactions') 
		WHERE name = 'userId'
	`).Scan(&hasUserIdColumn)

	if err != nil {
		log.Printf("Error checking for userId column: %v", err)
		hasUserIdColumn = false
	}

	// Check if the transaction_date column exists
	var hasTransactionDateColumn bool
	err = database.DB.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM pragma_table_info('transactions') 
		WHERE name = 'transaction_date'
	`).Scan(&hasTransactionDateColumn)

	if err != nil {
		log.Printf("Error checking for transaction_date column: %v", err)
		hasTransactionDateColumn = false
	}

	// If columns don't exist, add them
	if !hasOptionalColumn {
		log.Printf("Adding optional column to transactions table")
		_, err = database.DB.Exec(`ALTER TABLE transactions ADD COLUMN optional BOOLEAN NOT NULL DEFAULT 0`)
		if err != nil {
			log.Printf("Error adding optional column: %v", err)
			http.Error(w, "Error updating database schema: "+err.Error(), http.StatusInternalServerError)
			return
		}
		hasOptionalColumn = true
	}

	if !hasUserIdColumn {
		log.Printf("Adding userId column to transactions table")
		_, err = database.DB.Exec(`ALTER TABLE transactions ADD COLUMN userId TEXT`)
		if err != nil {
			log.Printf("Error adding userId column: %v", err)
			http.Error(w, "Error updating database schema: "+err.Error(), http.StatusInternalServerError)
			return
		}
		hasUserIdColumn = true
	}

	if !hasTransactionDateColumn {
		log.Printf("Adding transaction_date column to transactions table")
		_, err = database.DB.Exec(`ALTER TABLE transactions ADD COLUMN transaction_date DATETIME`)
		if err != nil {
			log.Printf("Error adding transaction_date column: %v", err)
			http.Error(w, "Error updating database schema: "+err.Error(), http.StatusInternalServerError)
			return
		}
		hasTransactionDateColumn = true
	}

	// Build query based on available columns
	updateQuery := `
		UPDATE transactions 
		SET amount = ?, description = ?, date = ?, transaction_date = ?, type = ?, payTo = ?, paid = ?, paidDate = ?, enteredBy = ?`

	updateArgs := []interface{}{t.Amount, t.Description, t.Date, t.TransactionDate, t.Type, t.PayTo, t.Paid, t.PaidDate, t.EnteredBy}

	if hasOptionalColumn {
		updateQuery += `, optional = ?`
		updateArgs = append(updateArgs, t.Optional)
	}

	if hasUserIdColumn {
		updateQuery += `, userId = ?`
		updateArgs = append(updateArgs, userID) // Use the authenticated user ID
	}

	updateQuery += ` WHERE id = ?`
	updateArgs = append(updateArgs, id)

	// If userId column exists, also check that user owns this transaction or has admin permission
	if hasUserIdColumn {
		updateQuery += ` AND (userId = ? OR userId IS NULL)`
		updateArgs = append(updateArgs, userID)
	}

	log.Printf("Executing update query: %s with %d args", updateQuery, len(updateArgs))

	result, err := database.DB.Exec(updateQuery, updateArgs...)
	if err != nil {
		log.Printf("Error updating transaction: %v", err)
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
		http.Error(w, "Transaction not found or you don't have permission to modify it", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
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

	// Check if the userId column exists
	var hasUserIdColumn bool
	err := database.DB.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM pragma_table_info('transactions') 
		WHERE name = 'userId'
	`).Scan(&hasUserIdColumn)

	if err != nil {
		log.Printf("Error checking for userId column: %v", err)
		hasUserIdColumn = false
	}

	// Build delete query
	deleteQuery := "DELETE FROM transactions WHERE id = ?"
	deleteArgs := []interface{}{id}

	// If userId column exists, also check that user owns this transaction
	if hasUserIdColumn {
		deleteQuery += " AND (userId = ? OR userId IS NULL)"
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

	// Check if the userId column exists
	var hasUserIdColumn bool
	err = database.DB.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM pragma_table_info('transactions') 
		WHERE name = 'userId'
	`).Scan(&hasUserIdColumn)

	if err != nil {
		log.Printf("Error checking for userId column: %v", err)
		hasUserIdColumn = false
	}

	log.Printf("Has userId column: %v", hasUserIdColumn)

	// Build separate queries for payTo and enteredBy
	payToQuery := `
		SELECT DISTINCT payTo 
		FROM transactions 
		WHERE payTo IS NOT NULL
	`

	enteredByQuery := `
		SELECT DISTINCT enteredBy 
		FROM transactions 
		WHERE enteredBy IS NOT NULL
	`

	args := []interface{}{}

	// Add user ID filter if the column exists
	if hasUserIdColumn {
		// Check if the user is in the approved sharing group (Sarah and Patrick)
		var isApprovedUser bool
		err := database.DB.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM users 
				WHERE id = ? AND 
				(name = 'Patrick' OR name = 'Sarah' OR 
				 username = 'patrick' OR username = 'sarah' OR
				 id = 'UgwzWuP8iHNF8nhqDHMwFFcg8Sc2' OR
				 id = '4fWxBBh9NYhMlwop2SJGt1ZzzI22' OR
				 username = 'sarah.elizabeth.wallis@gmail.com')
			)
		`, userID).Scan(&isApprovedUser)

		if err != nil {
			log.Printf("Error checking if user is approved for sharing: %v", err)
			isApprovedUser = false
		}

		if isApprovedUser {
			// If the user is Patrick or Sarah, they can see all transactions
			// from both of them or any legacy transactions without userId
			payToQuery += ` AND (
				userId = ? OR 
				userId IS NULL OR 
				userId IN (
					SELECT id FROM users 
					WHERE name = 'Patrick' OR name = 'Sarah' OR 
						  username = 'patrick' OR username = 'sarah' OR
						  id = 'UgwzWuP8iHNF8nhqDHMwFFcg8Sc2' OR
						  id = '4fWxBBh9NYhMlwop2SJGt1ZzzI22' OR
						  username = 'sarah.elizabeth.wallis@gmail.com'
				)
			)`
			enteredByQuery += ` AND (
				userId = ? OR 
				userId IS NULL OR 
				userId IN (
					SELECT id FROM users 
					WHERE name = 'Patrick' OR name = 'Sarah' OR 
						  username = 'patrick' OR username = 'sarah' OR
						  id = 'UgwzWuP8iHNF8nhqDHMwFFcg8Sc2' OR
						  id = '4fWxBBh9NYhMlwop2SJGt1ZzzI22' OR
						  username = 'sarah.elizabeth.wallis@gmail.com'
				)
			)`
			args = append(args, userID)
			log.Printf("Fetching ALL shared unique fields for approved user %s", userID)
		} else {
			// For other users, only show their own transactions
			payToQuery += " AND (userId = ?)"
			enteredByQuery += " AND (userId = ?)"
			args = append(args, userID)
			log.Printf("Fetching only personal unique fields for user %s", userID)
		}
	}

	// Add ORDER BY to make the results more predictable
	payToQuery += " ORDER BY payTo"
	enteredByQuery += " ORDER BY enteredBy"

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
