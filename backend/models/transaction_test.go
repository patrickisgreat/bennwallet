package models

import (
	"testing"
	"time"
)

func TestTransaction(t *testing.T) {
	// Create test dates
	now := time.Now()
	txDate := now.Add(-24 * time.Hour) // Yesterday

	// Format for PaidDate
	paidDateStr := now.Format("2006-01-02")

	// Create a test transaction
	tx := Transaction{
		ID:              "tx-id",
		Amount:          123.45,
		Description:     "Test transaction",
		Date:            now,
		TransactionDate: txDate,
		Type:            "Food",
		PayTo:           "Sarah",
		Paid:            true,
		PaidDate:        paidDateStr,
		EnteredBy:       "Patrick",
		Optional:        false,
		UserID:          "user-id",
	}

	// Test ID
	if tx.ID != "tx-id" {
		t.Errorf("Expected ID 'tx-id', got '%s'", tx.ID)
	}

	// Test Amount
	if tx.Amount != 123.45 {
		t.Errorf("Expected Amount 123.45, got %f", tx.Amount)
	}

	// Test Description
	if tx.Description != "Test transaction" {
		t.Errorf("Expected Description 'Test transaction', got '%s'", tx.Description)
	}

	// Test Date
	if !tx.Date.Equal(now) {
		t.Errorf("Expected Date %v, got %v", now, tx.Date)
	}

	// Test TransactionDate
	if !tx.TransactionDate.Equal(txDate) {
		t.Errorf("Expected TransactionDate %v, got %v", txDate, tx.TransactionDate)
	}

	// Test Type
	if tx.Type != "Food" {
		t.Errorf("Expected Type 'Food', got '%s'", tx.Type)
	}

	// Test PayTo
	if tx.PayTo != "Sarah" {
		t.Errorf("Expected PayTo 'Sarah', got '%s'", tx.PayTo)
	}

	// Test Paid
	if !tx.Paid {
		t.Errorf("Expected Paid true, got %v", tx.Paid)
	}

	// Test PaidDate
	if tx.PaidDate != paidDateStr {
		t.Errorf("Expected PaidDate '%s', got '%s'", paidDateStr, tx.PaidDate)
	}

	// Test EnteredBy
	if tx.EnteredBy != "Patrick" {
		t.Errorf("Expected EnteredBy 'Patrick', got '%s'", tx.EnteredBy)
	}

	// Test Optional
	if tx.Optional {
		t.Errorf("Expected Optional false, got %v", tx.Optional)
	}

	// Test UserID
	if tx.UserID != "user-id" {
		t.Errorf("Expected UserID 'user-id', got '%s'", tx.UserID)
	}
}

func TestTransactionOptionalFields(t *testing.T) {
	// Test with missing optional fields
	tx := Transaction{
		ID:              "tx-id",
		Amount:          123.45,
		Description:     "Test transaction",
		Date:            time.Now(),
		TransactionDate: time.Now(),
		Type:            "Food",
		Paid:            false,
		EnteredBy:       "Patrick",
		Optional:        false,
	}

	// PayTo is optional
	if tx.PayTo != "" {
		t.Errorf("Expected PayTo to be empty, got '%s'", tx.PayTo)
	}

	// PaidDate is optional
	if tx.PaidDate != "" {
		t.Errorf("Expected PaidDate to be empty, got '%s'", tx.PaidDate)
	}

	// UserID is optional
	if tx.UserID != "" {
		t.Errorf("Expected UserID to be empty, got '%s'", tx.UserID)
	}
}
