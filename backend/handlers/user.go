package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"bennwallet/backend/database"
	"bennwallet/backend/middleware"
	"bennwallet/backend/models"

	"github.com/gorilla/mux"
)

func GetUsers(w http.ResponseWriter, r *http.Request) {
	// Get user ID from authentication context
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	// Check if the user is an admin
	var isAdmin bool
	err := database.DB.QueryRow("SELECT isAdmin FROM users WHERE id = ?", userID).Scan(&isAdmin)
	if err != nil {
		http.Error(w, "Failed to check user permissions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Only admins can see all users
	if !isAdmin {
		http.Error(w, "Unauthorized: Admin access required", http.StatusForbidden)
		return
	}

	// Update query to include all fields
	rows, err := database.DB.Query("SELECT id, username, name, status, isAdmin, role FROM users")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		var status, role sql.NullString
		var isAdmin sql.NullBool

		err := rows.Scan(&u.ID, &u.Username, &u.Name, &status, &isAdmin, &role)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Set default values if nulls
		if status.Valid {
			u.Status = status.String
		} else {
			u.Status = "approved" // Default status
		}

		if isAdmin.Valid {
			u.IsAdmin = isAdmin.Bool
		}

		if role.Valid {
			u.Role = role.String
		} else {
			u.Role = "user" // Default role
		}

		users = append(users, u)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func GetUserByUsername(w http.ResponseWriter, r *http.Request) {
	// Get user ID from authentication context to verify authorization
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	username := vars["username"]

	var user models.User
	var status, role sql.NullString
	var isAdmin sql.NullBool

	err := database.DB.QueryRow("SELECT id, username, name, status, isAdmin, role FROM users WHERE username = ?", username).Scan(
		&user.ID, &user.Username, &user.Name, &status, &isAdmin, &role)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Set default values if nulls
	if status.Valid {
		user.Status = status.String
	} else {
		user.Status = "approved" // Default status
	}

	if isAdmin.Valid {
		user.IsAdmin = isAdmin.Bool
	}

	if role.Valid {
		user.Role = role.String
	} else {
		user.Role = "user" // Default role
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

	// Check if this is one of our default admins
	isDefaultAdmin := false
	for _, name := range models.DefaultAdmins {
		if request.Name == name {
			isDefaultAdmin = true
			break
		}
	}

	// Determine role based on default admin status
	role := "user"
	if isDefaultAdmin {
		role = "admin"
	}

	if err == sql.ErrNoRows {
		// User doesn't exist, create a new one
		_, err = database.DB.Exec(
			"INSERT INTO users (id, username, name, status, isAdmin, role) VALUES (?, ?, ?, ?, ?, ?)",
			request.FirebaseID,
			request.Email,
			request.Name,
			"approved",
			isDefaultAdmin,
			role,
		)

		if err != nil {
			http.Error(w, "Failed to create user: "+err.Error(), http.StatusInternalServerError)
			return
		}

		userID = request.FirebaseID
	} else if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	} else if isDefaultAdmin {
		// User exists, but we need to ensure they're an admin if they're a default admin
		_, err = database.DB.Exec(
			"UPDATE users SET isAdmin = ?, role = ? WHERE id = ?",
			true,
			"admin",
			request.FirebaseID,
		)

		if err != nil {
			http.Error(w, "Failed to update user privileges: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Return success with user ID
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"id": userID,
	})
}

// CreateOrUpdateFirebaseUser creates a new user account with the Firebase UID
// No linking with existing accounts is attempted
func CreateOrUpdateFirebaseUser(w http.ResponseWriter, r *http.Request) {
	// Get the Firebase user ID from the request context
	firebaseUID := middleware.GetUserIDFromContext(r)
	if firebaseUID == "" {
		http.Error(w, "Unauthorized: No Firebase UID found", http.StatusUnauthorized)
		return
	}

	// Parse the request body to get the user details
	var userRequest struct {
		Email    string `json:"email"`
		Name     string `json:"name"`
		Username string `json:"username,omitempty"` // Optional
	}

	err := json.NewDecoder(r.Body).Decode(&userRequest)
	if err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// If no username is provided, use the email
	if userRequest.Username == "" {
		userRequest.Username = userRequest.Email
	}

	log.Printf("Creating or updating user with Firebase UID %s", firebaseUID)

	// Check if this user already exists
	var existingID string
	err = database.DB.QueryRow("SELECT id FROM users WHERE id = ?", firebaseUID).Scan(&existingID)
	if err == nil {
		// User exists, update the record
		_, err = database.DB.Exec(
			"UPDATE users SET name = ?, username = ? WHERE id = ?",
			userRequest.Name, userRequest.Username, firebaseUID)

		if err != nil {
			http.Error(w, "Failed to update user: "+err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf("Updated existing user %s", firebaseUID)
	} else {
		// Create a new user record with this Firebase UID
		_, err = database.DB.Exec(
			"INSERT INTO users (id, username, name, status) VALUES (?, ?, ?, ?)",
			firebaseUID, userRequest.Username, userRequest.Name, "approved")

		if err != nil {
			http.Error(w, "Failed to create user: "+err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf("Created new user with Firebase UID %s", firebaseUID)
	}

	// Return the user info
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.User{
		ID:       firebaseUID,
		Username: userRequest.Username,
		Name:     userRequest.Name,
		Status:   "approved",
	})
}
