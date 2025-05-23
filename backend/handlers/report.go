package handlers

import (
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
		WHERE 1=1
	`
	var args []interface{}

	// Check if user is admin/superadmin
	var isAdmin bool
	err := database.DB.QueryRow(`
		SELECT role = 'admin' OR role = 'superadmin' FROM users WHERE id = $1
	`, userID).Scan(&isAdmin)
	if err != nil {
		log.Printf("Error checking admin status: %v", err)
		isAdmin = false
	}

	// Only apply user filter for non-admin users
	if !isAdmin {
		query += " AND (c.user_id = $1 OR c.user_id IS NULL)"
		args = append(args, userID)
	}

	// Add date filters
	if request.StartDate != "" {
		query += fmt.Sprintf(" AND t.date::date >= $%d::date", len(args)+1)
		args = append(args, request.StartDate)
	}
	if request.EndDate != "" {
		query += fmt.Sprintf(" AND t.date::date <= $%d::date", len(args)+1)
		args = append(args, request.EndDate)
	}

	// Add transaction date filters if column exists and filters are provided
	if hasTransactionDateColumn && request.TransactionDateMonth != nil && request.TransactionDateYear != nil {
		// Create start and end dates for the month
		startDate := fmt.Sprintf("%d-%02d-01", *request.TransactionDateYear, *request.TransactionDateMonth)
		endDate := fmt.Sprintf("%d-%02d-31", *request.TransactionDateYear, *request.TransactionDateMonth)

		query += fmt.Sprintf(" AND t.transaction_date::date >= $%d::date AND t.transaction_date::date <= $%d::date", len(args)+1, len(args)+2)
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
	if request.Paid != nil && *request.Paid {
		query += " AND t.paid = true"
		log.Printf("Paid filter: true")
	} else {
		log.Printf("Paid filter: not applied (value: %v)", request.Paid)
	}

	// Add optional filter if the column exists
	if hasOptionalColumn {
		log.Printf("Optional request value: %v", request.Optional)
		if request.Optional != nil {
			log.Printf("Optional pointer value: %v", *request.Optional)
		}

		if request.Optional != nil && !*request.Optional {
			query += " AND t.optional = false"
			log.Printf("Optional filter: excluding optional transactions (t.optional = false)")
		} else {
			log.Printf("Optional filter: including all transactions")
		}
	}

	// Add user access control if user_id column exists and user is not admin
	if hasUserIdColumn && !isAdmin {
		accessibleUsers, err := middleware.GetUserAccessibleResources(userID, models.ResourceTransactions, models.PermissionRead)
		if err != nil {
			log.Printf("Error getting accessible resources: %v", err)
		} else {
			// Build a query to include all accessible user transactions
			if len(accessibleUsers) > 0 {
				placeholders := make([]string, len(accessibleUsers))
				for i := range accessibleUsers {
					placeholders[i] = fmt.Sprintf("$%d", len(args)+1)
					args = append(args, accessibleUsers[i])
				}
				query += fmt.Sprintf(" AND (t.user_id IN (%s) OR t.user_id IS NULL)", strings.Join(placeholders, ","))
			}
		}
	}

	// Group by category and order by total descending
	query += " GROUP BY c.name ORDER BY total DESC"

	// Log the query and args for debugging
	log.Printf("Generated SQL Query: %s", query)
	log.Printf("Query Args: %v", args)
	log.Printf("User ID: %s", userID)
	log.Printf("Is Admin: %v", isAdmin)
	log.Printf("Start Date: %v", request.StartDate)
	log.Printf("End Date: %v", request.EndDate)
	log.Printf("Category: %v", request.Category)
	log.Printf("PayTo: %v", request.PayTo)
	log.Printf("EnteredBy: %v", request.EnteredBy)
	log.Printf("Paid: %v", request.Paid)
	log.Printf("Optional: %v", request.Optional)

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

	// Log the results
	log.Printf("Found %d categories with totals", len(*results))
	for _, result := range *results {
		log.Printf("Category: %s, Total: %.2f", result.Category, result.Total)
	}

	// If no results, let's check what data exists
	if len(*results) == 0 {
		// Check transactions table
		var txCount int
		err = database.DB.QueryRow("SELECT COUNT(*) FROM transactions").Scan(&txCount)
		if err != nil {
			log.Printf("Error counting transactions: %v", err)
		} else {
			log.Printf("Total transactions in database: %d", txCount)
		}

		// Let's check what transactions would match our filters
		debugQuery := `
			SELECT t.id, t.amount, t.description, t.date, t.paid, t.optional, c.name as category, t.user_id, t.entered_by
			FROM transactions t
			LEFT JOIN transaction_categories tc ON t.id = tc.transaction_id
			LEFT JOIN ynab_categories c ON tc.category_id = c.id
			WHERE 1=1
		`
		debugArgs := []interface{}{}
		argCount := 1

		if request.StartDate != "" {
			debugQuery += fmt.Sprintf(" AND t.date::date >= $%d::date", argCount)
			debugArgs = append(debugArgs, request.StartDate)
			argCount++
		}
		if request.EndDate != "" {
			debugQuery += fmt.Sprintf(" AND t.date::date <= $%d::date", argCount)
			debugArgs = append(debugArgs, request.EndDate)
			argCount++
		}
		if request.Paid != nil && *request.Paid {
			debugQuery += " AND t.paid = true"
		}
		if hasOptionalColumn && request.Optional != nil && *request.Optional {
			debugQuery += " AND t.optional = false"
		}

		log.Printf("Debug Query: %s", debugQuery)
		log.Printf("Debug Args: %v", debugArgs)

		debugRows, err := database.DB.Query(debugQuery, debugArgs...)
		if err != nil {
			log.Printf("Error in debug query: %v", err)
		} else {
			defer debugRows.Close()
			log.Printf("Transactions matching filters:")
			for debugRows.Next() {
				var id, description, category, userId, enteredBy string
				var amount float64
				var date time.Time
				var paid, optional bool
				if err := debugRows.Scan(&id, &amount, &description, &date, &paid, &optional, &category, &userId, &enteredBy); err == nil {
					log.Printf("  ID: %s, Amount: %.2f, Description: %s, Date: %s, Paid: %v, Optional: %v, Category: %s, UserID: %s, EnteredBy: %s",
						id, amount, description, date.Format("2006-01-02"), paid, optional, category, userId, enteredBy)
				}
			}
		}
	}
}
