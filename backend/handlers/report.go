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

	// Check if the optional column exists
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

	// Build the base query
	var query string
	query = `
		SELECT type as category, SUM(amount) as total
		FROM transactions
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
			query += " AND userId = ?"
			args = append(args, userID)
		} else {
			// Build a query to include all accessible user transactions
			if len(accessibleUsers) > 0 {
				placeholders := make([]string, len(accessibleUsers))
				for i := range accessibleUsers {
					placeholders[i] = "?"
					args = append(args, accessibleUsers[i])
				}
				query += fmt.Sprintf(" AND (userId IN (%s) OR userId IS NULL)", strings.Join(placeholders, ","))
			} else {
				// Fallback to only showing the user's own transactions
				query += " AND userId = ?"
				args = append(args, userID)
			}
		}
	}

	// Add date filters
	if request.StartDate != "" {
		query += " AND date >= ?"
		args = append(args, request.StartDate)
	}
	if request.EndDate != "" {
		query += " AND date <= ?"
		args = append(args, request.EndDate)
	}

	// Add transaction date filters if column exists and filters are provided
	if hasTransactionDateColumn && request.TransactionDateMonth != nil && request.TransactionDateYear != nil {
		// Create start and end dates for the month
		startDate := fmt.Sprintf("%d-%02d-01", *request.TransactionDateYear, *request.TransactionDateMonth)
		endDate := fmt.Sprintf("%d-%02d-31", *request.TransactionDateYear, *request.TransactionDateMonth)

		query += " AND transaction_date >= ? AND transaction_date <= ?"
		args = append(args, startDate, endDate)
	}

	// Add category filter
	if request.Category != "" {
		query += " AND type = ?"
		args = append(args, request.Category)
	}

	// Add PayTo filter with proper SQL query structuring
	if request.PayTo != "" {
		query += " AND payTo LIKE ?"
		args = append(args, "%"+request.PayTo+"%")
	}

	// Add EnteredBy filter with proper SQL query structuring
	if request.EnteredBy != "" {
		query += " AND enteredBy LIKE ?"
		args = append(args, "%"+request.EnteredBy+"%")
	}

	// Add paid filter
	if request.Paid != nil {
		query += " AND paid = ?"
		args = append(args, *request.Paid)
	} else {
		// Default to true (only show paid transactions)
		query += " AND paid = 1"
	}

	// Add optional filter if the column exists
	if hasOptionalColumn && (request.Optional == nil || *request.Optional == false) {
		query += " AND (optional = 0 OR optional IS NULL)"
	}

	// Add grouping and ordering
	query += " GROUP BY type ORDER BY total DESC"
	log.Printf("Executing query: %s with args: %v", query, args)

	// Run the query
	rows, err := database.DB.Query(query, args...)
	if err != nil {
		log.Printf("Error executing query: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var results []models.CategoryTotal
	for rows.Next() {
		var ct models.CategoryTotal
		err := rows.Scan(&ct.Category, &ct.Total)
		if err != nil {
			log.Printf("Error scanning result: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		results = append(results, ct)
	}

	// Check for any errors from iterating over rows
	if err = rows.Err(); err != nil {
		log.Printf("Error after scanning all rows: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Returning %d results", len(results))

	// Always return an array, even if empty
	if results == nil {
		results = []models.CategoryTotal{}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(results); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}
