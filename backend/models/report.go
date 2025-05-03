package models

type ReportFilter struct {
	StartDate string `json:"startDate,omitempty"`
	EndDate   string `json:"endDate,omitempty"`
	Category  string `json:"category,omitempty"`
	PayTo     string `json:"payTo,omitempty"`
	EnteredBy string `json:"enteredBy,omitempty"`
	Paid      *bool  `json:"paid,omitempty"`
	Optional  *bool  `json:"optional,omitempty"`
	UserId    string `json:"userId,omitempty"`
	// New fields for transaction date filtering
	TransactionDateMonth *int `json:"transactionDateMonth,omitempty"` // 1-12 for month
	TransactionDateYear  *int `json:"transactionDateYear,omitempty"`  // Full year (e.g., 2024)
}

type CategoryTotal struct {
	Category string  `json:"category"`
	Total    float64 `json:"total"`
}
