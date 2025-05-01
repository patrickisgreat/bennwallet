package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"bennwallet/backend/database"
)

// YNABSyncRequest represents a request to sync transaction data to YNAB
type YNABSyncRequest struct {
	UserID     string          `json:"userId"`
	Date       string          `json:"date"`
	PayeeName  string          `json:"payeeName"`
	Memo       string          `json:"memo"`
	Categories []CategorySplit `json:"categories"`
}

// CategorySplit represents a category split in a YNAB transaction
type CategorySplit struct {
	CategoryName string  `json:"categoryName"`
	Amount       float64 `json:"amount"`
}

// YNABTransaction represents a transaction to create in YNAB
type YNABTransaction struct {
	AccountID       string               `json:"account_id"`
	Date            string               `json:"date"`
	Amount          int64                `json:"amount"` // In milliunits (1/1000)
	PayeeID         string               `json:"payee_id,omitempty"`
	PayeeName       string               `json:"payee_name,omitempty"`
	Memo            string               `json:"memo,omitempty"`
	Cleared         string               `json:"cleared"`
	Approved        bool                 `json:"approved"`
	FlagColor       string               `json:"flag_color,omitempty"`
	Subtransactions []YNABSubtransaction `json:"subtransactions,omitempty"`
}

// YNABSubtransaction represents a subtransaction for split categories
type YNABSubtransaction struct {
	Amount     int64  `json:"amount"` // In milliunits (1/1000)
	CategoryID string `json:"category_id,omitempty"`
	Memo       string `json:"memo,omitempty"`
	PayeeID    string `json:"payee_id,omitempty"`
	PayeeName  string `json:"payee_name,omitempty"`
}

// CreateYNABTransaction sends a transaction to YNAB
func CreateYNABTransaction(request YNABSyncRequest) error {
	log.Printf("Starting YNAB transaction creation for user %s with %d categories",
		request.UserID, len(request.Categories))

	// Get user's YNAB configuration
	var dbToken, budgetID, accountID string
	err := database.DB.QueryRow(
		"SELECT token, budget_id, account_id FROM user_ynab_settings WHERE user_id = ?",
		request.UserID,
	).Scan(&dbToken, &budgetID, &accountID)

	if err != nil {
		log.Printf("Error getting YNAB settings for user %s: %v", request.UserID, err)
		return fmt.Errorf("error getting YNAB settings: %w", err)
	}

	// Check if token is stored in environment variables
	token := dbToken
	if strings.HasPrefix(dbToken, "enc:") {
		// For local dev, token is prefixed in DB
		token = strings.TrimPrefix(dbToken, "enc:")
		log.Printf("Using locally stored token for user %s", request.UserID)
	} else if dbToken == "[stored in environment variables]" {
		// For production, get token from environment
		envToken := os.Getenv(fmt.Sprintf("YNAB_TOKEN_USER_%s", request.UserID))
		if envToken == "" {
			// Fallback to default token
			envToken = os.Getenv("YNAB_TOKEN")
		}

		if envToken == "" {
			log.Printf("Error: No YNAB token found for user %s in env vars", request.UserID)
			return fmt.Errorf("no YNAB token found in environment variables")
		}

		token = envToken
		log.Printf("Using token from environment variables for user %s", request.UserID)
	}

	log.Printf("Found YNAB settings for user %s: budget=%s, account=%s",
		request.UserID, budgetID, accountID)

	// Convert to YNAB transaction
	transaction := YNABTransaction{
		AccountID: accountID,
		Date:      request.Date,
		PayeeName: request.PayeeName,
		Memo:      request.Memo,
		Cleared:   "cleared",
		Approved:  false,
	}

	// Setup subtransactions for each category split
	var totalAmount int64
	log.Printf("Processing %d category splits", len(request.Categories))

	for _, split := range request.Categories {
		log.Printf("Looking up category: '%s'", split.CategoryName)

		// Get category ID
		var categoryID string
		err := database.DB.QueryRow(
			"SELECT id FROM ynab_categories WHERE user_id = ? AND name = ?",
			request.UserID, split.CategoryName,
		).Scan(&categoryID)

		if err != nil {
			log.Printf("Error finding category '%s' for user %s: %v",
				split.CategoryName, request.UserID, err)

			// Attempt a more permissive search by using LIKE instead of exact match
			err = database.DB.QueryRow(
				"SELECT id FROM ynab_categories WHERE user_id = ? AND name LIKE ?",
				request.UserID, "%"+split.CategoryName+"%",
			).Scan(&categoryID)

			if err != nil {
				log.Printf("Still couldn't find category even with fuzzy search: %v", err)
				return fmt.Errorf("error finding category '%s': %w", split.CategoryName, err)
			}

			log.Printf("Found category '%s' with fuzzy match, ID: %s",
				split.CategoryName, categoryID)
		} else {
			log.Printf("Found category '%s' with exact match, ID: %s",
				split.CategoryName, categoryID)
		}

		// Convert dollar amount to milliunits (YNAB uses integer)
		amountMilliunits := int64(split.Amount * 1000)
		totalAmount += amountMilliunits

		// Add subtransaction
		transaction.Subtransactions = append(transaction.Subtransactions, YNABSubtransaction{
			Amount:     amountMilliunits,
			CategoryID: categoryID,
			Memo:       fmt.Sprintf("Imported from BennWallet"),
		})
	}

	// Set total transaction amount
	transaction.Amount = totalAmount
	log.Printf("Total transaction amount: %d milliunits", totalAmount)

	// Create request to YNAB API
	url := fmt.Sprintf("https://api.ynab.com/v1/budgets/%s/transactions", budgetID)
	payload := struct {
		Transaction YNABTransaction `json:"transaction"`
	}{
		Transaction: transaction,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling transaction: %v", err)
		return fmt.Errorf("error marshaling transaction: %w", err)
	}

	log.Printf("Sending transaction to YNAB API with URL: %s", url)
	// Don't log the full payload as it contains sensitive data

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error sending transaction to YNAB: %v", err)
		return fmt.Errorf("error sending transaction to YNAB: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		log.Printf("YNAB API error: %s (%d)", string(body), resp.StatusCode)

		// If 401 Unauthorized, provide a more specific error
		if resp.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("YNAB API unauthorized: token may be invalid or expired (%d)", resp.StatusCode)
		}

		return fmt.Errorf("YNAB API error: %s (%d)", string(body), resp.StatusCode)
	}

	log.Printf("Successfully created transaction in YNAB for user %s. Response: %s",
		request.UserID, string(body))
	return nil
}
