package services

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"bennwallet/backend/database"
	"bennwallet/backend/models"
)

// CreateSavedFilter creates a new saved filter
func CreateSavedFilter(userID, name, resourceType, filterConfig string, isDefault bool) (*models.SavedFilter, error) {
	// Validate the filter config JSON
	var configMap map[string]interface{}
	if err := json.Unmarshal([]byte(filterConfig), &configMap); err != nil {
		return nil, fmt.Errorf("invalid filter configuration JSON: %w", err)
	}

	// Generate a random ID
	id, err := generateID()
	if err != nil {
		return nil, err
	}

	// Set the timestamp
	now := time.Now()

	// If isDefault is true, unset other default filters for this user and resource type
	if isDefault {
		_, err = database.DB.Exec(`
			UPDATE saved_filters 
			SET is_default = 0 
			WHERE user_id = ? AND resource_type = ?
		`, userID, resourceType)
		if err != nil {
			return nil, fmt.Errorf("failed to update existing default filters: %w", err)
		}
	}

	// Insert the new filter
	_, err = database.DB.Exec(`
		INSERT INTO saved_filters (id, name, user_id, resource_type, filter_config, is_default, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, id, name, userID, resourceType, filterConfig, isDefault, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to insert saved filter: %w", err)
	}

	// Return the created filter
	filter := &models.SavedFilter{
		ID:           id,
		Name:         name,
		UserID:       userID,
		ResourceType: resourceType,
		FilterConfig: filterConfig,
		IsDefault:    isDefault,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	return filter, nil
}

// GetSavedFilters retrieves all saved filters for a user
func GetSavedFilters(userID, resourceType string) ([]models.SavedFilter, error) {
	// Query for filters
	rows, err := database.DB.Query(`
		SELECT id, name, user_id, resource_type, filter_config, is_default, created_at, updated_at
		FROM saved_filters
		WHERE user_id = ? AND resource_type = ?
	`, userID, resourceType)
	if err != nil {
		return nil, fmt.Errorf("failed to query saved filters: %w", err)
	}
	defer rows.Close()

	// Parse the results
	var filters []models.SavedFilter
	for rows.Next() {
		var filter models.SavedFilter
		err := rows.Scan(
			&filter.ID,
			&filter.Name,
			&filter.UserID,
			&filter.ResourceType,
			&filter.FilterConfig,
			&filter.IsDefault,
			&filter.CreatedAt,
			&filter.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan saved filter: %w", err)
		}
		filters = append(filters, filter)
	}

	return filters, nil
}

// GetSavedFilterByID retrieves a saved filter by ID
func GetSavedFilterByID(id string) (*models.SavedFilter, error) {
	var filter models.SavedFilter
	err := database.DB.QueryRow(`
		SELECT id, name, user_id, resource_type, filter_config, is_default, created_at, updated_at
		FROM saved_filters
		WHERE id = ?
	`, id).Scan(
		&filter.ID,
		&filter.Name,
		&filter.UserID,
		&filter.ResourceType,
		&filter.FilterConfig,
		&filter.IsDefault,
		&filter.CreatedAt,
		&filter.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("saved filter not found")
		}
		return nil, fmt.Errorf("failed to query saved filter: %w", err)
	}

	return &filter, nil
}

// GetDefaultFilter retrieves the default filter for a user and resource type
func GetDefaultFilter(userID, resourceType string) (*models.SavedFilter, error) {
	var filter models.SavedFilter
	err := database.DB.QueryRow(`
		SELECT id, name, user_id, resource_type, filter_config, is_default, created_at, updated_at
		FROM saved_filters
		WHERE user_id = ? AND resource_type = ? AND is_default = 1
	`, userID, resourceType).Scan(
		&filter.ID,
		&filter.Name,
		&filter.UserID,
		&filter.ResourceType,
		&filter.FilterConfig,
		&filter.IsDefault,
		&filter.CreatedAt,
		&filter.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No default filter found
		}
		return nil, fmt.Errorf("failed to query default filter: %w", err)
	}

	return &filter, nil
}

// UpdateSavedFilter updates an existing saved filter
func UpdateSavedFilter(id, name, filterConfig string, isDefault bool) (*models.SavedFilter, error) {
	// Get the existing filter to ensure it exists and to get its user ID and resource type
	filter, err := GetSavedFilterByID(id)
	if err != nil {
		return nil, err
	}

	// Validate the filter config JSON
	var configMap map[string]interface{}
	if err := json.Unmarshal([]byte(filterConfig), &configMap); err != nil {
		return nil, fmt.Errorf("invalid filter configuration JSON: %w", err)
	}

	// Set the updated timestamp
	now := time.Now()

	// If isDefault is true, unset other default filters for this user and resource type
	if isDefault {
		_, err = database.DB.Exec(`
			UPDATE saved_filters 
			SET is_default = 0 
			WHERE user_id = ? AND resource_type = ? AND id != ?
		`, filter.UserID, filter.ResourceType, id)
		if err != nil {
			return nil, fmt.Errorf("failed to update existing default filters: %w", err)
		}
	}

	// Update the filter
	_, err = database.DB.Exec(`
		UPDATE saved_filters 
		SET name = ?, filter_config = ?, is_default = ?, updated_at = ?
		WHERE id = ?
	`, name, filterConfig, isDefault, now, id)
	if err != nil {
		return nil, fmt.Errorf("failed to update saved filter: %w", err)
	}

	// Update the filter object
	filter.Name = name
	filter.FilterConfig = filterConfig
	filter.IsDefault = isDefault
	filter.UpdatedAt = now

	return filter, nil
}

// DeleteSavedFilter deletes a saved filter
func DeleteSavedFilter(id string) error {
	// Delete the filter
	result, err := database.DB.Exec(`
		DELETE FROM saved_filters 
		WHERE id = ?
	`, id)
	if err != nil {
		return fmt.Errorf("failed to delete saved filter: %w", err)
	}

	// Check if the filter existed
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("saved filter not found")
	}

	return nil
}

// Helper function to generate a random ID
func generateID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random ID: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}
