package handlers

import (
	"bennwallet/backend/middleware"
	"bennwallet/backend/models"
	"bennwallet/backend/services"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

type SettlementHandler struct {
	settlementService *services.SettlementService
	db                *sql.DB
}

func NewSettlementHandler(db *sql.DB) *SettlementHandler {
	return &SettlementHandler{
		settlementService: services.NewSettlementService(db),
		db:                db,
	}
}

// CreateSettlement creates a new settlement for a transaction
func (h *SettlementHandler) CreateSettlement(w http.ResponseWriter, r *http.Request) {
	userIDValue := r.Context().Value(middleware.UserIDKey)
	if userIDValue == nil {
		http.Error(w, "No user ID in context", http.StatusUnauthorized)
		return
	}
	userID := userIDValue.(string)

	var req struct {
		TransactionID string `json:"transactionId"`
		Notes         string `json:"notes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	settlement, err := h.settlementService.CreateSettlement(userID, req.TransactionID, req.Notes)
	if err != nil {
		log.Printf("Error creating settlement: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settlement)
}

// ApplyOthersTransactionToDebt allows a user to apply someone else's transaction to their debt
func (h *SettlementHandler) ApplyOthersTransactionToDebt(w http.ResponseWriter, r *http.Request) {
	userIDValue := r.Context().Value(middleware.UserIDKey)
	if userIDValue == nil {
		http.Error(w, "No user ID in context", http.StatusUnauthorized)
		return
	}
	userID := userIDValue.(string)
	log.Printf("ApplyOthersTransactionToDebt: userID = %s", userID)

	var req struct {
		TransactionID string `json:"transactionId"`
		Notes         string `json:"notes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get the transaction to verify it exists and the user can apply it
	var tx models.Transaction
	var owedBy sql.NullString
	err := h.db.QueryRow(`
		SELECT id, amount, entered_by, paid_by, owed_by 
		FROM transactions 
		WHERE id = $1 AND paid = false
	`, req.TransactionID).Scan(&tx.ID, &tx.Amount, &tx.EnteredBy, &tx.PaidBy, &owedBy)

	if err != nil {
		log.Printf("Error fetching transaction %s: %v", req.TransactionID, err)
		if err == sql.ErrNoRows {
			http.Error(w, "Transaction not found or already paid", http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("Database error: %v", err), http.StatusInternalServerError)
		}
		return
	}

	// Set OwedBy if it was in the database
	if owedBy.Valid {
		tx.OwedBy = owedBy.String
	}

	// Verify this transaction was entered by someone else
	if tx.EnteredBy == userID {
		http.Error(w, "You cannot apply your own transactions to your debt", http.StatusBadRequest)
		return
	}

	// Find existing active settlement between these users
	var settlementID string
	err = h.db.QueryRow(`
		SELECT id FROM settlements 
		WHERE created_by = $1 
		AND created_for = $2 
		AND status = 'active'
		ORDER BY created_at DESC
		LIMIT 1
	`, tx.EnteredBy, userID).Scan(&settlementID)

	if err == sql.ErrNoRows {
		// No active settlement exists, create one
		settlement, err := h.settlementService.ApplyTransactionAsPayment(userID, req.TransactionID, req.Notes)
		if err != nil {
			log.Printf("Error creating settlement from payment: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(settlement)
		return
	}

	// Apply to existing settlement
	err = h.settlementService.ApplyTransactionToSettlement(settlementID, req.TransactionID, tx.Amount, userID)
	if err != nil {
		log.Printf("Error applying transaction to settlement: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Return the updated settlement
	settlement, err := h.settlementService.GetSettlement(settlementID)
	if err != nil {
		http.Error(w, "Failed to get updated settlement", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settlement)
}

// ApplyTransaction applies a transaction to a settlement
func (h *SettlementHandler) ApplyTransaction(w http.ResponseWriter, r *http.Request) {
	userIDValue := r.Context().Value(middleware.UserIDKey)
	if userIDValue == nil {
		http.Error(w, "No user ID in context", http.StatusUnauthorized)
		return
	}
	userID := userIDValue.(string)
	settlementID := mux.Vars(r)["id"]

	var req struct {
		TransactionID string  `json:"transactionId"`
		Amount        float64 `json:"amount"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := h.settlementService.ApplyTransactionToSettlement(settlementID, req.TransactionID, req.Amount, userID)
	if err != nil {
		log.Printf("Error applying transaction to settlement: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Return the updated settlement
	settlement, err := h.settlementService.GetSettlement(settlementID)
	if err != nil {
		http.Error(w, "Failed to get updated settlement", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settlement)
}

// RemoveTransaction removes a transaction from a settlement
func (h *SettlementHandler) RemoveTransaction(w http.ResponseWriter, r *http.Request) {
	userIDValue := r.Context().Value(middleware.UserIDKey)
	if userIDValue == nil {
		http.Error(w, "No user ID in context", http.StatusUnauthorized)
		return
	}
	userID := userIDValue.(string)
	settlementID := mux.Vars(r)["id"]
	transactionID := mux.Vars(r)["transactionId"]

	err := h.settlementService.RemoveTransactionFromSettlement(settlementID, transactionID, userID)
	if err != nil {
		log.Printf("Error removing transaction from settlement: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Return the updated settlement
	settlement, err := h.settlementService.GetSettlement(settlementID)
	if err != nil {
		http.Error(w, "Failed to get updated settlement", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settlement)
}

// GetSettlement retrieves a specific settlement
func (h *SettlementHandler) GetSettlement(w http.ResponseWriter, r *http.Request) {
	settlementID := mux.Vars(r)["id"]

	settlement, err := h.settlementService.GetSettlement(settlementID)
	if err != nil {
		log.Printf("Error getting settlement: %v", err)
		http.Error(w, "Settlement not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settlement)
}

// GetUserSettlements retrieves all settlements for the current user
func (h *SettlementHandler) GetUserSettlements(w http.ResponseWriter, r *http.Request) {
	userIDValue := r.Context().Value(middleware.UserIDKey)
	if userIDValue == nil {
		http.Error(w, "No user ID in context", http.StatusUnauthorized)
		return
	}
	userID := userIDValue.(string)
	status := r.URL.Query().Get("status") // optional filter by status

	settlements, err := h.settlementService.GetUserSettlements(userID, status)
	if err != nil {
		log.Printf("Error getting user settlements: %v", err)
		http.Error(w, "Failed to get settlements", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settlements)
}

// GetTransactionSettlements retrieves all settlements that include a specific transaction
func (h *SettlementHandler) GetTransactionSettlements(w http.ResponseWriter, r *http.Request) {
	transactionID := mux.Vars(r)["transactionId"]

	query := `
		SELECT DISTINCT s.id, s.created_by, s.created_for, s.total_amount, 
		       s.remaining_amount, s.status, s.created_at
		FROM settlements s
		JOIN settlement_items si ON s.id = si.settlement_id
		WHERE si.transaction_id = $1
		ORDER BY s.created_at DESC
	`

	rows, err := h.db.Query(query, transactionID)
	if err != nil {
		log.Printf("Error getting transaction settlements: %v", err)
		http.Error(w, "Failed to get settlements", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var settlements []models.Settlement
	for rows.Next() {
		var s models.Settlement
		err := rows.Scan(&s.ID, &s.CreatorID, &s.RecipientID, &s.TotalAmount,
			&s.RemainingAmount, &s.Status, &s.CreatedAt)
		if err != nil {
			continue
		}
		settlements = append(settlements, s)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settlements)
}

// GetAvailableSettlementTransactions retrieves transactions available for applying to a settlement
func (h *SettlementHandler) GetAvailableSettlementTransactions(w http.ResponseWriter, r *http.Request) {
	settlementID := mux.Vars(r)["id"]
	userIDValue := r.Context().Value(middleware.UserIDKey)
	if userIDValue == nil {
		http.Error(w, "No user ID in context", http.StatusUnauthorized)
		return
	}
	userID := userIDValue.(string)

	// First get the settlement to know who created it
	settlement, err := h.settlementService.GetSettlement(settlementID)
	if err != nil {
		log.Printf("Error getting settlement: %v", err)
		http.Error(w, "Settlement not found", http.StatusNotFound)
		return
	}

	// Get unpaid transactions between the two users that can be used for offsetting
	var otherUserID string
	if userID == settlement.CreatorID {
		// Current user created the settlement, so the other person is RecipientID
		otherUserID = settlement.RecipientID
	} else {
		// Current user is the one who owes, so the other person is CreatorID
		otherUserID = settlement.CreatorID
	}

	log.Printf("GetAvailableTransactionsForSettlement: userID=%s, otherUserID=%s, settlementID=%s", userID, otherUserID, settlementID)

	// Find ALL unpaid transactions between these two users
	// This allows bidirectional offsetting
	query := `
		SELECT t.id, t.amount, t.description, t.date, t.transaction_date, 
		       t.type, t.paid_by, t.owed_by, t.paid, t.paid_date, t.entered_by, 
		       t.optional, t.note, t.user_id
		FROM transactions t
		WHERE t.paid = false
		  AND (
		    -- Transactions where current user paid and other user owes
		    (t.paid_by = $1 AND t.owed_by = $2)
		    OR
		    -- Transactions where other user paid and current user owes  
		    (t.paid_by = $2 AND t.owed_by = $1)
		  )
		  AND t.id NOT IN (
		    SELECT transaction_id FROM settlement_items WHERE settlement_id = $3
		  )
		ORDER BY t.transaction_date DESC
	`

	rows, err := h.db.Query(query, userID, otherUserID, settlementID)
	if err != nil {
		log.Printf("Error getting available transactions: %v", err)
		http.Error(w, "Failed to get transactions", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var transactions []models.Transaction
	for rows.Next() {
		var t models.Transaction
		var paidDate, transactionDate, userID, note, owedBy sql.NullString
		var dateStr string

		err := rows.Scan(&t.ID, &t.Amount, &t.Description, &dateStr, &transactionDate,
			&t.Type, &t.PaidBy, &owedBy, &t.Paid, &paidDate, &t.EnteredBy,
			&t.Optional, &note, &userID)
		if err != nil {
			log.Printf("Error scanning transaction: %v", err)
			continue
		}

		// Parse dates
		t.Date, _ = time.Parse("2006-01-02", dateStr)
		if transactionDate.Valid {
			t.TransactionDate, _ = time.Parse("2006-01-02", transactionDate.String)
		} else {
			t.TransactionDate = t.Date
		}
		if paidDate.Valid {
			t.PaidDate = paidDate.String
		}
		if note.Valid {
			t.Note = note.String
		}
		if userID.Valid {
			t.UserID = userID.String
		}
		if owedBy.Valid {
			t.OwedBy = owedBy.String
		}

		transactions = append(transactions, t)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(transactions)
}

// UpdateSettlementStatus updates the status of a settlement (for cancelling, completing, etc.)
func (h *SettlementHandler) UpdateSettlementStatus(w http.ResponseWriter, r *http.Request) {
	settlementID := mux.Vars(r)["id"]
	userIDValue := r.Context().Value(middleware.UserIDKey)
	if userIDValue == nil {
		http.Error(w, "No user ID in context", http.StatusUnauthorized)
		return
	}
	userID := userIDValue.(string)

	// Parse request body
	var req struct {
		Status string `json:"status"`
		Notes  string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate status
	validStatuses := map[string]bool{
		"active":    true,
		"completed": true,
		"cancelled": true,
	}
	if !validStatuses[req.Status] {
		http.Error(w, "Invalid status. Must be 'active', 'completed', or 'cancelled'", http.StatusBadRequest)
		return
	}

	// Get the settlement to check permissions
	settlement, err := h.settlementService.GetSettlement(settlementID)
	if err != nil {
		log.Printf("Error getting settlement: %v", err)
		http.Error(w, "Settlement not found", http.StatusNotFound)
		return
	}

	// Check if user has permission to update this settlement
	// Only the creator or the person it was created for can update it
	if userID != settlement.CreatorID && userID != settlement.RecipientID {
		http.Error(w, "You don't have permission to update this settlement", http.StatusForbidden)
		return
	}

	// Use the settlement service to update status (this handles adjustment transactions)
	updatedSettlement, err := h.settlementService.UpdateSettlementStatus(settlementID, req.Status, userID, req.Notes)
	if err != nil {
		log.Printf("Error updating settlement status: %v", err)
		http.Error(w, "Failed to update settlement status", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedSettlement)
}

// GetSettlementsGroupedByMonth returns settlements grouped by month
func (h *SettlementHandler) GetSettlementsGroupedByMonth(w http.ResponseWriter, r *http.Request) {
	userIDValue := r.Context().Value(middleware.UserIDKey)
	if userIDValue == nil {
		http.Error(w, "No user ID in context", http.StatusUnauthorized)
		return
	}
	userID := userIDValue.(string)
	status := r.URL.Query().Get("status") // optional filter by status

	monthlyGroups, err := h.settlementService.GetSettlementsGroupedByMonth(userID, status)
	if err != nil {
		log.Printf("Error getting settlements grouped by month: %v", err)
		http.Error(w, "Failed to get settlements", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(monthlyGroups)
}

// GetSettlementReportData returns settlement report data for a specific month
func (h *SettlementHandler) GetSettlementReportData(w http.ResponseWriter, r *http.Request) {
	userIDValue := r.Context().Value(middleware.UserIDKey)
	if userIDValue == nil {
		http.Error(w, "No user ID in context", http.StatusUnauthorized)
		return
	}
	userID := userIDValue.(string)

	month := r.URL.Query().Get("month") // YYYY-MM format
	if month == "" {
		http.Error(w, "Month parameter is required (YYYY-MM format)", http.StatusBadRequest)
		return
	}

	showPaidParam := r.URL.Query().Get("paid")
	showPaid := showPaidParam == "true"

	report, err := h.settlementService.GetSettlementReportData(userID, month, showPaid)
	if err != nil {
		log.Printf("Error getting settlement report data: %v", err)
		http.Error(w, fmt.Sprintf("Failed to get settlement report: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}
