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

	// Add user ID check if the column exists
	if hasUserIdColumn {
		// Check if the user has permission to view this transaction
		// First, get the owner of the transaction
		var transactionOwnerID sql.NullString
		ownerErr := database.DB.QueryRow("SELECT user_id FROM transactions WHERE id = $1", id).Scan(&transactionOwnerID)

		if ownerErr != nil && ownerErr != sql.ErrNoRows {
			log.Printf("Error getting transaction owner: %v", ownerErr)
			http.Error(w, "Error checking transaction access", http.StatusInternalServerError)
			return
		}

		var resourceOwnerID string
		if ownerErr == sql.ErrNoRows || !transactionOwnerID.Valid {
			// Transaction doesn't exist or has no owner - allow access to continue with normal query
			// This will be filtered properly in the next step
			resourceOwnerID = userID // Default to the current user
		} else {
			resourceOwnerID = transactionOwnerID.String
		}

		// Check if the user has permission to access this transaction
		hasAccess := middleware.CheckUserPermission(userID, resourceOwnerID, models.ResourceTransactions, models.PermissionRead)

		if !hasAccess && userID != resourceOwnerID {
			log.Printf("User %s does not have permission to access transaction %s owned by %s",
				userID, id, resourceOwnerID)
			http.Error(w, "Transaction not found", http.StatusNotFound)
			return
		}

		// Build the query with access control
		query += " AND (user_id = $2 OR user_id IS NULL)"
		args := []interface{}{id, resourceOwnerID}

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

	// Build query based on available columns
	updateQuery := `
		UPDATE transactions 
		SET amount = $1, description = $2, date = $3, transaction_date = $4, type = $5, pay_to = $6, paid = $7, paid_date = $8, entered_by = $9`

	updateArgs := []interface{}{t.Amount, t.Description, t.Date, t.TransactionDate, t.Type, t.PayTo, t.Paid, t.PaidDate, t.EnteredBy}

	if hasOptionalColumn {
		updateQuery += fmt.Sprintf(`, optional = $%d`, len(updateArgs)+1)
		updateArgs = append(updateArgs, t.Optional)
	}

	if hasUserIdColumn {
		updateQuery += fmt.Sprintf(`, user_id = $%d`, len(updateArgs)+1)
		updateArgs = append(updateArgs, userID) // Use the authenticated user ID
	}

	updateQuery += fmt.Sprintf(` WHERE id = $%d`, len(updateArgs)+1)
	updateArgs = append(updateArgs, id)

	// If userId column exists, also check that user owns this transaction or has admin permission
	if hasUserIdColumn {
		updateQuery += fmt.Sprintf(` AND (user_id = $%d OR user_id IS NULL)`, len(updateArgs)+1)
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
