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

	// First check if we should return hierarchical structure
	hierarchical := r.URL.Query().Get("hierarchical")
	if hierarchical == "true" {
		getCategoriesHierarchical(w, r, userId)
		return
	}

	// Original flat list for backward compatibility
	rows, err := database.DB.Query(`
		SELECT c.id, c.name, c.name as description, '#3498DB' as color 
		FROM ynab_categories c
		WHERE c.user_id = $1 AND c.hidden = false
		ORDER BY c.name
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

// getCategoriesHierarchical returns categories grouped by their category groups
func getCategoriesHierarchical(w http.ResponseWriter, r *http.Request, userId string) {
	log.Printf("Getting hierarchical categories for user ID: %s", userId)

	// Get category groups first
	groupRows, err := database.DB.Query(`
		SELECT id, name 
		FROM ynab_category_groups 
		WHERE user_id = $1 
		ORDER BY name
	`, userId)
	if err != nil {
		log.Printf("Error querying ynab_category_groups: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer groupRows.Close()

	type CategoryGroup struct {
		ID         string                `json:"id"`
		Name       string                `json:"name"`
		Categories []models.YNABCategory `json:"categories"`
	}

	var groups []CategoryGroup
	for groupRows.Next() {
		var group CategoryGroup
		err := groupRows.Scan(&group.ID, &group.Name)
		if err != nil {
			log.Printf("Error scanning group: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		groups = append(groups, group)
	}

	// Get categories for each group
	for i, group := range groups {
		catRows, err := database.DB.Query(`
			SELECT id, name
			FROM ynab_categories
			WHERE user_id = $1 AND (group_id = $2 OR category_group_id = $2) AND hidden = false
			ORDER BY name
		`, userId, group.ID)
		if err != nil {
			log.Printf("Error querying categories for group %s: %v", group.ID, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var categories []models.YNABCategory
		for catRows.Next() {
			var cat models.YNABCategory
			err := catRows.Scan(&cat.ID, &cat.Name)
			if err != nil {
				catRows.Close()
				log.Printf("Error scanning category: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			cat.CategoryGroupID = group.ID
			cat.CategoryGroupName = group.Name
			categories = append(categories, cat)
		}
		catRows.Close()

		groups[i].Categories = categories
	}

	log.Printf("Returning %d category groups with categories for user %s", len(groups), userId)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(groups)
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
