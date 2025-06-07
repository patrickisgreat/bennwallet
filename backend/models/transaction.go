package models

import "time"

type Transaction struct {
	ID              string     `json:"id"`
	Amount          float64    `json:"amount"`
	Description     string     `json:"description"`
	Date            time.Time  `json:"date"`
	TransactionDate time.Time  `json:"transactionDate"`
	Type            string     `json:"type"`
	PaidBy          string     `json:"paidBy,omitempty"` // User ID of who paid for this expense
	OwedBy          string     `json:"owedBy,omitempty"` // User ID of who owes money for this expense
	PayTo           string     `json:"payTo,omitempty"`  // Deprecated - kept for backward compatibility
	Paid            bool       `json:"paid"`
	PaidDate        string     `json:"paidDate,omitempty"`
	EnteredBy       string     `json:"enteredBy"`
	Optional        bool       `json:"optional"`
	UserID          string     `json:"userId,omitempty"`
	Note            string     `json:"note,omitempty"`
	InSettlement    bool       `json:"inSettlement,omitempty"` // Whether this transaction is part of a settlement
	Categories      []Category `json:"categories,omitempty"`

	// Display names - populated by API for frontend display
	PaidByName    string `json:"paidByName,omitempty"`
	OwedByName    string `json:"owedByName,omitempty"`
	EnteredByName string `json:"enteredByName,omitempty"`
}

// TransactionCategory represents a join between a transaction and a category
type TransactionCategory struct {
	ID            int       `json:"id"`
	TransactionID string    `json:"transactionId"`
	CategoryID    int       `json:"categoryId"`
	Amount        float64   `json:"amount"`
	CreatedAt     time.Time `json:"createdAt"`
}
