package security

import (
	"log"
	"strings"
)

// LogYNABAPIRequest logs details about a YNAB API request for debugging
// It masks sensitive information like full tokens
func LogYNABAPIRequest(url string, token string, budgetID string, accountID string) {
	log.Printf("YNAB API Request Debug Info:")
	log.Printf("- URL: %s", url)

	// Log partial token (first 8 chars) for identification but mask the rest
	if token != "" {
		if len(token) > 8 {
			log.Printf("- Token: %s... (length: %d)", token[:8], len(token))
		} else {
			log.Printf("- Token: [TOO SHORT - POSSIBLE ERROR] (length: %d)", len(token))
		}
	} else {
		log.Printf("- Token: [EMPTY]")
	}

	// Log budget and account IDs
	log.Printf("- Budget ID: %s (length: %d)", budgetID, len(budgetID))
	if accountID != "" {
		log.Printf("- Account ID: %s (length: %d)", accountID, len(accountID))
	}

	// Check for common formatting issues
	if strings.Contains(budgetID, " ") {
		log.Printf("- WARNING: Budget ID contains spaces!")
	}
	if accountID != "" && strings.Contains(accountID, " ") {
		log.Printf("- WARNING: Account ID contains spaces!")
	}
}
