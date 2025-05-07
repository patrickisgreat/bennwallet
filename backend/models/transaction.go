package models

import "time"

type Transaction struct {
	ID              string     `json:"id"`
	Amount          float64    `json:"amount"`
	Description     string     `json:"description"`
	Date            time.Time  `json:"date"`
	TransactionDate time.Time  `json:"transactionDate"`
	Type            string     `json:"type"`
	PayTo           string     `json:"payTo,omitempty"`
	Paid            bool       `json:"paid"`
	PaidDate        string     `json:"paidDate,omitempty"`
	EnteredBy       string     `json:"enteredBy"`
	Optional        bool       `json:"optional"`
	UserID          string     `json:"userId,omitempty"`
	Categories      []Category `json:"categories,omitempty"`
}

// TransactionCategory represents a join between a transaction and a category
type TransactionCategory struct {
	ID            int       `json:"id"`
	TransactionID string    `json:"transactionId"`
	CategoryID    int       `json:"categoryId"`
	Amount        float64   `json:"amount"`
	CreatedAt     time.Time `json:"createdAt"`
}
