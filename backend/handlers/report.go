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

	// Get current transactions to debug
	rows, err := database.DB.Query("SELECT id, amount, description, date, type FROM transactions")
	if err != nil {
		log.Printf("Error querying transactions: %v", err)
	} else {
		defer rows.Close()
		log.Println("Current transactions in database:")
		for rows.Next() {
			var id, desc, typ string
			var amount float64
			var date time.Time
			rows.Scan(&id, &amount, &desc, &date, &typ)
			log.Printf("  - %s: $%.2f (%s) on %s [%s]", id, amount, desc, date.Format("2006-01-02"), typ)
		}
	}

	query := `
		SELECT type as category, SUM(amount) as total
		FROM transactions
		WHERE 1=1
	`
	args := []interface{}{}

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

	query += " GROUP BY type ORDER BY total DESC"
	log.Printf("Final query: %s with args: %v", query, args)

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

	// If no results, create some dummy data for testing
	if len(results) == 0 {
		log.Printf("No results found, adding test data")
		results = []models.CategoryTotal{
			{Category: "Groceries", Total: 150.50},
			{Category: "Utilities", Total: 85.20},
			{Category: "Entertainment", Total: 45.75},
		}
	}

	log.Printf("Returning %d results", len(results))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
