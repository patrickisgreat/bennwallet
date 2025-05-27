package services

import (
	"bennwallet/backend/models"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type SettlementService struct {
	db *sql.DB
}

func NewSettlementService(db *sql.DB) *SettlementService {
	return &SettlementService{db: db}
}

// CreateSettlement creates a new settlement for applying transactions against debt
// This is used when someone wants to apply their transactions to offset debt
func (s *SettlementService) CreateSettlement(userID string, transactionID string, notes string) (*models.Settlement, error) {
	// First, get the transaction to determine the amount and who it's for
	var transaction models.Transaction
	err := s.db.QueryRow(`
		SELECT id, amount, pay_to, entered_by, user_id 
		FROM transactions 
		WHERE id = $1
	`, transactionID).Scan(&transaction.ID, &transaction.Amount, &transaction.PayTo, &transaction.EnteredBy, &transaction.UserID)

	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	// The settlement is created BY the current user
	// If they entered this transaction, they're saying someone else can use it to pay them back
	// The "created_for" should be who owes them money
	var createdBy, createdFor string
	createdBy = userID
	
	// For a settlement to make sense:
	// - Current user must have entered the transaction (they are owed money)
	// - The transaction must have a PayTo indicating who owes them
	if transaction.EnteredBy != userID {
		return nil, fmt.Errorf("you can only create settlements for transactions you entered")
	}

	// Find the user who should pay (from PayTo field)
	var payToUserID string
	err = s.db.QueryRow(`
		SELECT id FROM users WHERE name = $1 OR username = $1 LIMIT 1
	`, transaction.PayTo).Scan(&payToUserID)
	
	if err != nil {
		// If we can't find the user, we can't create a settlement
		return nil, fmt.Errorf("cannot find user '%s' to create settlement", transaction.PayTo)
	}
	createdFor = payToUserID

	settlementID := uuid.New().String()
	now := time.Now()

	// Start a transaction
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Create the settlement
	_, err = tx.Exec(`
		INSERT INTO settlements (id, created_by, created_for, total_amount, remaining_amount, status, notes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, settlementID, createdBy, createdFor, transaction.Amount, transaction.Amount, "active", notes, now, now)

	if err != nil {
		return nil, fmt.Errorf("failed to create settlement: %w", err)
	}

	// Add the initial transaction as a settlement item
	_, err = tx.Exec(`
		INSERT INTO settlement_items (settlement_id, transaction_id, applied_amount, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, settlementID, transactionID, transaction.Amount, userID, now)

	if err != nil {
		return nil, fmt.Errorf("failed to create settlement item: %w", err)
	}

	// Create history entry
	details := models.SettlementDetails{
		"transaction_id": transactionID,
		"amount":         transaction.Amount,
	}

	if err := s.addHistoryEntry(tx, settlementID, "created", userID, &transactionID, &transaction.Amount, details); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s.GetSettlement(settlementID)
}

// ApplyTransactionToSettlement applies a transaction to offset a settlement
func (s *SettlementService) ApplyTransactionToSettlement(settlementID string, transactionID string, amount float64, userID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get the settlement
	var settlement models.Settlement
	err = tx.QueryRow(`
		SELECT id, remaining_amount, status, created_by, created_for
		FROM settlements
		WHERE id = $1
	`, settlementID).Scan(&settlement.ID, &settlement.RemainingAmount, &settlement.Status, &settlement.CreatedBy, &settlement.CreatedFor)

	if err != nil {
		return fmt.Errorf("failed to get settlement: %w", err)
	}

	// Validate the settlement is active
	if settlement.Status != "active" {
		return fmt.Errorf("settlement is not active")
	}

	// Validate the user has permission (either creator or target)
	if userID != settlement.CreatedBy && userID != settlement.CreatedFor {
		return fmt.Errorf("user does not have permission to modify this settlement")
	}

	// Validate amount doesn't exceed remaining
	if amount > settlement.RemainingAmount {
		return fmt.Errorf("amount exceeds remaining settlement amount")
	}

	// Add the settlement item
	_, err = tx.Exec(`
		INSERT INTO settlement_items (settlement_id, transaction_id, applied_amount, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, settlementID, transactionID, amount, userID, time.Now())

	if err != nil {
		return fmt.Errorf("failed to create settlement item: %w", err)
	}

	// Update remaining amount
	newRemaining := settlement.RemainingAmount - amount
	status := settlement.Status
	var completedAt *time.Time

	if newRemaining <= 0 {
		status = "completed"
		now := time.Now()
		completedAt = &now
	}

	_, err = tx.Exec(`
		UPDATE settlements 
		SET remaining_amount = $1, status = $2, completed_at = $3, updated_at = $4
		WHERE id = $5
	`, newRemaining, status, completedAt, time.Now(), settlementID)

	if err != nil {
		return fmt.Errorf("failed to update settlement: %w", err)
	}

	// Add history entry
	details := models.SettlementDetails{
		"remaining_before": settlement.RemainingAmount,
		"remaining_after":  newRemaining,
	}

	if err := s.addHistoryEntry(tx, settlementID, "transaction_applied", userID, &transactionID, &amount, details); err != nil {
		return err
	}

	if status == "completed" {
		if err := s.addHistoryEntry(tx, settlementID, "completed", userID, nil, nil, nil); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// RemoveTransactionFromSettlement removes a transaction from a settlement
func (s *SettlementService) RemoveTransactionFromSettlement(settlementID string, transactionID string, userID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get the settlement item
	var appliedAmount float64
	err = tx.QueryRow(`
		SELECT applied_amount FROM settlement_items
		WHERE settlement_id = $1 AND transaction_id = $2
	`, settlementID, transactionID).Scan(&appliedAmount)

	if err != nil {
		return fmt.Errorf("settlement item not found: %w", err)
	}

	// Delete the settlement item
	_, err = tx.Exec(`
		DELETE FROM settlement_items
		WHERE settlement_id = $1 AND transaction_id = $2
	`, settlementID, transactionID)

	if err != nil {
		return fmt.Errorf("failed to remove settlement item: %w", err)
	}

	// Update the settlement's remaining amount
	_, err = tx.Exec(`
		UPDATE settlements 
		SET remaining_amount = remaining_amount + $1, 
		    status = CASE WHEN status = 'completed' THEN 'active' ELSE status END,
		    completed_at = CASE WHEN status = 'completed' THEN NULL ELSE completed_at END,
		    updated_at = $2
		WHERE id = $3
	`, appliedAmount, time.Now(), settlementID)

	if err != nil {
		return fmt.Errorf("failed to update settlement: %w", err)
	}

	// Add history entry
	details := models.SettlementDetails{
		"amount_removed": appliedAmount,
	}

	if err := s.addHistoryEntry(tx, settlementID, "transaction_removed", userID, &transactionID, &appliedAmount, details); err != nil {
		return err
	}

	return tx.Commit()
}

// GetSettlement retrieves a settlement with all its items and history
func (s *SettlementService) GetSettlement(settlementID string) (*models.Settlement, error) {
	var settlement models.Settlement
	err := s.db.QueryRow(`
		SELECT id, created_by, created_for, total_amount, remaining_amount, status, 
		       created_at, updated_at, completed_at, notes
		FROM settlements
		WHERE id = $1
	`, settlementID).Scan(
		&settlement.ID, &settlement.CreatedBy, &settlement.CreatedFor,
		&settlement.TotalAmount, &settlement.RemainingAmount, &settlement.Status,
		&settlement.CreatedAt, &settlement.UpdatedAt, &settlement.CompletedAt, &settlement.Notes,
	)

	if err != nil {
		return nil, err
	}

	// Get settlement items
	items, err := s.getSettlementItems(settlementID)
	if err != nil {
		return nil, err
	}
	settlement.Items = items

	// Get history
	history, err := s.getSettlementHistory(settlementID)
	if err != nil {
		return nil, err
	}
	settlement.History = history

	return &settlement, nil
}

// GetUserSettlements retrieves all settlements for a user
func (s *SettlementService) GetUserSettlements(userID string, status string) ([]models.SettlementSummary, error) {
	query := `
		SELECT s.id, s.created_by, u1.name as created_by_name, s.created_for, u2.name as created_for_name,
		       s.total_amount, s.remaining_amount, s.status, s.created_at,
		       COUNT(si.id) as item_count
		FROM settlements s
		JOIN users u1 ON s.created_by = u1.id
		JOIN users u2 ON s.created_for = u2.id
		LEFT JOIN settlement_items si ON s.id = si.settlement_id
		WHERE (s.created_by = $1 OR s.created_for = $1)
	`

	args := []interface{}{userID}
	if status != "" {
		query += " AND s.status = $2"
		args = append(args, status)
	}

	query += " GROUP BY s.id, u1.name, u2.name ORDER BY s.created_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var settlements []models.SettlementSummary
	for rows.Next() {
		var summary models.SettlementSummary
		err := rows.Scan(
			&summary.ID, &summary.CreatedBy, &summary.CreatedByName,
			&summary.CreatedFor, &summary.CreatedForName,
			&summary.TotalAmount, &summary.RemainingAmount, &summary.Status,
			&summary.CreatedAt, &summary.ItemCount,
		)
		if err != nil {
			return nil, err
		}
		settlements = append(settlements, summary)
	}

	return settlements, nil
}

// Helper functions

func (s *SettlementService) getSettlementItems(settlementID string) ([]models.SettlementItem, error) {
	rows, err := s.db.Query(`
		SELECT si.id, si.settlement_id, si.transaction_id, si.applied_amount, 
		       si.created_at, si.created_by, 
		       t.amount, t.description, t.date, t.pay_to, t.entered_by
		FROM settlement_items si
		JOIN transactions t ON si.transaction_id = t.id
		WHERE si.settlement_id = $1
		ORDER BY si.created_at DESC
	`, settlementID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.SettlementItem
	for rows.Next() {
		var item models.SettlementItem
		var transaction models.Transaction
		var dateStr string

		err := rows.Scan(
			&item.ID, &item.SettlementID, &item.TransactionID, &item.AppliedAmount,
			&item.CreatedAt, &item.CreatedBy,
			&transaction.Amount, &transaction.Description, &dateStr,
			&transaction.PayTo, &transaction.EnteredBy,
		)
		if err != nil {
			return nil, err
		}

		// Parse the date string
		if dateStr != "" {
			parsedDate, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse date: %w", err)
			}
			transaction.Date = parsedDate
		}

		transaction.ID = item.TransactionID
		item.Transaction = &transaction
		items = append(items, item)
	}

	return items, nil
}

func (s *SettlementService) getSettlementHistory(settlementID string) ([]models.SettlementHistory, error) {
	rows, err := s.db.Query(`
		SELECT id, settlement_id, action, actor_id, transaction_id, amount, details, created_at
		FROM settlement_history
		WHERE settlement_id = $1
		ORDER BY created_at DESC
	`, settlementID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []models.SettlementHistory
	for rows.Next() {
		var entry models.SettlementHistory
		var detailsJSON []byte

		err := rows.Scan(
			&entry.ID, &entry.SettlementID, &entry.Action, &entry.ActorID,
			&entry.TransactionID, &entry.Amount, &detailsJSON, &entry.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if detailsJSON != nil {
			json.Unmarshal(detailsJSON, &entry.Details)
		}

		history = append(history, entry)
	}

	return history, nil
}

func (s *SettlementService) addHistoryEntry(tx *sql.Tx, settlementID, action, actorID string, transactionID *string, amount *float64, details models.SettlementDetails) error {
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO settlement_history (settlement_id, action, actor_id, transaction_id, amount, details, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, settlementID, action, actorID, transactionID, amount, detailsJSON, time.Now())

	return err
}

// ApplyTransactionAsPayment creates a new settlement when someone applies another user's transaction as payment
func (s *SettlementService) ApplyTransactionAsPayment(userID string, transactionID string, notes string) (*models.Settlement, error) {
	// Get the transaction
	var transaction models.Transaction
	err := s.db.QueryRow(`
		SELECT id, amount, pay_to, entered_by, user_id 
		FROM transactions 
		WHERE id = $1 AND paid = false
	`, transactionID).Scan(&transaction.ID, &transaction.Amount, &transaction.PayTo, &transaction.EnteredBy, &transaction.UserID)
	
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	// The user applying this transaction should be the one who owes money
	// The transaction should have been entered by the person they owe
	if transaction.EnteredBy == userID {
		return nil, fmt.Errorf("you cannot apply your own transactions as payment")
	}

	// Create a settlement where:
	// - created_by is the person who entered the original transaction (creditor)
	// - created_for is the current user (debtor applying the payment)
	settlementID := uuid.New().String()
	now := time.Now()

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Create the settlement
	_, err = tx.Exec(`
		INSERT INTO settlements (id, created_by, created_for, total_amount, remaining_amount, status, notes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, settlementID, transaction.EnteredBy, userID, transaction.Amount, 0, "completed", notes, now, now)
	
	if err != nil {
		return nil, fmt.Errorf("failed to create settlement: %w", err)
	}

	// Add the transaction as a settlement item
	_, err = tx.Exec(`
		INSERT INTO settlement_items (settlement_id, transaction_id, applied_amount, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, settlementID, transactionID, transaction.Amount, userID, now)
	
	if err != nil {
		return nil, fmt.Errorf("failed to create settlement item: %w", err)
	}

	// Create history entry
	details := models.SettlementDetails{
		"transaction_id": transactionID,
		"amount": transaction.Amount,
		"applied_by": userID,
		"payment_application": true,
	}
	
	if err := s.addHistoryEntry(tx, settlementID, "created", userID, &transactionID, &transaction.Amount, details); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s.GetSettlement(settlementID)
}
