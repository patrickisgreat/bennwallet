package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"time"

	"bennwallet/backend/database"
	"bennwallet/backend/models"

	"github.com/gorilla/mux"
)

func GetTransactions(w http.ResponseWriter, r *http.Request) {
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

	// Base query with the appropriate columns
	var query string
	if hasOptionalColumn {
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

	// Parse query parameters
	payTo := r.URL.Query().Get("payTo")
	if payTo != "" {
		query += " AND payTo = ?"
		args = append(args, payTo)
	}

	enteredBy := r.URL.Query().Get("enteredBy")
	if enteredBy != "" {
		query += " AND enteredBy = ?"
		args = append(args, enteredBy)
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

		var err error
		if hasOptionalColumn {
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

	var t models.Transaction
	var paidDate sql.NullString
	var transactionDate sql.NullTime

	var query string
	if hasOptionalColumn {
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

	var err2 error
	if hasOptionalColumn {
		err2 = database.DB.QueryRow(query, id).Scan(&t.ID, &t.Amount, &t.Description, &t.Date, &transactionDate, &t.Type, &t.PayTo, &t.Paid, &paidDate, &t.EnteredBy, &t.Optional)
	} else {
		err2 = database.DB.QueryRow(query, id).Scan(&t.ID, &t.Amount, &t.Description, &t.Date, &transactionDate, &t.Type, &t.PayTo, &t.Paid, &paidDate, &t.EnteredBy)
		// Set default value for optional
		t.Optional = false
	}

	if err2 != nil {
		if err2 == sql.ErrNoRows {
			http.Error(w, "Transaction not found", http.StatusNotFound)
		} else {
			http.Error(w, err2.Error(), http.StatusInternalServerError)
		}
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

func AddTransaction(w http.ResponseWriter, r *http.Request) {
	var t models.Transaction
	err := json.NewDecoder(r.Body).Decode(&t)
	if err != nil {
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

	_, err = database.DB.Exec(`
		INSERT INTO transactions (id, amount, description, date, transaction_date, type, payTo, paid, paidDate, enteredBy, optional)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, t.ID, t.Amount, t.Description, t.Date, t.TransactionDate, t.Type, t.PayTo, t.Paid, t.PaidDate, t.EnteredBy, t.Optional)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

func UpdateTransaction(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var t models.Transaction
	err := json.NewDecoder(r.Body).Decode(&t)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err = database.DB.Exec(`
		UPDATE transactions 
		SET amount = ?, description = ?, date = ?, transaction_date = ?, type = ?, payTo = ?, paid = ?, paidDate = ?, enteredBy = ?, optional = ?
		WHERE id = ?
	`, t.Amount, t.Description, t.Date, t.TransactionDate, t.Type, t.PayTo, t.Paid, t.PaidDate, t.EnteredBy, t.Optional, id)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func DeleteTransaction(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	_, err := database.DB.Exec("DELETE FROM transactions WHERE id = ?", id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
