package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"bennwallet/backend/database"
	"bennwallet/backend/models"

	"github.com/gorilla/mux"
)

func GetUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query("SELECT id, username, name FROM users")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		err := rows.Scan(&u.ID, &u.Username, &u.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		users = append(users, u)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func GetUserByUsername(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	username := vars["username"]

	var user models.User
	err := database.DB.QueryRow("SELECT id, username, name FROM users WHERE username = ?", username).Scan(&user.ID, &user.Username, &user.Name)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// SyncFirebaseUser syncs a Firebase user with the backend database
// This ensures that Firebase users exist in our users table
func SyncFirebaseUser(w http.ResponseWriter, r *http.Request) {
	var request struct {
		FirebaseID string `json:"firebaseId"`
		Name       string `json:"name"`
		Email      string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if request.FirebaseID == "" {
		http.Error(w, "firebaseId is required", http.StatusBadRequest)
		return
	}

	// Check if user already exists
	var userID string
	err := database.DB.QueryRow("SELECT id FROM users WHERE id = ?", request.FirebaseID).Scan(&userID)

	if err == sql.ErrNoRows {
		// User doesn't exist, create a new one
		_, err = database.DB.Exec(
			"INSERT INTO users (id, username, name) VALUES (?, ?, ?)",
			request.FirebaseID,
			request.Email,
			request.Name,
		)

		if err != nil {
			http.Error(w, "Failed to create user: "+err.Error(), http.StatusInternalServerError)
			return
		}

		userID = request.FirebaseID
	} else if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return success with user ID
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"id": userID,
	})
}
