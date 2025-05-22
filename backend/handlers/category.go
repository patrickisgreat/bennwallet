package handlers

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"

	"bennwallet/backend/database"
	"bennwallet/backend/middleware"
	"bennwallet/backend/models"
)

func GetCategories(w http.ResponseWriter, r *http.Request) {
	// Get user ID from authentication context
	userId := middleware.GetUserIDFromContext(r)
	if userId == "" {
		// For backward compatibility, still check the query parameter
		userId = r.URL.Query().Get("userId")
		if userId == "" {
			http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
			return
		}
	}

	log.Printf("Getting categories for user ID: %s", userId)

	// Changed from categories to ynab_categories table
	rows, err := database.DB.Query(`
		SELECT id, name, name as description, '#3498DB' as color 
		FROM ynab_categories 
		WHERE user_id = $1 AND hidden = false
		ORDER BY name
	`, userId)
	if err != nil {
		log.Printf("Error querying ynab_categories: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var categories []models.Category
	for rows.Next() {
		var c models.Category
		err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.Color)
		if err != nil {
			log.Printf("Error scanning category: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Add userId to the response
		c.UserID = userId
		categories = append(categories, c)
	}

	log.Printf("Returning %d categories for user %s", len(categories), userId)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(categories)
}

func CreateCategory(w http.ResponseWriter, r *http.Request) {
	// Get user ID from authentication context
	userId := middleware.GetUserIDFromContext(r)
	if userId == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	// Since we're switching to YNAB categories which are synced from YNAB,
	// creating a category directly should be discouraged
	http.Error(w, "Creating categories directly is disabled. Please sync categories from YNAB instead.", http.StatusNotImplemented)
}

func UpdateCategory(w http.ResponseWriter, r *http.Request) {
	// Get user ID from authentication context
	userId := middleware.GetUserIDFromContext(r)
	if userId == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	// Since we're switching to YNAB categories which are synced from YNAB,
	// updating a category directly should be discouraged
	http.Error(w, "Updating categories directly is disabled. Please sync categories from YNAB instead.", http.StatusNotImplemented)

}

func DeleteCategory(w http.ResponseWriter, r *http.Request) {
	// Get user ID from authentication context
	userId := middleware.GetUserIDFromContext(r)
	if userId == "" {
		// For backward compatibility, still check the query parameter
		userId = r.URL.Query().Get("userId")
		if userId == "" {
			http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
			return
		}
	}

	// Since we're switching to YNAB categories which are synced from YNAB,
	// deleting a category directly should be discouraged
	http.Error(w, "Deleting categories directly is disabled. Please sync categories from YNAB instead.", http.StatusNotImplemented)
}

func generateRandomColor() string {
	colors := []string{
		"#FF6B6B", "#4ECDC4", "#45B7D1", "#96CEB4", "#FFEEAD",
		"#D4A5A5", "#9B59B6", "#3498DB", "#1ABC9C", "#F1C40F",
	}
	return colors[rand.Intn(len(colors))]
}
