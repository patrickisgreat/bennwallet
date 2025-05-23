package models

import "time"

// Secret represents a user's secret in the database
type Secret struct {
	UserID      string    `json:"user_id"`
	SecretType  string    `json:"secret_type"`
	SecretValue string    `json:"secret_value"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
} 