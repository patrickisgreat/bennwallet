package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"bennwallet/backend/database"
	"bennwallet/backend/middleware"
)

// CheckDatabaseHandler provides information about database tables and counts
func CheckDatabaseHandler(w http.ResponseWriter, r *http.Request) {
	// Get the user ID from the authentication context
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	// Check if user is admin
	var isAdmin bool
	err := database.DB.QueryRow("SELECT is_admin FROM users WHERE id = $1", userID).Scan(&isAdmin)
	if err != nil {
		log.Printf("Error checking if user is admin: %v", err)
		isAdmin = false // Default to not admin
	}

	// If not admin, check if there's a 'role' column and check for admin/superadmin role
	if !isAdmin {
		var hasRoleColumn bool
		err = database.DB.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM information_schema.columns 
				WHERE table_name = 'users' AND column_name = 'role'
			)
		`).Scan(&hasRoleColumn)

		if err == nil && hasRoleColumn {
			var role sql.NullString
			err = database.DB.QueryRow("SELECT role FROM users WHERE id = $1", userID).Scan(&role)
			if err == nil && role.Valid {
				isAdmin = role.String == "admin" || role.String == "superadmin"
			}
		}
	}

	// If user is still not admin, reject the request
	if !isAdmin {
		http.Error(w, "Unauthorized: Admin access required", http.StatusForbidden)
		return
	}

	// Define tables to check
	tables := []string{
		"users",
		"transactions",
		"categories",
		"transaction_categories",
		"permissions",
	}

	// Get counts for each table
	tableCounts := make(map[string]int)
	errors := []string{}

	for _, table := range tables {
		// First check if the table exists
		var tableExists bool
		err := database.DB.QueryRow(fmt.Sprintf(`
			SELECT EXISTS (
				SELECT FROM information_schema.tables 
				WHERE table_name = '%s'
			)
		`, table)).Scan(&tableExists)

		if err != nil {
			log.Printf("Error checking if %s table exists: %v", table, err)
			errors = append(errors, fmt.Sprintf("Error checking if %s table exists: %v", table, err))
			continue
		}

		if !tableExists {
			tableCounts[table] = -1 // -1 indicates the table doesn't exist
			continue
		}

		// Count rows in the table
		var count int
		err = database.DB.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
		if err != nil {
			log.Printf("Error counting rows in %s: %v", table, err)
			errors = append(errors, fmt.Sprintf("Error counting rows in %s: %v", table, err))
			continue
		}

		tableCounts[table] = count
	}

	// Additional diagnostic information
	// Check if required columns exist
	var hasUserIdColumn bool
	err = database.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'transactions' AND column_name = 'user_id'
		)
	`).Scan(&hasUserIdColumn)

	if err != nil {
		log.Printf("Error checking for user_id column: %v", err)
		errors = append(errors, fmt.Sprintf("Error checking for user_id column: %v", err))
	} else if !hasUserIdColumn {
		errors = append(errors, "transactions table is missing user_id column")
	}

	// Check postgres version
	var version string
	err = database.DB.QueryRow("SELECT version()").Scan(&version)
	if err != nil {
		log.Printf("Error getting postgres version: %v", err)
		errors = append(errors, fmt.Sprintf("Error getting postgres version: %v", err))
	}

	// Return the information
	response := struct {
		Status  string         `json:"status"`
		Tables  map[string]int `json:"tables"`
		Version string         `json:"version,omitempty"`
		Errors  []string       `json:"errors,omitempty"`
	}{
		Status: func() string {
			if len(errors) > 0 {
				return "warning"
			}
			return "ok"
		}(),
		Tables:  tableCounts,
		Version: version,
		Errors:  errors,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CheckTransactionCategories returns diagnostics about transaction categories
func CheckTransactionCategories(w http.ResponseWriter, r *http.Request) {
	// Get user ID from authentication context
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	// Check if user is an admin
	var isAdmin bool
	err := database.DB.QueryRow("SELECT role IN ('admin', 'superadmin') FROM users WHERE id = $1", userID).Scan(&isAdmin)
	if err != nil || !isAdmin {
		http.Error(w, "Unauthorized: Admin access required", http.StatusForbidden)
		return
	}

	// Get transaction count
	var transactionCount int
	err = database.DB.QueryRow("SELECT COUNT(*) FROM transactions").Scan(&transactionCount)
	if err != nil {
		http.Error(w, "Error counting transactions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get transaction_categories count
	var categoryAssociationCount int
	err = database.DB.QueryRow("SELECT COUNT(*) FROM transaction_categories").Scan(&categoryAssociationCount)
	if err != nil {
		http.Error(w, "Error counting transaction_categories: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get transactions with categories
	var transactionsWithCategories int
	err = database.DB.QueryRow(`
		SELECT COUNT(DISTINCT transaction_id) 
		FROM transaction_categories
	`).Scan(&transactionsWithCategories)
	if err != nil {
		http.Error(w, "Error counting transactions with categories: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get YNAB categories count
	var ynabCategoriesCount int
	err = database.DB.QueryRow("SELECT COUNT(*) FROM ynab_categories").Scan(&ynabCategoriesCount)
	if err != nil {
		http.Error(w, "Error counting YNAB categories: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Sample of 5 transactions with their category associations
	rows, err := database.DB.Query(`
		SELECT t.id, t.description, t.user_id, 
		       (SELECT COUNT(*) FROM transaction_categories tc WHERE tc.transaction_id = t.id) as category_count
		FROM transactions t
		ORDER BY t.id
		LIMIT 5
	`)
	if err != nil {
		http.Error(w, "Error querying transaction samples: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type TransactionSample struct {
		ID            string `json:"id"`
		Description   string `json:"description"`
		UserID        string `json:"user_id"`
		CategoryCount int    `json:"category_count"`
	}

	var samples []TransactionSample
	for rows.Next() {
		var sample TransactionSample
		err := rows.Scan(&sample.ID, &sample.Description, &sample.UserID, &sample.CategoryCount)
		if err != nil {
			log.Printf("Error scanning transaction sample: %v", err)
			continue
		}
		samples = append(samples, sample)
	}

	// Build response
	response := struct {
		TotalTransactions          int                 `json:"total_transactions"`
		TotalCategoryAssociations  int                 `json:"total_category_associations"`
		TransactionsWithCategories int                 `json:"transactions_with_categories"`
		TotalYNABCategories        int                 `json:"total_ynab_categories"`
		TransactionSamples         []TransactionSample `json:"transaction_samples"`
	}{
		TotalTransactions:          transactionCount,
		TotalCategoryAssociations:  categoryAssociationCount,
		TransactionsWithCategories: transactionsWithCategories,
		TotalYNABCategories:        ynabCategoriesCount,
		TransactionSamples:         samples,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
