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
func StoreSecret(userID string, secretType SecretType, value string) error {
	// Check if we're running on Fly.io
	if os.Getenv("FLY_APP_NAME") != "" {
		// For Fly.io, we use the FLY_API_TOKEN to manage secrets
		// The secret name will be in the format: YNAB_TOKEN_USER_1
		secretName := fmt.Sprintf("%s_USER_%s", strings.ToUpper(string(secretType)), userID)

		// We're setting the secret through the Fly.io API
		// In production, you'd need to authenticate with FLY_API_TOKEN
		// For now, just log what would happen
		log.Printf("Would store '%s' secret for user %s on Fly.io as '%s'", secretType, userID, secretName)

		// When actually implementing with Fly.io, you would use their API:
		// https://fly.io/docs/reference/secrets/#setting-secrets
	}

	// Regardless of Fly.io, we'll store a reference in the database
	// For Fly.io, we only store a reference. For local, we store the actual encrypted value
	hashedValue := value
	if os.Getenv("FLY_APP_NAME") != "" {
		// In prod, just store a placeholder since the real value is in Fly.io secrets
		hashedValue = "[stored in fly.io secrets]"
	} else {
		// In local development, hash or encrypt the token here
		// This is a simplified placeholder - use proper encryption in production
		hashedValue = fmt.Sprintf("enc:%s", value)
	}

	// Store in database with last updated time
	_, err := database.DB.Exec(`
		INSERT INTO user_ynab_settings (user_id, token, budget_id, account_id, sync_enabled, last_synced)
		VALUES (?, ?, ?, ?, ?, ?)
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
		SELECT token FROM user_ynab_settings WHERE user_id = ?
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
