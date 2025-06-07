package services

import (
	"bennwallet/backend/models"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
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

// resolveUserReference takes a user reference (ID or name) and returns the actual user ID
func (s *SettlementService) resolveUserReference(userRef string) (string, error) {
	var userID string
	err := s.db.QueryRow(`
		SELECT id FROM users 
		WHERE id = $1 
		   OR LOWER(name) = LOWER($2)
		   OR LOWER(username) = LOWER($2)
		LIMIT 1
	`, userRef, userRef).Scan(&userID)

	if err != nil {
		return "", fmt.Errorf("user not found: %s", userRef)
	}
	return userID, nil
}

// isUserInvolved checks if a user is involved in a transaction (as payer or ower)
func (s *SettlementService) isUserInvolved(userID string, paidBy string, owedBy string) (bool, string) {
	// Try to resolve the paid_by reference
	paidByID, _ := s.resolveUserReference(paidBy)
	if paidByID == userID {
		return true, "paid"
	}

	// Try to resolve the owed_by reference
	owedByID, _ := s.resolveUserReference(owedBy)
	if owedByID == userID {
		return true, "owed"
	}

	return false, ""
}

// CreateSettlement creates a new settlement for applying transactions against debt
// This is used when someone wants to apply their transactions to offset debt
func (s *SettlementService) CreateSettlement(userID string, transactionID string, notes string) (*models.Settlement, error) {
	// First, get the transaction to determine the amount and who it's for
	var transaction models.Transaction
	var owedBy sql.NullString
	err := s.db.QueryRow(`
		SELECT id, amount, paid_by, owed_by, entered_by, user_id 
		FROM transactions 
		WHERE id = $1
	`, transactionID).Scan(&transaction.ID, &transaction.Amount, &transaction.PaidBy, &owedBy, &transaction.EnteredBy, &transaction.UserID)

	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	// Set OwedBy if it was in the database
	if owedBy.Valid {
		transaction.OwedBy = owedBy.String
	}

	// Determine who creates the settlement and who it's for
	// Settlement is always between current user and the other person in the transaction
	var createdBy, createdFor string
	createdBy = userID

	// Check if the user is involved in this transaction
	isInvolved, role := s.isUserInvolved(userID, transaction.PaidBy, transaction.OwedBy)
	if !isInvolved {
		return nil, fmt.Errorf("you can only create settlements for transactions where you paid or owe money")
	}

	// Determine who the settlement is with based on the user's role
	if role == "owed" {
		// Current user owes money, settlement is with the person who paid
		createdFor = transaction.PaidBy
	} else {
		// Current user paid, settlement is with the person who owes
		createdFor = transaction.OwedBy
	}

	// Resolve the other user's ID
	otherUserID, err := s.resolveUserReference(createdFor)
	if err != nil {
		return nil, fmt.Errorf("cannot find user to create settlement with: %w", err)
	}
	createdFor = otherUserID

	if err != nil {
		// If we can't find the user, we can't create a settlement
		return nil, fmt.Errorf("cannot find user with ID '%s' to create settlement", createdFor)
	}

	settlementID := uuid.New().String()
	now := time.Now()

	// Start a transaction
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Create an empty settlement that can grow as transactions are added
	log.Printf("DEBUG: Creating settlement %s between %s and %s", settlementID, createdBy, createdFor)
	_, err = tx.Exec(`
		INSERT INTO settlements (id, creator_id, recipient_id, total_amount, remaining_amount, status, notes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, settlementID, createdBy, createdFor, 0, 0, "active", notes, now, now)

	if err != nil {
		return nil, fmt.Errorf("failed to create settlement: %w", err)
	}

	log.Printf("DEBUG: Settlement created, now applying initial transaction %s with amount %f", transactionID, transaction.Amount)
	// Apply the initial transaction using the standard method
	if err := s.applyTransactionToSettlementTx(tx, settlementID, transactionID, transaction.Amount, userID); err != nil {
		return nil, fmt.Errorf("failed to apply initial transaction: %w", err)
	}

	// Create history entry for settlement creation
	details := models.SettlementDetails{
		"action": "settlement_created",
	}

	if err := s.addHistoryEntry(tx, settlementID, "created", userID, nil, nil, details); err != nil {
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

	if err := s.applyTransactionToSettlementTx(tx, settlementID, transactionID, amount, userID); err != nil {
		return err
	}

	return tx.Commit()
}

// applyTransactionToSettlementTx is the internal method that works within an existing transaction
func (s *SettlementService) applyTransactionToSettlementTx(tx *sql.Tx, settlementID string, transactionID string, amount float64, userID string) error {
	// Get the settlement
	var settlement models.Settlement
	err := tx.QueryRow(`
		SELECT id, total_amount, status, creator_id, recipient_id
		FROM settlements
		WHERE id = $1
	`, settlementID).Scan(&settlement.ID, &settlement.TotalAmount, &settlement.Status, &settlement.CreatorID, &settlement.RecipientID)

	if err != nil {
		return fmt.Errorf("failed to get settlement: %w", err)
	}

	// Debug logging
	log.Printf("DEBUG: Settlement %s status: %s, total: %f", settlementID, settlement.Status, settlement.TotalAmount)

	// Validate the settlement is active
	if settlement.Status != "active" {
		return fmt.Errorf("settlement is not active (status: %s)", settlement.Status)
	}

	// Validate the user has permission (either creator or target)
	if userID != settlement.CreatorID && userID != settlement.RecipientID {
		return fmt.Errorf("user does not have permission to modify this settlement")
	}

	// Add the settlement item
	_, err = tx.Exec(`
		INSERT INTO settlement_items (settlement_id, transaction_id, applied_amount, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, settlementID, transactionID, amount, userID, time.Now())

	if err != nil {
		return fmt.Errorf("failed to create settlement item: %w", err)
	}

	// Mark the transaction as paid since it's been settled
	_, err = tx.Exec(`
		UPDATE transactions 
		SET paid = true, paid_date = $1
		WHERE id = $2
	`, time.Now().Format("2006-01-02"), transactionID)

	if err != nil {
		return fmt.Errorf("failed to mark transaction as paid: %w", err)
	}

	// Simply add the transaction amount to the settlement's total
	// Don't manage "remaining amount" - settlements can grow indefinitely until manually completed
	_, err = tx.Exec(`
		UPDATE settlements 
		SET total_amount = total_amount + $1, updated_at = $2
		WHERE id = $3
	`, amount, time.Now(), settlementID)

	if err != nil {
		return fmt.Errorf("failed to update settlement total: %w", err)
	}

	// Add history entry
	details := models.SettlementDetails{
		"amount_added": amount,
	}

	if err := s.addHistoryEntry(tx, settlementID, "transaction_applied", userID, &transactionID, &amount, details); err != nil {
		return err
	}

	return nil
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

	// Mark the transaction as unpaid since it's no longer settled
	_, err = tx.Exec(`
		UPDATE transactions 
		SET paid = false, paid_date = NULL
		WHERE id = $1
	`, transactionID)

	if err != nil {
		return fmt.Errorf("failed to mark transaction as unpaid: %w", err)
	}

	// Update the settlement's total amount (subtract the removed amount)
	_, err = tx.Exec(`
		UPDATE settlements 
		SET total_amount = total_amount - $1,
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
	var completedAt sql.NullTime
	err := s.db.QueryRow(`
		SELECT id, total_amount, status, creator_id, recipient_id, created_at, updated_at, completed_at, notes
		FROM settlements
		WHERE id = $1
	`, settlementID).Scan(&settlement.ID, &settlement.TotalAmount, &settlement.Status, &settlement.CreatorID, &settlement.RecipientID, &settlement.CreatedAt, &settlement.UpdatedAt, &completedAt, &settlement.Notes)

	if err != nil {
		return nil, err
	}

	// Handle nullable completed_at
	if completedAt.Valid {
		settlement.CompletedAt = &completedAt.Time
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
		SELECT 
			s.id,
			s.creator_id,
			creator.name as creator_name,
			s.recipient_id,
			recipient.name as recipient_name,
			s.total_amount,
			s.status,
			s.created_at,
			COUNT(si.id) as item_count
		FROM settlements s
		LEFT JOIN users creator ON s.creator_id = creator.id
		LEFT JOIN users recipient ON s.recipient_id = recipient.id
		LEFT JOIN settlement_items si ON s.id = si.settlement_id
		WHERE (s.creator_id = $1 OR s.recipient_id = $1)
	`
	args := []interface{}{userID}

	if status != "" {
		query += " AND s.status = $2"
		args = append(args, status)
	}

	query += " GROUP BY s.id, creator.name, recipient.name ORDER BY s.created_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get settlements: %w", err)
	}
	defer rows.Close()

	var settlements []models.SettlementSummary
	for rows.Next() {
		var summary models.SettlementSummary
		err := rows.Scan(
			&summary.ID,
			&summary.CreatorID,
			&summary.CreatorName,
			&summary.RecipientID,
			&summary.RecipientName,
			&summary.TotalAmount,
			&summary.Status,
			&summary.CreatedAt,
			&summary.ItemCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan settlement: %w", err)
		}
		settlements = append(settlements, summary)
	}

	return settlements, nil
}

// UpdateSettlementStatus updates the status of a settlement (e.g., complete, cancel)
func (s *SettlementService) UpdateSettlementStatus(settlementID string, status string, userID string, notes string) (*models.Settlement, error) {
	// Validate status
	validStatuses := map[string]bool{
		"active":    true,
		"completed": true,
		"cancelled": true,
	}

	if !validStatuses[status] {
		return nil, fmt.Errorf("invalid status: %s", status)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Get settlement details before updating
	settlement, err := s.GetSettlement(settlementID)
	if err != nil {
		return nil, err
	}

	// Update the settlement status
	now := time.Now()
	query := `
		UPDATE settlements 
		SET status = $1, updated_at = $2, notes = CASE WHEN $3 = '' THEN notes ELSE $3 END
	`
	args := []interface{}{status, now, notes}

	// Set completed_at if completing the settlement
	if status == "completed" {
		query += ", completed_at = $4 WHERE id = $5"
		args = append(args, now, settlementID)
	} else {
		query += " WHERE id = $4"
		args = append(args, settlementID)
	}

	_, err = tx.Exec(query, args...)

	if err != nil {
		return nil, fmt.Errorf("failed to update settlement status: %w", err)
	}

	// If completing the settlement and there's a net amount, create adjustment transactions
	if status == "completed" && settlement.TotalAmount > 0 {
		err = s.createSettlementAdjustmentTransactions(tx, settlement, now)
		if err != nil {
			return nil, fmt.Errorf("failed to create settlement adjustment transactions: %w", err)
		}

		// Mark all transactions in the settlement as paid
		_, err = tx.Exec(`
			UPDATE transactions 
			SET paid = true, paid_date = $1
			WHERE id IN (
				SELECT transaction_id 
				FROM settlement_items 
				WHERE settlement_id = $2
			)
		`, now.Format("2006-01-02"), settlementID)
		if err != nil {
			return nil, fmt.Errorf("failed to mark transactions as paid: %w", err)
		}
	}

	// If cancelling the settlement, mark all associated transactions as unpaid
	if status == "cancelled" {
		_, err = tx.Exec(`
			UPDATE transactions 
			SET paid = false 
			WHERE id IN (
				SELECT transaction_id 
				FROM settlement_items 
				WHERE settlement_id = $1
			)
		`, settlementID)
		if err != nil {
			return nil, fmt.Errorf("failed to mark transactions as unpaid: %w", err)
		}
	}

	// Create history entry
	details := models.SettlementDetails{
		"status": status,
		"notes":  notes,
	}

	if err := s.addHistoryEntry(tx, settlementID, "status_updated", userID, nil, nil, details); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s.GetSettlement(settlementID)
}

// createSettlementAdjustmentTransactions creates adjustment transactions to reflect the settlement
func (s *SettlementService) createSettlementAdjustmentTransactions(tx *sql.Tx, settlement *models.Settlement, now time.Time) error {
	// Calculate net amounts by user
	userAmounts := make(map[string]float64)

	// Get all settlement items to calculate net
	for _, item := range settlement.Items {
		// Get the transaction details
		var paidBy, owedBy sql.NullString
		err := tx.QueryRow(`
			SELECT paid_by, owed_by FROM transactions WHERE id = $1
		`, item.TransactionID).Scan(&paidBy, &owedBy)

		if err != nil {
			continue // Skip if we can't find the transaction
		}

		// Add/subtract amounts based on who paid/owes
		if paidBy.Valid {
			userAmounts[paidBy.String] += item.AppliedAmount
		}
		if owedBy.Valid {
			userAmounts[owedBy.String] -= item.AppliedAmount
		}
	}

	// Create adjustment transactions for non-zero balances
	for userID, amount := range userAmounts {
		if amount == 0 {
			continue // Skip zero amounts
		}

		// Determine the other user
		var otherUserID string
		if userID == settlement.CreatorID {
			otherUserID = settlement.RecipientID
		} else {
			otherUserID = userID
		}

		// Create settlement adjustment transaction
		adjID := uuid.New().String()
		description := fmt.Sprintf("Settlement adjustment")

		var paidBy, owedBy string
		if amount > 0 {
			// This user is owed money
			paidBy = userID
			owedBy = otherUserID
		} else {
			// This user owes money
			paidBy = otherUserID
			owedBy = userID
			amount = -amount // Make positive
		}

		_, err := tx.Exec(`
			INSERT INTO transactions 
			(id, amount, description, date, transaction_date, type, paid_by, owed_by, paid, paid_date, entered_by, optional, note, user_id) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		`, adjID, amount, description, now.Format("2006-01-02"), now.Format("2006-01-02"),
			"settlement", paidBy, owedBy, true, now.Format("2006-01-02"), settlement.CreatorID, false,
			fmt.Sprintf("Settlement adjustment - ID: %s", settlement.ID[:8]), settlement.CreatorID)

		if err != nil {
			return fmt.Errorf("failed to create adjustment transaction: %w", err)
		}
	}

	return nil
}

// Helper functions

func (s *SettlementService) getSettlementItems(settlementID string) ([]models.SettlementItem, error) {
	rows, err := s.db.Query(`
		SELECT si.id, si.settlement_id, si.transaction_id, si.applied_amount, 
		       si.created_at, si.created_by, 
		       t.amount, t.description, t.date, t.paid_by, t.owed_by, t.entered_by
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

		var owedBy sql.NullString
		err := rows.Scan(
			&item.ID, &item.SettlementID, &item.TransactionID, &item.AppliedAmount,
			&item.CreatedAt, &item.CreatedBy,
			&transaction.Amount, &transaction.Description, &dateStr,
			&transaction.PaidBy, &owedBy, &transaction.EnteredBy,
		)
		if err != nil {
			return nil, err
		}

		// Set OwedBy if it was in the database
		if owedBy.Valid {
			transaction.OwedBy = owedBy.String
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
	var owedBy sql.NullString
	err := s.db.QueryRow(`
		SELECT id, amount, paid_by, owed_by, entered_by, user_id 
		FROM transactions 
		WHERE id = $1 AND paid = false
	`, transactionID).Scan(&transaction.ID, &transaction.Amount, &transaction.PaidBy, &owedBy, &transaction.EnteredBy, &transaction.UserID)

	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	// Set OwedBy if it was in the database
	if owedBy.Valid {
		transaction.OwedBy = owedBy.String
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

	// Create the settlement as active (can be added to later)
	_, err = tx.Exec(`
		INSERT INTO settlements (id, creator_id, recipient_id, total_amount, remaining_amount, status, notes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, settlementID, transaction.EnteredBy, userID, transaction.Amount, transaction.Amount, "active", notes, now, now)

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
		"transaction_id":      transactionID,
		"amount":              transaction.Amount,
		"applied_by":          userID,
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

// GetSettlementsGroupedByMonth returns settlements grouped by month
func (s *SettlementService) GetSettlementsGroupedByMonth(userID string, status string) ([]models.MonthlySettlementGroup, error) {
	settlements, err := s.GetUserSettlements(userID, status)
	if err != nil {
		return nil, err
	}

	// Group settlements by month
	monthGroups := make(map[string][]models.SettlementSummary)
	monthInfo := make(map[string]models.MonthlySettlementGroup)

	for _, settlement := range settlements {
		monthKey := settlement.CreatedAt.Format("2006-01")

		if _, exists := monthGroups[monthKey]; !exists {
			monthGroups[monthKey] = []models.SettlementSummary{}
			monthInfo[monthKey] = models.MonthlySettlementGroup{
				Month:     monthKey,
				Year:      settlement.CreatedAt.Year(),
				MonthName: settlement.CreatedAt.Format("January"),
			}
		}

		monthGroups[monthKey] = append(monthGroups[monthKey], settlement)
	}

	// Convert map to slice and calculate totals
	var result []models.MonthlySettlementGroup
	for monthKey, settlements := range monthGroups {
		group := monthInfo[monthKey]
		group.Settlements = settlements

		// Calculate totals
		totalAmount := 0.0
		totalItems := 0
		for _, settlement := range settlements {
			totalAmount += settlement.TotalAmount
			totalItems += settlement.ItemCount
		}

		group.TotalAmount = totalAmount
		group.ItemCount = totalItems

		result = append(result, group)
	}

	// Sort by month (newest first)
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].Month < result[j].Month {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result, nil
}

// GetSettlementReportData returns settlement report data for a specific month
func (s *SettlementService) GetSettlementReportData(userID string, month string, showPaid bool) (*models.SettlementReport, error) {
	// Parse month (YYYY-MM format)
	parts := strings.Split(month, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid month format, expected YYYY-MM")
	}

	year, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid year: %w", err)
	}

	monthInt, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid month: %w", err)
	}

	startDate := time.Date(year, time.Month(monthInt), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 1, 0)
	monthName := startDate.Format("January")

	// Get settlements for the month
	query := `
		SELECT 
			s.id, s.creator_id, s.recipient_id, s.total_amount, s.status,
			si.applied_amount,
			t.amount as transaction_amount, t.paid_by, t.owed_by,
			c.id as category_id, c.name as category_name
		FROM settlements s
		JOIN settlement_items si ON s.id = si.settlement_id
		JOIN transactions t ON si.transaction_id = t.id
		LEFT JOIN transaction_categories tc ON t.id = tc.transaction_id
		LEFT JOIN ynab_categories c ON tc.category_id = c.id
		WHERE (s.creator_id = $1 OR s.recipient_id = $1)
		AND s.created_at >= $2 AND s.created_at < $3
	`

	if !showPaid {
		query += " AND s.status != 'completed'"
	}

	query += " ORDER BY s.created_at DESC"

	rows, err := s.db.Query(query, userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get settlement data: %w", err)
	}
	defer rows.Close()

	categoryTotals := make(map[string]*models.SettlementCategoryTotal)
	settlementDeductions := make(map[string]*models.SettlementCategoryTotal)
	totalOwed := 0.0
	totalPaid := 0.0

	for rows.Next() {
		var settlementID, creatorID, recipientID, status string
		var settlementAmount, appliedAmount, transactionAmount float64
		var paidBy, owedBy sql.NullString
		var categoryID, categoryName sql.NullString

		err := rows.Scan(
			&settlementID, &creatorID, &recipientID, &settlementAmount, &status,
			&appliedAmount,
			&transactionAmount, &paidBy, &owedBy,
			&categoryID, &categoryName,
		)
		if err != nil {
			continue
		}

		// Determine if this is owed or paid by the current user
		isOwedByUser := owedBy.Valid && owedBy.String == userID
		isPaidByUser := paidBy.Valid && paidBy.String == userID

		if showPaid {
			if isPaidByUser {
				totalPaid += appliedAmount
			}
			if isOwedByUser {
				totalOwed += appliedAmount
			}
		} else {
			// For unpaid items, we want to show what's still owed
			if isOwedByUser {
				totalOwed += appliedAmount
			}
		}

		// Track category totals
		if categoryID.Valid && categoryName.Valid {
			catKey := categoryID.String

			if _, exists := categoryTotals[catKey]; !exists {
				categoryTotals[catKey] = &models.SettlementCategoryTotal{
					CategoryID:   categoryID.String,
					CategoryName: categoryName.String,
					Amount:       0,
					Count:        0,
				}
			}

			categoryTotals[catKey].Amount += appliedAmount
			categoryTotals[catKey].Count++

			// Settlement deductions (amounts being settled)
			if status == "completed" || showPaid {
				if _, exists := settlementDeductions[catKey]; !exists {
					settlementDeductions[catKey] = &models.SettlementCategoryTotal{
						CategoryID:   categoryID.String,
						CategoryName: categoryName.String,
						Amount:       0,
						Count:        0,
					}
				}
				settlementDeductions[catKey].Amount += appliedAmount
				settlementDeductions[catKey].Count++
			}
		}
	}

	// Convert maps to slices
	var categoryTotalSlice []models.SettlementCategoryTotal
	for _, ct := range categoryTotals {
		categoryTotalSlice = append(categoryTotalSlice, *ct)
	}

	var settlementDeductionSlice []models.SettlementCategoryTotal
	for _, sd := range settlementDeductions {
		settlementDeductionSlice = append(settlementDeductionSlice, *sd)
	}

	report := &models.SettlementReport{
		Month:                month,
		Year:                 year,
		MonthName:            monthName,
		TotalOwed:            totalOwed,
		TotalPaid:            totalPaid,
		NetAmount:            totalPaid - totalOwed,
		CategoryTotals:       categoryTotalSlice,
		SettlementDeductions: settlementDeductionSlice,
	}

	return report, nil
}
