package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"bennwallet/backend/database"
	"bennwallet/backend/models"
)

func GetYNABSplits(w http.ResponseWriter, r *http.Request) {
	var request models.ReportFilter
	log.Println("YNAB Splits Report requested")

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("Error decoding YNAB filter: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Received request: %+v", request)

	// Print the userId to see if it's included
	log.Printf("UserId in request: %s", request.UserId)

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

	log.Printf("Table has optional column: %v", hasOptionalColumn)

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

	log.Printf("Table has transaction_date column: %v", hasTransactionDateColumn)

	// Debugging: List all transactions with their fields
	var query string
	if hasOptionalColumn {
		query = "SELECT id, amount, description, date, type, payTo, enteredBy, paid, optional FROM transactions"
	} else {
		query = "SELECT id, amount, description, date, type, payTo, enteredBy, paid FROM transactions"
	}

	rows, err := database.DB.Query(query)
	if err != nil {
		log.Printf("Error querying transactions: %v", err)
	} else {
		defer rows.Close()
		count := 0
		log.Println("Current transactions in database:")
		for rows.Next() {
			var id, desc, typ, payTo, enteredBy string
			var amount float64
			var date time.Time
			var paid bool
			var optional bool

			if hasOptionalColumn {
				if err := rows.Scan(&id, &amount, &desc, &date, &typ, &payTo, &enteredBy, &paid, &optional); err != nil {
					log.Printf("Error scanning row: %v", err)
					continue
				}
				log.Printf("  - %s: $%.2f (%s) on %s [%s] payTo=%s enteredBy=%s paid=%v optional=%v",
					id, amount, desc, date.Format("2006-01-02"), typ, payTo, enteredBy, paid, optional)
			} else {
				if err := rows.Scan(&id, &amount, &desc, &date, &typ, &payTo, &enteredBy, &paid); err != nil {
					log.Printf("Error scanning row: %v", err)
					continue
				}
				log.Printf("  - %s: $%.2f (%s) on %s [%s] payTo=%s enteredBy=%s paid=%v",
					id, amount, desc, date.Format("2006-01-02"), typ, payTo, enteredBy, paid)
			}
			count++
		}
		log.Printf("Total transactions found: %d", count)
	}

	// Build the base query
	query = `
		SELECT type as category, SUM(amount) as total
		FROM transactions
		WHERE 1=1
	`
	var args []interface{}

	// Add date filters
	if request.StartDate != "" {
		query += " AND date >= ?"
		args = append(args, request.StartDate)
		log.Printf("Added StartDate filter: %s", request.StartDate)
	}
	if request.EndDate != "" {
		query += " AND date <= ?"
		args = append(args, request.EndDate)
		log.Printf("Added EndDate filter: %s", request.EndDate)
	}

	// Add transaction date filters if column exists and filters are provided
	if hasTransactionDateColumn && request.TransactionDateMonth != nil && request.TransactionDateYear != nil {
		// Create start and end dates for the month
		startDate := fmt.Sprintf("%d-%02d-01", *request.TransactionDateYear, *request.TransactionDateMonth)
		endDate := fmt.Sprintf("%d-%02d-31", *request.TransactionDateYear, *request.TransactionDateMonth)

		query += " AND transaction_date >= ? AND transaction_date <= ?"
		args = append(args, startDate, endDate)
		log.Printf("Added TransactionDate filters: month=%d, year=%d", *request.TransactionDateMonth, *request.TransactionDateYear)
	}

	if request.Category != "" {
		query += " AND type = ?"
		args = append(args, request.Category)
		log.Printf("Added Category filter: %s", request.Category)
	}
	if request.PayTo != "" {
		query += " AND payTo LIKE ?"
		args = append(args, "%"+request.PayTo+"%")
		log.Printf("Added PayTo LIKE filter: '%s' (as %%%s%%)", request.PayTo, request.PayTo)

		// Debug: Check if any rows actually match this condition
		var matchCount int
		countQuery := "SELECT COUNT(*) FROM transactions WHERE payTo LIKE ?"
		err := database.DB.QueryRow(countQuery, "%"+request.PayTo+"%").Scan(&matchCount)
		if err != nil {
			log.Printf("Error checking PayTo match count: %v", err)
		} else {
			log.Printf("PayTo filter would match %d transactions", matchCount)
		}
	}
	if request.EnteredBy != "" {
		query += " AND enteredBy LIKE ?"
		args = append(args, "%"+request.EnteredBy+"%")
		log.Printf("Added EnteredBy LIKE filter: '%s' (as %%%s%%)", request.EnteredBy, request.EnteredBy)

		// Debug: Check if any rows actually match this condition
		var matchCount int
		countQuery := "SELECT COUNT(*) FROM transactions WHERE enteredBy LIKE ?"
		err := database.DB.QueryRow(countQuery, "%"+request.EnteredBy+"%").Scan(&matchCount)
		if err != nil {
			log.Printf("Error checking EnteredBy match count: %v", err)
		} else {
			log.Printf("EnteredBy filter would match %d transactions", matchCount)
		}
	}

	// Handle paid filter (default to true if not specified)
	if request.Paid == nil {
		// Default to true (only show paid transactions)
		paid := true
		request.Paid = &paid
	}

	query += " AND paid = ?"
	args = append(args, *request.Paid)
	log.Printf("Added Paid filter: %v", *request.Paid)

	// Only filter out optional transactions if the column exists and the filter requests it
	if hasOptionalColumn && (request.Optional == nil || *request.Optional == false) {
		query += " AND (optional = 0 OR optional IS NULL)"
		log.Printf("Added filter to exclude optional transactions")
	} else if hasOptionalColumn && *request.Optional == true {
		// Include optional transactions if specifically requested
		log.Printf("Including optional transactions as requested")
	} else {
		log.Printf("Skipping optional filter since column doesn't exist")
	}

	// We don't filter by userId since that column doesn't exist in the database

	query += " GROUP BY type ORDER BY total DESC"
	log.Printf("Final query: %s with args: %v", query, args)

	// Run the query with better error handling
	rows, err = database.DB.Query(query, args...)
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
