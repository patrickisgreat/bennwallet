package models

import "time"

type Transaction struct {
	ID          string    `json:"id"`
	Amount      float64   `json:"amount"`
	Description string    `json:"description"`
	Date        time.Time `json:"date"`
	Type        string    `json:"type"`
	PayTo       string    `json:"payTo,omitempty"`
	Paid        bool      `json:"paid"`
	PaidDate    string    `json:"paidDate,omitempty"`
	EnteredBy   string    `json:"enteredBy"`
}
