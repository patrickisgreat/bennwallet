package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// Settlement represents a transaction offsetting/settlement between users
type Settlement struct {
	ID               string           `json:"id"`
	CreatedBy        string           `json:"createdBy"`
	CreatedFor       string           `json:"createdFor"`
	TotalAmount      float64          `json:"totalAmount"`
	RemainingAmount  float64          `json:"remainingAmount"`
	Status           string           `json:"status"` // active, completed, cancelled
	CreatedAt        time.Time        `json:"createdAt"`
	UpdatedAt        time.Time        `json:"updatedAt"`
	CompletedAt      *time.Time       `json:"completedAt,omitempty"`
	Notes            string           `json:"notes,omitempty"`
	Items            []SettlementItem `json:"items,omitempty"`
	History          []SettlementHistory `json:"history,omitempty"`
}

// SettlementItem represents a transaction applied to a settlement
type SettlementItem struct {
	ID            int       `json:"id"`
	SettlementID  string    `json:"settlementId"`
	TransactionID string    `json:"transactionId"`
	AppliedAmount float64   `json:"appliedAmount"`
	CreatedAt     time.Time `json:"createdAt"`
	CreatedBy     string    `json:"createdBy"`
	Transaction   *Transaction `json:"transaction,omitempty"`
}

// SettlementHistory tracks all actions taken on a settlement
type SettlementHistory struct {
	ID            int              `json:"id"`
	SettlementID  string           `json:"settlementId"`
	Action        string           `json:"action"` // created, transaction_applied, transaction_removed, completed, cancelled
	ActorID       string           `json:"actorId"`
	TransactionID *string          `json:"transactionId,omitempty"`
	Amount        *float64         `json:"amount,omitempty"`
	Details       SettlementDetails `json:"details,omitempty"`
	CreatedAt     time.Time        `json:"createdAt"`
}

// SettlementDetails holds additional JSON data for history entries
type SettlementDetails map[string]interface{}

// Value implements the driver.Valuer interface for database storage
func (sd SettlementDetails) Value() (driver.Value, error) {
	if sd == nil {
		return nil, nil
	}
	return json.Marshal(sd)
}

// Scan implements the sql.Scanner interface for database retrieval
func (sd *SettlementDetails) Scan(value interface{}) error {
	if value == nil {
		*sd = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, sd)
}

// SettlementSummary provides a summary view for listing settlements
type SettlementSummary struct {
	ID              string    `json:"id"`
	CreatedBy       string    `json:"createdBy"`
	CreatedByName   string    `json:"createdByName"`
	CreatedFor      string    `json:"createdFor"`
	CreatedForName  string    `json:"createdForName"`
	TotalAmount     float64   `json:"totalAmount"`
	RemainingAmount float64   `json:"remainingAmount"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"createdAt"`
	ItemCount       int       `json:"itemCount"`
}