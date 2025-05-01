package models

import "time"

// YNABConfig stores YNAB API configuration
type YNABConfig struct {
	ApiToken      string    `json:"apiToken"`
	BudgetID      string    `json:"budgetId"`
	LastSyncTime  time.Time `json:"lastSyncTime"`
	SyncFrequency int       `json:"syncFrequency"` // in minutes
}

// CategoryGroup represents a YNAB category group
type CategoryGroup struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Hidden     bool       `json:"hidden"`
	Deleted    bool       `json:"deleted"`
	Categories []Category `json:"categories"`
}

// YNABCategory represents a category from YNAB API
type YNABCategory struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	CategoryGroupID string `json:"category_group_id"`
	Hidden          bool   `json:"hidden"`
	Deleted         bool   `json:"deleted"`
}
