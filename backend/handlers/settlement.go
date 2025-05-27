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
	err := h.db.QueryRow(`
		SELECT id, amount, entered_by, pay_to 
		FROM transactions 
		WHERE id = $1 AND paid = false
	`, req.TransactionID).Scan(&tx.ID, &tx.Amount, &tx.EnteredBy, &tx.PayTo)

	if err != nil {
		log.Printf("Error fetching transaction %s: %v", req.TransactionID, err)
		if err == sql.ErrNoRows {
			http.Error(w, "Transaction not found or already paid", http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("Database error: %v", err), http.StatusInternalServerError)
		}
		return
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
		err := rows.Scan(&s.ID, &s.CreatedBy, &s.CreatedFor, &s.TotalAmount, 
			&s.RemainingAmount, &s.Status, &s.CreatedAt)
		if err != nil {
			continue
		}
		settlements = append(settlements, s)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settlements)
}