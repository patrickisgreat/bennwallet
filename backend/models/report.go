package models

import "time"

// Report represents a predefined report in the system
type Report struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// CustomReport represents a user-defined report
type CustomReport struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	UserID       string    `json:"userId"`
	Description  string    `json:"description"`
	ReportConfig string    `json:"reportConfig"` // JSON string of report configuration
	IsPublic     bool      `json:"isPublic"`     // Whether this report is shared with other users
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// ReportConfig represents the configuration for a custom report
type ReportConfig struct {
	Type         string            `json:"type"`         // Type of report (summary, detailed, etc.)
	DataSource   string            `json:"dataSource"`   // transactions, categories, etc.
	Filters      []FilterParameter `json:"filters"`      // Filters to apply
	GroupBy      []string          `json:"groupBy"`      // Fields to group by
	Aggregations []Aggregation     `json:"aggregations"` // Aggregations to calculate
	SortField    string            `json:"sortField"`    // Field to sort by
	SortOrder    string            `json:"sortOrder"`    // ASC or DESC
	Limit        int               `json:"limit"`        // Max records to return
}

// Aggregation represents an aggregation function to apply in a report
type Aggregation struct {
	Function string `json:"function"` // SUM, AVG, COUNT, etc.
	Field    string `json:"field"`    // Field to aggregate
	Alias    string `json:"alias"`    // Alias for the aggregated field in the output
}

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
