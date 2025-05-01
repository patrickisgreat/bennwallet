package models

import (
	"database/sql"
	"time"
)

// YNABConfig represents the YNAB configuration for a user
type YNABConfig struct {
	ID                 int64     `json:"id"`
	UserID             string    `json:"user_id"`
	EncryptedAPIToken  string    `json:"-"`
	EncryptedBudgetID  string    `json:"-"`
	EncryptedAccountID string    `json:"-"`
	LastSyncTime       time.Time `json:"last_sync_time"`
	SyncFrequency      int       `json:"sync_frequency"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// YNABConfigResponse is the safe version of YNABConfig for API responses
type YNABConfigResponse struct {
	ID            int64     `json:"id"`
	UserID        string    `json:"user_id"`
	LastSyncTime  time.Time `json:"last_sync_time"`
	SyncFrequency int       `json:"sync_frequency"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ToResponse converts a YNABConfig to a YNABConfigResponse
func (c *YNABConfig) ToResponse() YNABConfigResponse {
	return YNABConfigResponse{
		ID:            c.ID,
		UserID:        c.UserID,
		LastSyncTime:  c.LastSyncTime,
		SyncFrequency: c.SyncFrequency,
		CreatedAt:     c.CreatedAt,
		UpdatedAt:     c.UpdatedAt,
	}
}

// GetYNABConfig retrieves the YNAB configuration for a user
func GetYNABConfig(db *sql.DB, userID string) (*YNABConfig, error) {
	var config YNABConfig
	err := db.QueryRow(`
		SELECT id, user_id, encrypted_api_token, encrypted_budget_id, encrypted_account_id, 
		       last_sync_time, sync_frequency, created_at, updated_at
		FROM ynab_config
		WHERE user_id = ?`, userID).Scan(
		&config.ID, &config.UserID, &config.EncryptedAPIToken, &config.EncryptedBudgetID,
		&config.EncryptedAccountID, &config.LastSyncTime, &config.SyncFrequency,
		&config.CreatedAt, &config.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &config, nil
}

// SaveYNABConfig saves or updates the YNAB configuration for a user
func SaveYNABConfig(db *sql.DB, config *YNABConfig) error {
	// Check if config exists
	existing, err := GetYNABConfig(db, config.UserID)
	if err != nil {
		return err
	}

	if existing == nil {
		// Insert new config
		_, err = db.Exec(`
			INSERT INTO ynab_config (user_id, encrypted_api_token, encrypted_budget_id, 
				encrypted_account_id, sync_frequency)
			VALUES (?, ?, ?, ?, ?)`,
			config.UserID, config.EncryptedAPIToken, config.EncryptedBudgetID,
			config.EncryptedAccountID, config.SyncFrequency,
		)
	} else {
		// Update existing config
		_, err = db.Exec(`
			UPDATE ynab_config
			SET encrypted_api_token = ?, encrypted_budget_id = ?, encrypted_account_id = ?,
				sync_frequency = ?, updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ?`,
			config.EncryptedAPIToken, config.EncryptedBudgetID, config.EncryptedAccountID,
			config.SyncFrequency, config.UserID,
		)
	}
	return err
}

// UpdateLastSyncTime updates the last sync time for a user's YNAB config
func UpdateLastSyncTime(db *sql.DB, userID string) error {
	_, err := db.Exec(`
		UPDATE ynab_config
		SET last_sync_time = CURRENT_TIMESTAMP
		WHERE user_id = ?`, userID)
	return err
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
