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

	log.Printf("GetUsers called by user ID: %s", userID)

	// Check if the user is an admin or superadmin by role
	var role string
	err := database.DB.QueryRow("SELECT role FROM users WHERE id = $1", userID).Scan(&role)
	if err != nil {
		log.Printf("Error checking user role: %v", err)
		http.Error(w, "Failed to check user permissions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Only admins or superadmins can see all users
	isAdmin := role == "admin" || role == "superadmin"
	if !isAdmin {
		log.Printf("User %s with role %s attempted to access user list - access denied", userID, role)
		http.Error(w, "Unauthorized: Admin access required", http.StatusForbidden)
		return
	}

	log.Printf("User %s has role %s, authorized to view users list", userID, role)

	// Update query to include only fields that exist in the database
	rows, err := database.DB.Query(`
		SELECT id, username, name, role
		FROM users
		ORDER BY name ASC
	`)
	if err != nil {
		log.Printf("Error querying users: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User

		err := rows.Scan(&u.ID, &u.Username, &u.Name, &u.Role)
		if err != nil {
			log.Printf("Error scanning user row: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Set default values for missing fields
		u.Status = "approved"
		u.IsAdmin = (u.Role == "admin" || u.Role == "superadmin")
		u.Email = u.Username // Use username as fallback for email

		users = append(users, u)
	}

	log.Printf("Returning %d users", len(users))

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

	err := database.DB.QueryRow("SELECT id, username, name, role FROM users WHERE username = $1", username).Scan(
		&user.ID, &user.Username, &user.Name, &user.Role)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Set default values for missing columns
	user.Status = "approved"
	user.IsAdmin = (user.Role == "admin" || user.Role == "superadmin")
	user.Email = user.Username // Use username as email fallback

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// SyncFirebaseUser syncs a Firebase user with the backend database
// This ensures that Firebase users exist in our users table for permissions system
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

	log.Printf("Syncing Firebase user with ID: %s, Email: %s, Name: %s", request.FirebaseID, request.Email, request.Name)

	// Check if user already exists by Firebase ID
	var userID string
	err := database.DB.QueryRow("SELECT id FROM users WHERE id = $1", request.FirebaseID).Scan(&userID)
	if err != nil && err != sql.ErrNoRows {
		http.Error(w, "Failed to look up user", http.StatusInternalServerError)
		return
	}

	// Get roles from config or database, not hardcoded
	role := "user" // Default role for new users
	isAdmin := false

	if userID == "" {
		// User doesn't exist, create a new one
		_, err = database.DB.Exec(
			"INSERT INTO users (id, username, name, status, is_admin, role) VALUES ($1, $2, $3, $4, $5, $6)",
			request.FirebaseID,
			request.Email,
			request.Name,
			"approved",
			isAdmin,
			role,
		)

		if err != nil {
			http.Error(w, "Failed to create user: "+err.Error(), http.StatusInternalServerError)
			return
		}

		userID = request.FirebaseID
		log.Printf("Created new user with Firebase ID: %s, role: %s", request.FirebaseID, role)
	}

	// Return success with user ID
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"id":   userID,
		"role": role,
	})
}

// GetCurrentUser gets the current user's profile information
func GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	// Get the user ID from the authentication context
	userID := middleware.GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized: No user ID found", http.StatusUnauthorized)
		return
	}

	log.Printf("GetCurrentUser called for user ID: %s", userID)

	// Special case for Patrick Bennett - ensure superadmin role
	if userID == "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2" {
		user := models.User{
			ID:       userID,
			Username: "Patrick Bennett",
			Name:     "Patrick Bennett",
			Status:   "approved",
			IsAdmin:  true,
			Role:     "superadmin",
		}
		log.Printf("Returning hardcoded superadmin user data for Patrick Bennett")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(user)
		return
	}

	var user models.User
	var status, role sql.NullString
	var isAdmin sql.NullBool

	// Query the database for the user
	err := database.DB.QueryRow(`
		SELECT id, username, name, status, is_admin, role 
		FROM users 
		WHERE id = $1
	`, userID).Scan(&user.ID, &user.Username, &user.Name, &status, &isAdmin, &role)

	if err != nil {
		if err == sql.ErrNoRows {
			// User not found in our database, create a generic profile
			user = models.User{
				ID:       userID,
				Username: "user_" + userID,
				Name:     "User " + userID,
				Status:   "approved",
				Role:     "user",
			}
		} else {
			log.Printf("Error fetching user data: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
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
	}

	log.Printf("Returning user data: %+v", user)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}
