package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"bennwallet/backend/database"
	"bennwallet/backend/middleware"
	"bennwallet/backend/models"
)

func GetYNABSplits(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userID := r.Context().Value(middleware.UserIDKey).(string)

	// Decode request body
	var request models.ReportFilter
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Check if optional column exists
	var hasOptionalColumn bool
	err := database.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 
			FROM information_schema.columns 
			WHERE table_name = 'transactions' 
			AND column_name = 'optional'
		)
	`).Scan(&hasOptionalColumn)
	if err != nil {
		log.Printf("Error checking for optional column: %v", err)
		hasOptionalColumn = false
	}

	// Check if transaction_date column exists
	var hasTransactionDateColumn bool
	err = database.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 
			FROM information_schema.columns 
			WHERE table_name = 'transactions' 
			AND column_name = 'transaction_date'
		)
	`).Scan(&hasTransactionDateColumn)
	if err != nil {
		log.Printf("Error checking for transaction_date column: %v", err)
		hasTransactionDateColumn = false
	}

	// Check if user_id column exists
	var hasUserIdColumn bool
	err = database.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 
			FROM information_schema.columns 
			WHERE table_name = 'transactions' 
			AND column_name = 'user_id'
		)
	`).Scan(&hasUserIdColumn)
	if err != nil {
		log.Printf("Error checking for user_id column: %v", err)
		hasUserIdColumn = false
	}

	// Process categories from the relationship table
	var results []models.CategoryTotal
	processCategoryRelationships(w, r, request, userID, hasOptionalColumn, hasTransactionDateColumn, hasUserIdColumn, &results)

	// Return results
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// Handle categories from the transaction_categories relationship table
func processCategoryRelationships(w http.ResponseWriter, r *http.Request, request models.ReportFilter,
	userID string, hasOptionalColumn, hasTransactionDateColumn, hasUserIdColumn bool, results *[]models.CategoryTotal) {

	// Build the base query
	var query string
	query = `
		SELECT c.name as category, SUM(tc.amount) as total
		FROM transaction_categories tc
		JOIN ynab_categories c ON tc.category_id = c.id
		JOIN transactions t ON tc.transaction_id = t.id
		WHERE c.user_id = $1
	`
	var args []interface{}
	args = append(args, userID)

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
		query += fmt.Sprintf(` AND (
			t.pay_to ILIKE $%d OR 
			t.pay_to ILIKE $%d OR
			t.pay_to = $%d
		)`, len(args)+1, len(args)+2, len(args)+3)
		args = append(args, request.PayTo, "%"+request.PayTo+"%", request.PayTo)
	}

	// Add EnteredBy filter with proper SQL query structuring
	if request.EnteredBy != "" {
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
					matchedUserIds = append(matchedUserIds, id)
					matchedUserNames = append(matchedUserNames, name)
					if username != "" && username != name {
						matchedUserNames = append(matchedUserNames, username)
					}
				}
			}
		}

		// Build the entered_by filter
		enteredByConditions := make([]string, 0)
		for _, id := range matchedUserIds {
			enteredByConditions = append(enteredByConditions, fmt.Sprintf("t.entered_by = $%d", len(args)+1))
			args = append(args, id)
		}
		for _, name := range matchedUserNames {
			enteredByConditions = append(enteredByConditions, fmt.Sprintf("t.entered_by ILIKE $%d", len(args)+1))
			args = append(args, "%"+name+"%")
		}
		query += fmt.Sprintf(" AND (%s)", strings.Join(enteredByConditions, " OR "))
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
	if hasOptionalColumn {
		if request.Optional != nil {
			if *request.Optional {
				// When Optional is true, include ALL transactions (both optional and non-optional)
				query += " AND (t.optional = true OR t.optional = false OR t.optional IS NULL)"
			} else {
				// When Optional is false, only include non-optional transactions
				query += " AND (t.optional = false OR t.optional IS NULL)"
			}
		} else {
			// Default to false (don't filter on optional status)
			query += " AND (t.optional = false OR t.optional IS NULL)"
		}
	}

	// Add user access control if user_id column exists
	if hasUserIdColumn {
		accessibleUsers, err := middleware.GetUserAccessibleResources(userID, models.ResourceTransactions, models.PermissionRead)
		if err != nil {
			log.Printf("Error getting accessible resources: %v", err)
			// Fallback to only showing the user's own transactions
			query += " AND t.user_id = $1"
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
			}
		}
	}

	// Group by category and order by total descending
	query += " GROUP BY c.name ORDER BY total DESC"

	// Execute the query
	rows, err := database.DB.Query(query, args...)
	if err != nil {
		log.Printf("Error querying category relationships: %v", err)
		return
	}
	defer rows.Close()

	// Process results
	for rows.Next() {
		var category string
		var total float64
		if err := rows.Scan(&category, &total); err != nil {
			log.Printf("Error scanning category relationship row: %v", err)
			continue
		}
		*results = append(*results, models.CategoryTotal{
			Category: category,
			Total:    total,
		})
	}
}
