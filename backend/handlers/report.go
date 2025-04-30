package handlers

import (
	"encoding/json"
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

	// Get current transactions to debug
	rows, err := database.DB.Query("SELECT id, amount, description, date, type, payTo, enteredBy FROM transactions")
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
			if err := rows.Scan(&id, &amount, &desc, &date, &typ, &payTo, &enteredBy); err != nil {
				log.Printf("Error scanning row: %v", err)
				continue
			}
			log.Printf("  - %s: $%.2f (%s) on %s [%s] payTo=%s enteredBy=%s",
				id, amount, desc, date.Format("2006-01-02"), typ, payTo, enteredBy)
			count++
		}
		log.Printf("Total transactions found: %d", count)
	}

	query := `
		SELECT type as category, SUM(amount) as total
		FROM transactions
		WHERE 1=1
	`
	args := []interface{}{}

	log.Printf("Starting to build query with filters. Initial query: %s", query)

	if request.StartDate != "" {
		// Use SQLite's date() function to extract just the date part for comparison
		query += " AND date(date) >= date(?)"
		args = append(args, request.StartDate)
		log.Printf("Added StartDate filter: %s", request.StartDate)
	}
	if request.EndDate != "" {
		// Use SQLite's date() function to extract just the date part for comparison
		query += " AND date(date) <= date(?)"
		args = append(args, request.EndDate)
		log.Printf("Added EndDate filter: %s", request.EndDate)
	}
	if request.Category != "" {
		query += " AND type = ?"
		args = append(args, request.Category)
		log.Printf("Added Category filter: %s", request.Category)
	}
	if request.PayTo != "" {
		query += " AND payTo = ?"
		args = append(args, request.PayTo)
		log.Printf("Added PayTo filter: %s", request.PayTo)
	}
	if request.EnteredBy != "" {
		query += " AND enteredBy = ?"
		args = append(args, request.EnteredBy)
		log.Printf("Added EnteredBy filter: %s", request.EnteredBy)
	}
	if request.Paid != nil {
		query += " AND paid = ?"
		args = append(args, *request.Paid)
		log.Printf("Added Paid filter: %v", *request.Paid)
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
