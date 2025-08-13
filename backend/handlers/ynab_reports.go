package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"bennwallet/backend/database"
	"bennwallet/backend/middleware"
	"bennwallet/backend/models"
	"bennwallet/backend/services"
)

func GetYNABSplits(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	var request models.ReportFilter
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	hasOptionalColumn := checkColumnExists("transactions", "optional")
	hasUserIdColumn := checkColumnExists("transactions", "user_id")

	var results []models.CategoryTotal
	processCategoryRelationships(w, r, request, userID, hasOptionalColumn, hasUserIdColumn, &results)

	includeSettlements := r.URL.Query().Get("includeSettlements") == "true"
	if includeSettlements {
		response := struct {
			CategoryTotals []models.CategoryTotal   `json:"categoryTotals"`
			SettlementData *models.SettlementReport `json:"settlementData,omitempty"`
		}{
			CategoryTotals: results,
		}

		if request.TransactionDateMonth != nil && request.TransactionDateYear != nil {
			month := fmt.Sprintf("%d-%02d", *request.TransactionDateYear, *request.TransactionDateMonth)
			settlementService := services.NewSettlementService(database.DB)
			settlementData, err := settlementService.GetSettlementReportData(userID, month, request.Paid != nil && *request.Paid)
			if err != nil {
				log.Printf("Error getting settlement data: %v", err)
			} else {
				response.SettlementData = settlementData
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func checkColumnExists(tableName, columnName string) bool {
	var exists bool
	err := database.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 
			FROM information_schema.columns 
			WHERE table_name = $1 AND column_name = $2
		)
	`, tableName, columnName).Scan(&exists)
	if err != nil {
		log.Printf("Error checking for %s column: %v", columnName, err)
		return false
	}
	return exists
}

func processCategoryRelationships(w http.ResponseWriter, r *http.Request, request models.ReportFilter,
	userID string, hasOptionalColumn, hasUserIdColumn bool, results *[]models.CategoryTotal) {

	query := `
		SELECT c.name as category, SUM(tc.amount) as total
		FROM transaction_categories tc
		JOIN ynab_categories c ON tc.category_id = c.id
		JOIN transactions t ON tc.transaction_id = t.id
		WHERE 1=1
	`
	var args []interface{}

	isAdmin, err := services.IsAdmin(userID)
	if err != nil {
		log.Printf("Error checking admin status: %v", err)
		isAdmin = false
	}

	if !isAdmin {
		query += " AND (c.user_id = $1 OR c.user_id IS NULL)"
		args = append(args, userID)
	}

	if request.TransactionDateMonth != nil && request.TransactionDateYear != nil {
		startDate := fmt.Sprintf("%d-%02d-01", *request.TransactionDateYear, *request.TransactionDateMonth)

		nextMonth := *request.TransactionDateMonth + 1
		year := *request.TransactionDateYear
		if nextMonth > 12 {
			nextMonth = 1
			year++
		}
		endOfMonth := time.Date(year, time.Month(nextMonth), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1)
		endDate := endOfMonth.Format("2006-01-02")

		query += fmt.Sprintf(" AND t.transaction_date::date >= $%d::date AND t.transaction_date::date <= $%d::date", len(args)+1, len(args)+2)
		args = append(args, startDate, endDate)
	} else {
		if request.StartDate != "" {
			query += fmt.Sprintf(" AND t.transaction_date::date >= $%d::date", len(args)+1)
			args = append(args, request.StartDate)
		}
		if request.EndDate != "" {
			query += fmt.Sprintf(" AND t.transaction_date::date <= $%d::date", len(args)+1)
			args = append(args, request.EndDate)
		}
	}

	if request.Category != "" {
		query += fmt.Sprintf(" AND c.name = $%d", len(args)+1)
		args = append(args, request.Category)
	}

	if request.PayTo != "" {
		query += fmt.Sprintf(` AND (
			t.owed_by ILIKE $%d OR 
			t.owed_by ILIKE $%d OR
			t.owed_by = $%d
		)`, len(args)+1, len(args)+2, len(args)+3)
		args = append(args, request.PayTo, "%"+request.PayTo+"%", request.PayTo)
	}

	if request.EnteredBy != "" {
		matchedUserIds, matchedUserNames := getUserMatches(request.EnteredBy)

		paidByConditions := make([]string, 0)
		for _, id := range matchedUserIds {
			paidByConditions = append(paidByConditions, fmt.Sprintf("t.paid_by = $%d", len(args)+1))
			args = append(args, id)
		}
		for _, name := range matchedUserNames {
			paidByConditions = append(paidByConditions, fmt.Sprintf("t.paid_by ILIKE $%d", len(args)+1))
			args = append(args, "%"+name+"%")
		}
		query += fmt.Sprintf(" AND (%s)", strings.Join(paidByConditions, " OR "))
	}

	if request.Paid != nil {
		query += fmt.Sprintf(" AND t.paid = $%d", len(args)+1)
		args = append(args, *request.Paid)
	}

	if hasOptionalColumn && request.Optional != nil && *request.Optional {
		query += fmt.Sprintf(" AND t.optional = $%d", len(args)+1)
		args = append(args, false)
	}

	if hasUserIdColumn && !isAdmin {
		accessibleUsers, err := middleware.GetUserAccessibleResources(userID, models.ResourceTransactions, models.PermissionRead)
		if err != nil {
			log.Printf("Error getting accessible resources: %v", err)
		} else if len(accessibleUsers) > 0 {
			placeholders := make([]string, len(accessibleUsers))
			for i := range accessibleUsers {
				placeholders[i] = fmt.Sprintf("$%d", len(args)+1)
				args = append(args, accessibleUsers[i])
			}
			query += fmt.Sprintf(" AND (t.user_id IN (%s) OR t.user_id IS NULL)", strings.Join(placeholders, ","))
		}
	}

	query += " GROUP BY c.name ORDER BY total DESC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		log.Printf("Error querying category relationships: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var category string
		var total float64
		if err := rows.Scan(&category, &total); err != nil {
			log.Printf("Error scanning category relationship row: %v", err)
			continue
		}
		*results = append(*results, models.CategoryTotal{
			Category: category,
			Total:    total,
		})
	}
}

func getUserMatches(filter string) ([]interface{}, []string) {
	rows, err := database.DB.Query(`
		SELECT id, name, username FROM users 
		WHERE id = $1 
		   OR name ILIKE $2 
		   OR name ILIKE $3
		   OR username ILIKE $4
		   OR username ILIKE $5
	`, filter, filter, "%"+filter+"%", filter, "%"+filter+"%")

	matchedUserIds := []interface{}{filter}
	matchedUserNames := []string{filter}

	if err != nil {
		log.Printf("Error querying users table: %v", err)
		return matchedUserIds, matchedUserNames
	}
	defer rows.Close()

	for rows.Next() {
		var id, name, username string
		if err := rows.Scan(&id, &name, &username); err == nil {
			matchedUserIds = append(matchedUserIds, id)
			matchedUserNames = append(matchedUserNames, name)
			if username != "" && username != name {
				matchedUserNames = append(matchedUserNames, username)
			}
		}
	}

	return matchedUserIds, matchedUserNames
}
