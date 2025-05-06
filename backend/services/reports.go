package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"bennwallet/backend/database"
	"bennwallet/backend/models"
)

// CreateCustomReport creates a new custom report
func CreateCustomReport(userID, name, description, reportConfig string, isPublic bool) (*models.CustomReport, error) {
	// Validate the report config JSON
	var configMap map[string]interface{}
	if err := json.Unmarshal([]byte(reportConfig), &configMap); err != nil {
		return nil, fmt.Errorf("invalid report configuration JSON: %w", err)
	}

	// Generate a random ID
	id, err := generateID()
	if err != nil {
		return nil, err
	}

	// Set the timestamp
	now := time.Now()

	// Insert the new report
	_, err = database.DB.Exec(`
		INSERT INTO custom_reports (id, name, user_id, description, report_config, is_public, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, id, name, userID, description, reportConfig, isPublic, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to insert custom report: %w", err)
	}

	// Return the created report
	report := &models.CustomReport{
		ID:           id,
		Name:         name,
		UserID:       userID,
		Description:  description,
		ReportConfig: reportConfig,
		IsPublic:     isPublic,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	return report, nil
}

// GetCustomReportByID retrieves a custom report by ID
func GetCustomReportByID(id string) (*models.CustomReport, error) {
	var report models.CustomReport
	err := database.DB.QueryRow(`
		SELECT id, name, user_id, description, report_config, is_public, created_at, updated_at
		FROM custom_reports
		WHERE id = ?
	`, id).Scan(
		&report.ID,
		&report.Name,
		&report.UserID,
		&report.Description,
		&report.ReportConfig,
		&report.IsPublic,
		&report.CreatedAt,
		&report.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("custom report not found")
		}
		return nil, fmt.Errorf("failed to query custom report: %w", err)
	}

	return &report, nil
}

// GetUserCustomReports retrieves all custom reports for a user
func GetUserCustomReports(userID string) ([]models.CustomReport, error) {
	// Query for reports
	rows, err := database.DB.Query(`
		SELECT id, name, user_id, description, report_config, is_public, created_at, updated_at
		FROM custom_reports
		WHERE user_id = ?
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query custom reports: %w", err)
	}
	defer rows.Close()

	// Parse the results
	var reports []models.CustomReport
	for rows.Next() {
		var report models.CustomReport
		err := rows.Scan(
			&report.ID,
			&report.Name,
			&report.UserID,
			&report.Description,
			&report.ReportConfig,
			&report.IsPublic,
			&report.CreatedAt,
			&report.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan custom report: %w", err)
		}
		reports = append(reports, report)
	}

	return reports, nil
}

// GetAccessibleCustomReports retrieves all custom reports that a user can access
// Includes the user's own reports and public reports created by other users
func GetAccessibleCustomReports(userID string) ([]models.CustomReport, error) {
	// Check if the user is an admin
	isUserAdmin, err := IsAdmin(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check admin status: %w", err)
	}

	var rows *sql.Rows
	if isUserAdmin {
		// Admins can see all reports
		rows, err = database.DB.Query(`
			SELECT id, name, user_id, description, report_config, is_public, created_at, updated_at
			FROM custom_reports
		`)
	} else {
		// Regular users can see their own reports and public reports
		rows, err = database.DB.Query(`
			SELECT id, name, user_id, description, report_config, is_public, created_at, updated_at
			FROM custom_reports
			WHERE user_id = ? OR is_public = 1
		`, userID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query accessible reports: %w", err)
	}
	defer rows.Close()

	// Parse the results
	var reports []models.CustomReport
	for rows.Next() {
		var report models.CustomReport
		err := rows.Scan(
			&report.ID,
			&report.Name,
			&report.UserID,
			&report.Description,
			&report.ReportConfig,
			&report.IsPublic,
			&report.CreatedAt,
			&report.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan custom report: %w", err)
		}
		reports = append(reports, report)
	}

	return reports, nil
}

// UpdateCustomReport updates an existing custom report
func UpdateCustomReport(id, name, description, reportConfig string, isPublic bool) (*models.CustomReport, error) {
	// Get the existing report to ensure it exists
	report, err := GetCustomReportByID(id)
	if err != nil {
		return nil, err
	}

	// Validate the report config JSON
	var configMap map[string]interface{}
	if err := json.Unmarshal([]byte(reportConfig), &configMap); err != nil {
		return nil, fmt.Errorf("invalid report configuration JSON: %w", err)
	}

	// Set the updated timestamp
	now := time.Now()

	// Update the report
	_, err = database.DB.Exec(`
		UPDATE custom_reports 
		SET name = ?, description = ?, report_config = ?, is_public = ?, updated_at = ?
		WHERE id = ?
	`, name, description, reportConfig, isPublic, now, id)
	if err != nil {
		return nil, fmt.Errorf("failed to update custom report: %w", err)
	}

	// Update the report object
	report.Name = name
	report.Description = description
	report.ReportConfig = reportConfig
	report.IsPublic = isPublic
	report.UpdatedAt = now

	return report, nil
}

// DeleteCustomReport deletes a custom report
func DeleteCustomReport(id string) error {
	// Delete the report
	result, err := database.DB.Exec(`
		DELETE FROM custom_reports 
		WHERE id = ?
	`, id)
	if err != nil {
		return fmt.Errorf("failed to delete custom report: %w", err)
	}

	// Check if the report existed
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("custom report not found")
	}

	return nil
}
