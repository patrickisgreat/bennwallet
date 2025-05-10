package models

import "time"

// SavedFilter represents a saved filter configuration for a specific resource type
type SavedFilter struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	UserID       string    `json:"userId"`
	ResourceType string    `json:"resourceType"` // transactions, categories, etc.
	FilterConfig string    `json:"filterConfig"` // JSON string of filter parameters
	IsDefault    bool      `json:"isDefault"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// FilterParameter represents a single filter parameter in a filter configuration
type FilterParameter struct {
	Field    string      `json:"field"`    // Field name to filter on
	Operator string      `json:"operator"` // =, !=, >, <, LIKE, etc.
	Value    interface{} `json:"value"`    // Value to compare against
}

// TransactionFilterConfig represents a filter configuration for transactions
type TransactionFilterConfig struct {
	Parameters []FilterParameter `json:"parameters"`
	SortField  string            `json:"sortField"`
	SortOrder  string            `json:"sortOrder"` // ASC or DESC
	Limit      int               `json:"limit"`     // Max records to return
}

// CategoryFilterConfig represents a filter configuration for categories
type CategoryFilterConfig struct {
	Parameters []FilterParameter `json:"parameters"`
	SortField  string            `json:"sortField"`
	SortOrder  string            `json:"sortOrder"` // ASC or DESC
	Limit      int               `json:"limit"`     // Max records to return
}
