package handlers

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"strconv"

	"bennwallet/backend/database"
	"bennwallet/backend/middleware"
	"bennwallet/backend/models"

	"github.com/gorilla/mux"
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

	// Changed from ? to $1 for PostgreSQL compatibility
	rows, err := database.DB.Query("SELECT id, name, description, color FROM categories WHERE user_id = $1 ORDER BY name", userId)
	if err != nil {
		log.Printf("Error querying categories: %v", err)
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

func AddCategory(w http.ResponseWriter, r *http.Request) {
	// Get user ID from authentication context
	userId := middleware.GetUserIDFromContext(r)
	if userId == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	var c models.Category
	err := json.NewDecoder(r.Body).Decode(&c)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Set the user ID from the authentication context
	c.UserID = userId

	// Generate a random color if not provided
	if c.Color == "" {
		c.Color = generateRandomColor()
	}

	// Use RETURNING clause to get the inserted ID with PostgreSQL
	err = database.DB.QueryRow(`
		INSERT INTO categories (name, description, user_id, color)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, c.Name, c.Description, c.UserID, c.Color).Scan(&c.ID)

	if err != nil {
		log.Printf("Error inserting category: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(c)
}

func UpdateCategory(w http.ResponseWriter, r *http.Request) {
	// Get user ID from authentication context
	userId := middleware.GetUserIDFromContext(r)
	if userId == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	var c models.Category
	err := json.NewDecoder(r.Body).Decode(&c)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Use the user ID from the authentication context
	_, err = database.DB.Exec(`
		UPDATE categories 
		SET name = $1, description = $2, color = $3
		WHERE id = $4 AND user_id = $5
	`, c.Name, c.Description, c.Color, id, userId)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return the updated category
	c.ID, _ = strconv.Atoi(id) // Convert id to int
	c.UserID = userId
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(c)
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

	vars := mux.Vars(r)
	id := vars["id"]

	_, err := database.DB.Exec("DELETE FROM categories WHERE id = $1 AND user_id = $2", id, userId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func generateRandomColor() string {
	colors := []string{
		"#FF6B6B", "#4ECDC4", "#45B7D1", "#96CEB4", "#FFEEAD",
		"#D4A5A5", "#9B59B6", "#3498DB", "#1ABC9C", "#F1C40F",
	}
	return colors[rand.Intn(len(colors))]
}
