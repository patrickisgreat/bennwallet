package handlers

import (
	"encoding/json"
	"net/http"

	"bennwallet/backend/middleware"
	"bennwallet/backend/models"
	"bennwallet/backend/services"

	"github.com/gorilla/mux"
)

// GetCustomReports returns all custom reports for the current user
func GetCustomReports(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	// Check if to include all accessible reports or just user's own reports
	includeAccessible := r.URL.Query().Get("includeAccessible") == "true"

	var reports []models.CustomReport
	var err error

	if includeAccessible {
		// Get all reports accessible to user (own reports + public reports)
		reports, err = services.GetAccessibleCustomReports(userID)
	} else {
		// Get only user's own reports
		reports, err = services.GetUserCustomReports(userID)
	}

	if err != nil {
		http.Error(w, "Failed to get custom reports: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reports)
}

// GetCustomReport returns a specific custom report
func GetCustomReport(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	// Get report ID from URL parameter
	vars := mux.Vars(r)
	reportID := vars["id"]
	if reportID == "" {
		http.Error(w, "Report ID is required", http.StatusBadRequest)
		return
	}

	// Get the report
	report, err := services.GetCustomReportByID(reportID)
	if err != nil {
		http.Error(w, "Failed to get custom report: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Check if the user can access this report
	if report.UserID != userID && !report.IsPublic {
		isAdmin, err := services.IsAdmin(userID)
		if err != nil {
			http.Error(w, "Failed to check admin status: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if !isAdmin {
			http.Error(w, "Forbidden: You do not have permission to access this report", http.StatusForbidden)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}

// CreateCustomReport creates a new custom report
func CreateCustomReport(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	// Parse the request body
	var request struct {
		Name         string `json:"name"`
		Description  string `json:"description"`
		ReportConfig string `json:"reportConfig"`
		IsPublic     bool   `json:"isPublic"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate the request
	if request.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	if request.ReportConfig == "" {
		http.Error(w, "reportConfig is required", http.StatusBadRequest)
		return
	}

	// Create the custom report
	report, err := services.CreateCustomReport(userID, request.Name, request.Description, request.ReportConfig, request.IsPublic)
	if err != nil {
		http.Error(w, "Failed to create custom report: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(report)
}

// UpdateCustomReport updates an existing custom report
func UpdateCustomReport(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	// Get report ID from URL parameter
	vars := mux.Vars(r)
	reportID := vars["id"]
	if reportID == "" {
		http.Error(w, "Report ID is required", http.StatusBadRequest)
		return
	}

	// Check if the user owns the report
	report, err := services.GetCustomReportByID(reportID)
	if err != nil {
		http.Error(w, "Failed to get custom report: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if report.UserID != userID {
		isAdmin, err := services.IsAdmin(userID)
		if err != nil {
			http.Error(w, "Failed to check admin status: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if !isAdmin {
			http.Error(w, "Forbidden: You do not have permission to update this report", http.StatusForbidden)
			return
		}
	}

	// Parse the request body
	var request struct {
		Name         string `json:"name"`
		Description  string `json:"description"`
		ReportConfig string `json:"reportConfig"`
		IsPublic     bool   `json:"isPublic"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate the request
	if request.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	if request.ReportConfig == "" {
		http.Error(w, "reportConfig is required", http.StatusBadRequest)
		return
	}

	// Update the custom report
	updatedReport, err := services.UpdateCustomReport(reportID, request.Name, request.Description, request.ReportConfig, request.IsPublic)
	if err != nil {
		http.Error(w, "Failed to update custom report: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedReport)
}

// DeleteCustomReport deletes a custom report
func DeleteCustomReport(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	// Get report ID from URL parameter
	vars := mux.Vars(r)
	reportID := vars["id"]
	if reportID == "" {
		http.Error(w, "Report ID is required", http.StatusBadRequest)
		return
	}

	// Check if the user owns the report
	report, err := services.GetCustomReportByID(reportID)
	if err != nil {
		http.Error(w, "Failed to get custom report: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if report.UserID != userID {
		isAdmin, err := services.IsAdmin(userID)
		if err != nil {
			http.Error(w, "Failed to check admin status: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if !isAdmin {
			http.Error(w, "Forbidden: You do not have permission to delete this report", http.StatusForbidden)
			return
		}
	}

	// Delete the custom report
	err = services.DeleteCustomReport(reportID)
	if err != nil {
		http.Error(w, "Failed to delete custom report: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// RunCustomReport runs a custom report and returns the results
func RunCustomReport(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	// Get report ID from URL parameter
	vars := mux.Vars(r)
	reportID := vars["id"]
	if reportID == "" {
		http.Error(w, "Report ID is required", http.StatusBadRequest)
		return
	}

	// Get the report
	report, err := services.GetCustomReportByID(reportID)
	if err != nil {
		http.Error(w, "Failed to get custom report: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Check if the user can access this report
	if report.UserID != userID && !report.IsPublic {
		isAdmin, err := services.IsAdmin(userID)
		if err != nil {
			http.Error(w, "Failed to check admin status: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if !isAdmin {
			http.Error(w, "Forbidden: You do not have permission to run this report", http.StatusForbidden)
			return
		}
	}

	// TODO: Implement report execution logic based on the report configuration
	// This would involve parsing the report config, running the appropriate queries,
	// and formatting the results

	// For now, just return the report configuration
	result := map[string]interface{}{
		"report":  report,
		"results": map[string]string{"status": "Report execution not yet implemented"},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
