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

	rows, err := database.DB.Query("SELECT id, name, description, color FROM categories WHERE user_id = ? ORDER BY name", userId)
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

	result, err := database.DB.Exec(`
		INSERT INTO categories (name, description, user_id, color)
		VALUES (?, ?, ?, ?)
	`, c.Name, c.Description, c.UserID, c.Color)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	c.ID = int(id)
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
		SET name = ?, description = ?, color = ?
		WHERE id = ? AND user_id = ?
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

	_, err := database.DB.Exec("DELETE FROM categories WHERE id = ? AND user_id = ?", id, userId)
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
