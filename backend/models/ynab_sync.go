package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
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
	// Get user's YNAB configuration - it's okay to get the token from here for now
	var token, budgetID, accountID string
	err := database.DB.QueryRow(
		"SELECT token, budget_id, account_id FROM user_ynab_settings WHERE user_id = ?",
		request.UserID,
	).Scan(&token, &budgetID, &accountID)

	if err != nil {
		return fmt.Errorf("error getting YNAB settings: %w", err)
	}

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
	for _, split := range request.Categories {
		// Get category ID
		var categoryID string
		err := database.DB.QueryRow(
			"SELECT id FROM ynab_categories WHERE user_id = ? AND name = ?",
			request.UserID, split.CategoryName,
		).Scan(&categoryID)

		if err != nil {
			return fmt.Errorf("error finding category '%s': %w", split.CategoryName, err)
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

	// Create request to YNAB API
	url := fmt.Sprintf("https://api.ynab.com/v1/budgets/%s/transactions", budgetID)
	payload := struct {
		Transaction YNABTransaction `json:"transaction"`
	}{
		Transaction: transaction,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshaling transaction: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending transaction to YNAB: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("YNAB API error: %s (%d)", string(body), resp.StatusCode)
	}

	log.Printf("Successfully created transaction in YNAB for user %s", request.UserID)
	return nil
}
