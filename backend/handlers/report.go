package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"bennwallet/backend/database"
	"bennwallet/backend/middleware"
	"bennwallet/backend/models"
)

func GetYNABSplits(w http.ResponseWriter, r *http.Request) {
	// Get the user ID from the authentication context
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	var request models.ReportFilter
	log.Println("YNAB Splits Report requested")

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("Error decoding YNAB filter: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Received request: %+v", request)

	// Debug the database records to understand the entered_by format issue
	debugRowsTransaction, err := database.DB.Query(`
		SELECT id, entered_by, pay_to, username_id, created_at
		FROM transactions 
		LIMIT 10
	`)
	if err != nil {
		log.Printf("Error fetching transaction debug data: %v", err)
	} else {
		defer debugRowsTransaction.Close()
		log.Println("DEBUG TRANSACTION SAMPLES:")
		for debugRowsTransaction.Next() {
			var id, enteredBy, payTo, usernameID string
			var createdAt time.Time
			if err := debugRowsTransaction.Scan(&id, &enteredBy, &payTo, &usernameID, &createdAt); err == nil {
				log.Printf("Transaction: id=%s, entered_by=%s, pay_to=%s, username_id=%s, created_at=%v",
					id, enteredBy, payTo, usernameID, createdAt)
			}
		}
	}

	// Let's debug what's in the database for pay_to and entered_by
	var debugRows *sql.Rows
	debugRows, err = database.DB.Query(`
		SELECT DISTINCT pay_to FROM transactions
	`)
	if err != nil {
		log.Printf("Error querying distinct pay_to values: %v", err)
	} else {
		defer debugRows.Close()
		log.Println("Available pay_to values in database:")
		var values []string
		for debugRows.Next() {
			var val string
			if err := debugRows.Scan(&val); err == nil && val != "" {
				values = append(values, val)
			}
		}
		log.Printf("pay_to values: %v", values)
	}

	debugRows, err = database.DB.Query(`
		SELECT DISTINCT entered_by FROM transactions
	`)
	if err != nil {
		log.Printf("Error querying distinct entered_by values: %v", err)
	} else {
		defer debugRows.Close()
		log.Println("Available entered_by values in database:")
		var values []string
		for debugRows.Next() {
			var val string
			if err := debugRows.Scan(&val); err == nil && val != "" {
				values = append(values, val)
			}
		}
		log.Printf("entered_by values: %v", values)
	}

	// Check if columns exist using PostgreSQL information_schema
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

	// Check if the transaction_categories relationship table exists
	var hasCategoriesTable bool
	err = database.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_name = 'transaction_categories'
		)
	`).Scan(&hasCategoriesTable)

	if err != nil {
		log.Printf("Error checking for transaction_categories table: %v", err)
		hasCategoriesTable = false
	}

	log.Printf("Database structure check: hasOptionalColumn=%v, hasTransactionDateColumn=%v, hasUserIdColumn=%v, hasCategoriesTable=%v",
		hasOptionalColumn, hasTransactionDateColumn, hasUserIdColumn, hasCategoriesTable)

	// We'll collect results from both legacy type field and the categories relationship
	var allResults []models.CategoryTotal

	// 1. First handle the legacy type field (category) query
	processLegacyCategories(w, r, request, userID, hasOptionalColumn, hasTransactionDateColumn, hasUserIdColumn, &allResults)

	// 2. If categories table exists, also fetch categories from the relationship table
	if hasCategoriesTable {
		processCategoryRelationships(w, r, request, userID, hasOptionalColumn, hasTransactionDateColumn, hasUserIdColumn, &allResults)
	}

	// 3. Merge and consolidate results
	consolidatedResults := consolidateResults(allResults)

	log.Printf("Returning %d consolidated results", len(consolidatedResults))

	// Always return an array, even if empty
	if consolidatedResults == nil {
		consolidatedResults = []models.CategoryTotal{}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(consolidatedResults); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}

// Handle legacy categories (type field)
func processLegacyCategories(w http.ResponseWriter, r *http.Request, request models.ReportFilter,
	userID string, hasOptionalColumn, hasTransactionDateColumn, hasUserIdColumn bool, results *[]models.CategoryTotal) {

	// Build the base query
	var query string
	query = `
		SELECT type as category, SUM(amount) as total
		FROM transactions
		WHERE 1=1 AND type IS NOT NULL AND type != ''
	`
	var args []interface{}

	// Add user permissions filtering
	if hasUserIdColumn {
		// Get accessible user IDs through permissions system
		accessibleUsers, err := middleware.GetUserAccessibleResources(userID, models.ResourceTransactions, models.PermissionRead)
		if err != nil {
			log.Printf("Error getting accessible resources: %v", err)
			// Fallback to only showing the user's own transactions
			query += " AND user_id = $1"
			args = append(args, userID)
		} else {
			// Build a query to include all accessible user transactions
			if len(accessibleUsers) > 0 {
				placeholders := make([]string, len(accessibleUsers))
				for i := range accessibleUsers {
					placeholders[i] = fmt.Sprintf("$%d", len(args)+1)
					args = append(args, accessibleUsers[i])
				}
				query += fmt.Sprintf(" AND (user_id IN (%s) OR user_id IS NULL)", strings.Join(placeholders, ","))
			} else {
				// Fallback to only showing the user's own transactions
				query += " AND user_id = $1"
				args = append(args, userID)
			}
		}
	}

	// Add date filters
	if request.StartDate != "" {
		query += fmt.Sprintf(" AND date >= $%d", len(args)+1)
		args = append(args, request.StartDate)
	}
	if request.EndDate != "" {
		query += fmt.Sprintf(" AND date <= $%d", len(args)+1)
		args = append(args, request.EndDate)
	}

	// Add transaction date filters if column exists and filters are provided
	if hasTransactionDateColumn && request.TransactionDateMonth != nil && request.TransactionDateYear != nil {
		// Create start and end dates for the month
		startDate := fmt.Sprintf("%d-%02d-01", *request.TransactionDateYear, *request.TransactionDateMonth)
		endDate := fmt.Sprintf("%d-%02d-31", *request.TransactionDateYear, *request.TransactionDateMonth)

		query += fmt.Sprintf(" AND transaction_date >= $%d AND transaction_date <= $%d", len(args)+1, len(args)+2)
		args = append(args, startDate, endDate)
	}

	// Add category filter
	if request.Category != "" {
		query += fmt.Sprintf(" AND type = $%d", len(args)+1)
		args = append(args, request.Category)
	}

	// Add PayTo filter with proper SQL query structuring
	if request.PayTo != "" {
		log.Printf("Filtering by PayTo: '%s'", request.PayTo)

		// Try multiple different matching approaches
		query += fmt.Sprintf(` AND (
			pay_to ILIKE $%d OR 
			pay_to ILIKE $%d OR
			pay_to = $%d
		)`, len(args)+1, len(args)+2, len(args)+3)

		args = append(args, request.PayTo, "%"+request.PayTo+"%", request.PayTo)

		log.Printf("PayTo filter SQL: %s with args: %v", query, args)
	}

	// Add EnteredBy filter with proper SQL query structuring
	if request.EnteredBy != "" {
		log.Printf("Filtering by EnteredBy: '%s'", request.EnteredBy)

		// First query users table to find both ID and name matching the filter
		rows, err := database.DB.Query(`
			SELECT id, name, username FROM users 
			WHERE id = $1 
			   OR name ILIKE $2 
			   OR name ILIKE $3
			   OR username ILIKE $4
			   OR username ILIKE $5
		`, request.EnteredBy, request.EnteredBy, "%"+request.EnteredBy+"%", request.EnteredBy, "%"+request.EnteredBy+"%")

		matchedUserIds := []interface{}{request.EnteredBy}
		matchedUserNames := []string{request.EnteredBy}

		if err != nil {
			log.Printf("Error querying users table: %v", err)
		} else {
			defer rows.Close()
			for rows.Next() {
				var id, name, username string
				if err := rows.Scan(&id, &name, &username); err == nil {
					log.Printf("EnteredBy filter: matched user ID=%s, name=%s, username=%s", id, name, username)
					matchedUserIds = append(matchedUserIds, id)
					matchedUserNames = append(matchedUserNames, name)
					// Also add username as a potential match
					if username != "" && username != name {
						matchedUserNames = append(matchedUserNames, username)
					}
				}
			}
		}

		log.Printf("Matched user IDs for filtering: %v", matchedUserIds)
		log.Printf("Matched user names for filtering: %v", matchedUserNames)

		// Try multiple different matching approaches including both IDs and names
		placeholders := make([]string, len(matchedUserIds)*2)
		for i := range matchedUserIds {
			placeholders[i*2] = fmt.Sprintf("entered_by = $%d", len(args)+i+1)
			placeholders[i*2+1] = fmt.Sprintf("entered_by ILIKE $%d", len(args)+len(matchedUserIds)+i+1)
		}

		query += fmt.Sprintf(" AND (%s)", strings.Join(placeholders, " OR "))

		// Add all matching IDs and names (with wildcards) to args
		for _, id := range matchedUserIds {
			args = append(args, id)
		}
		for _, name := range matchedUserNames {
			args = append(args, "%"+name+"%")
		}

		log.Printf("EnteredBy filter SQL: %s with args: %v", query, args)
	}

	// Add paid filter
	if request.Paid != nil {
		query += fmt.Sprintf(" AND paid = $%d", len(args)+1)
		args = append(args, *request.Paid)
	} else {
		// Default to false (don't filter on paid status)
		query += " AND paid = false"
	}

	// Add optional filter if the column exists
	if hasOptionalColumn && (request.Optional == nil || *request.Optional == false) {
		query += " AND (optional = false OR optional IS NULL)"
	}

	// Add grouping and ordering
	query += " GROUP BY type ORDER BY total DESC"
	log.Printf("Executing legacy category query: %s with args: %v", query, args)

	// Run the query
	rows, err := database.DB.Query(query, args...)
	if err != nil {
		log.Printf("Error executing legacy category query: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var ct models.CategoryTotal
		err := rows.Scan(&ct.Category, &ct.Total)
		if err != nil {
			log.Printf("Error scanning legacy category result: %v", err)
			return
		}
		*results = append(*results, ct)
	}

	// Check for any errors from iterating over rows
	if err = rows.Err(); err != nil {
		log.Printf("Error after scanning all legacy category rows: %v", err)
		return
	}

	log.Printf("Found %d legacy category results", len(*results))
}

// Handle categories from the transaction_categories relationship table
func processCategoryRelationships(w http.ResponseWriter, r *http.Request, request models.ReportFilter,
	userID string, hasOptionalColumn, hasTransactionDateColumn, hasUserIdColumn bool, results *[]models.CategoryTotal) {

	// Build the base query joining transactions with categories
	var query string
	query = `
		SELECT c.name as category, SUM(t.amount) as total
		FROM transactions t
		JOIN transaction_categories tc ON t.id = tc.transaction_id
		JOIN categories c ON tc.category_id = c.id
		WHERE 1=1
	`
	var args []interface{}

	// Add user permissions filtering
	if hasUserIdColumn {
		// Get accessible user IDs through permissions system
		accessibleUsers, err := middleware.GetUserAccessibleResources(userID, models.ResourceTransactions, models.PermissionRead)
		if err != nil {
			log.Printf("Error getting accessible resources: %v", err)
			// Fallback to only showing the user's own transactions
			query += " AND t.user_id = $1"
			args = append(args, userID)
		} else {
			// Build a query to include all accessible user transactions
			if len(accessibleUsers) > 0 {
				placeholders := make([]string, len(accessibleUsers))
				for i := range accessibleUsers {
					placeholders[i] = fmt.Sprintf("$%d", len(args)+1)
					args = append(args, accessibleUsers[i])
				}
				query += fmt.Sprintf(" AND (t.user_id IN (%s) OR t.user_id IS NULL)", strings.Join(placeholders, ","))
			} else {
				// Fallback to only showing the user's own transactions
				query += " AND t.user_id = $1"
				args = append(args, userID)
			}
		}
	}

	// Add date filters
	if request.StartDate != "" {
		query += fmt.Sprintf(" AND t.date >= $%d", len(args)+1)
		args = append(args, request.StartDate)
	}
	if request.EndDate != "" {
		query += fmt.Sprintf(" AND t.date <= $%d", len(args)+1)
		args = append(args, request.EndDate)
	}

	// Add transaction date filters if column exists and filters are provided
	if hasTransactionDateColumn && request.TransactionDateMonth != nil && request.TransactionDateYear != nil {
		// Create start and end dates for the month
		startDate := fmt.Sprintf("%d-%02d-01", *request.TransactionDateYear, *request.TransactionDateMonth)
		endDate := fmt.Sprintf("%d-%02d-31", *request.TransactionDateYear, *request.TransactionDateMonth)

		query += fmt.Sprintf(" AND t.transaction_date >= $%d AND t.transaction_date <= $%d", len(args)+1, len(args)+2)
		args = append(args, startDate, endDate)
	}

	// Add category filter
	if request.Category != "" {
		query += fmt.Sprintf(" AND c.name = $%d", len(args)+1)
		args = append(args, request.Category)
	}

	// Add PayTo filter with proper SQL query structuring
	if request.PayTo != "" {
		log.Printf("Filtering by PayTo in categoriesRelationship: '%s'", request.PayTo)

		// Try multiple different matching approaches
		query += fmt.Sprintf(` AND (
			t.pay_to ILIKE $%d OR 
			t.pay_to ILIKE $%d OR
			t.pay_to = $%d
		)`, len(args)+1, len(args)+2, len(args)+3)

		args = append(args, request.PayTo, "%"+request.PayTo+"%", request.PayTo)

		log.Printf("PayTo filter SQL: %s with args: %v", query, args)
	}

	// Add EnteredBy filter with proper SQL query structuring
	if request.EnteredBy != "" {
		log.Printf("Filtering by EnteredBy in categoriesRelationship: '%s'", request.EnteredBy)

		// First query users table to find both ID and name matching the filter
		rows, err := database.DB.Query(`
			SELECT id, name FROM users WHERE id = $1 OR name ILIKE $2 OR name ILIKE $3
		`, request.EnteredBy, request.EnteredBy, "%"+request.EnteredBy+"%")

		matchedUserIds := []interface{}{request.EnteredBy}
		matchedUserNames := []string{request.EnteredBy}

		if err != nil {
			log.Printf("Error querying users table: %v", err)
		} else {
			defer rows.Close()
			for rows.Next() {
				var id, name string
				if err := rows.Scan(&id, &name); err == nil {
					matchedUserIds = append(matchedUserIds, id)
					matchedUserNames = append(matchedUserNames, name)
				}
			}
		}

		log.Printf("Matched user IDs for filtering in categories relationship: %v", matchedUserIds)
		log.Printf("Matched user names for filtering in categories relationship: %v", matchedUserNames)

		// Try multiple different matching approaches including both IDs and names
		placeholders := make([]string, len(matchedUserIds)*2)
		for i := range matchedUserIds {
			placeholders[i*2] = fmt.Sprintf("t.entered_by = $%d", len(args)+i+1)
			placeholders[i*2+1] = fmt.Sprintf("t.entered_by ILIKE $%d", len(args)+len(matchedUserIds)+i+1)
		}

		query += fmt.Sprintf(" AND (%s)", strings.Join(placeholders, " OR "))

		// Add all matching IDs and names (with wildcards) to args
		for _, id := range matchedUserIds {
			args = append(args, id)
		}
		for _, name := range matchedUserNames {
			args = append(args, "%"+name+"%")
		}

		log.Printf("EnteredBy filter SQL: %s with args: %v", query, args)
	}

	// Add paid filter
	if request.Paid != nil {
		query += fmt.Sprintf(" AND t.paid = $%d", len(args)+1)
		args = append(args, *request.Paid)
	} else {
		// Default to false (don't filter on paid status)
		query += " AND t.paid = false"
	}

	// Add optional filter if the column exists
	if hasOptionalColumn && (request.Optional == nil || *request.Optional == false) {
		query += " AND (t.optional = false OR t.optional IS NULL)"
	}

	// Add grouping and ordering
	query += " GROUP BY c.name ORDER BY total DESC"
	log.Printf("Executing category relationship query: %s with args: %v", query, args)

	// Run the query
	rows, err := database.DB.Query(query, args...)
	if err != nil {
		log.Printf("Error executing category relationship query: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var ct models.CategoryTotal
		err := rows.Scan(&ct.Category, &ct.Total)
		if err != nil {
			log.Printf("Error scanning category relationship result: %v", err)
			return
		}
		*results = append(*results, ct)
	}

	// Check for any errors from iterating over rows
	if err = rows.Err(); err != nil {
		log.Printf("Error after scanning all category relationship rows: %v", err)
		return
	}

	log.Printf("Found %d category relationship results", len(*results))
}

// Merge and consolidate results by summing totals for the same category
func consolidateResults(allResults []models.CategoryTotal) []models.CategoryTotal {
	// Create a map to track totals by category
	categoryTotals := make(map[string]float64)

	// Sum up totals for each category
	for _, result := range allResults {
		categoryTotals[result.Category] += result.Total
	}

	// Create consolidated results
	var consolidatedResults []models.CategoryTotal
	for category, total := range categoryTotals {
		consolidatedResults = append(consolidatedResults, models.CategoryTotal{
			Category: category,
			Total:    total,
		})
	}

	// Sort results by total (descending)
	// Use bubble sort for simplicity (since we don't have sort package imported)
	for i := 0; i < len(consolidatedResults)-1; i++ {
		for j := 0; j < len(consolidatedResults)-i-1; j++ {
			if consolidatedResults[j].Total < consolidatedResults[j+1].Total {
				consolidatedResults[j], consolidatedResults[j+1] = consolidatedResults[j+1], consolidatedResults[j]
			}
		}
	}

	return consolidatedResults
}
