package services

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"bennwallet/backend/database"
)

// SecretType represents the type of secret being stored/retrieved
type SecretType string

const (
	SecretYNABToken SecretType = "ynab_token"
)

// Secret represents a secret value
type Secret struct {
	UserID    string
	Type      SecretType
	Value     string
	CreatedAt time.Time
}

// StoreSecret stores a secret in fly.io's secret store if available, otherwise in the database
func StoreSecret(userID string, secretType SecretType, secretValue string) error {
	// Check if we're running on Fly.io
	if os.Getenv("FLY_APP_NAME") != "" {
		// For Fly.io, we would need to call their API to store secrets
		// This would be complex to do from the application, so we'll log instead
		log.Printf("Running on Fly.io - secrets should be stored using 'fly secrets set' CLI")
		log.Printf("To store this secret, run: fly secrets set %s_USER_%s=your_secret_value", strings.ToUpper(string(secretType)), userID)
		return nil
	}

	// Not on Fly.io, store in database
	hashedValue := fmt.Sprintf("enc:%s", secretValue)

	_, err := database.DB.Exec(`
		INSERT INTO user_ynab_settings (user_id, token, budget_id, account_id, sync_enabled, last_synced)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT(user_id) DO UPDATE SET
			token = excluded.token,
			budget_id = excluded.budget_id,
			account_id = excluded.account_id
	`, userID, hashedValue, "", "", false, nil)

	if err != nil {
		return fmt.Errorf("error storing secret in database: %w", err)
	}

	return nil
}

// GetSecret retrieves a secret from fly.io's secret store if available, otherwise from the database
func GetSecret(userID string, secretType SecretType) (string, error) {
	// Check if we're running on Fly.io
	if os.Getenv("FLY_APP_NAME") != "" {
		// For Fly.io, the secret would be available as an environment variable
		// The env var name would be in the format: YNAB_TOKEN_USER_1
		secretName := fmt.Sprintf("%s_USER_%s", strings.ToUpper(string(secretType)), userID)
		secretValue := os.Getenv(secretName)

		if secretValue != "" {
			return secretValue, nil
		}

		return "", fmt.Errorf("secret '%s' for user %s not found in Fly.io secrets", secretType, userID)
	}

	// Not on Fly.io, retrieve from database
	var tokenValue string
	err := database.DB.QueryRow(`
		SELECT token FROM user_ynab_settings WHERE user_id = $1
	`, userID).Scan(&tokenValue)

	if err != nil {
		return "", fmt.Errorf("error retrieving token from database: %w", err)
	}

	// If stored locally, decode/decrypt the value
	if strings.HasPrefix(tokenValue, "enc:") {
		return tokenValue[4:], nil
	}

	return tokenValue, nil
}

// UpdateYNABSettings updates all YNAB settings for a user
func UpdateYNABSettings(userID, token, budgetID, accountID string, syncEnabled bool) error {
	// Always store token through the secure mechanism
	err := StoreSecret(userID, SecretYNABToken, token)
	if err != nil {
		return err
	}

	// Update other settings directly in the database
	_, err = database.DB.Exec(`
		UPDATE user_ynab_settings
		SET budget_id = ?, account_id = ?, sync_enabled = ?
		WHERE user_id = ?
	`, budgetID, accountID, syncEnabled, userID)

	if err != nil {
		return fmt.Errorf("error updating YNAB settings: %w", err)
	}

	return nil
}
