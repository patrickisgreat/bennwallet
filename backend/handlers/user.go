package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"

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

	log.Printf("Syncing Firebase user with ID: %s, Email: %s, Name: %s", request.FirebaseID, request.Email, request.Name)

	// Check if this is a special case for Sarah's email
	isSarahEmail := request.Email == "sarah.elizabeth.wallis@gmail.com"
	// Check if this is Sarah's known Firebase UID
	isSarahUID := request.FirebaseID == "4fWxBBh9NYhMlwop2SJGt1ZzzI22"

	// Check if user already exists by Firebase ID
	var userID string
	err := database.DB.QueryRow("SELECT id FROM users WHERE id = ?", request.FirebaseID).Scan(&userID)

	// If user doesn't exist by Firebase ID, but it's Sarah's email or UID, we need to handle migration
	if err == sql.ErrNoRows && (isSarahEmail || isSarahUID) {
		// Check if Sarah's legacy account exists (with ID=1)
		var legacyExists bool
		err = database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = '1' AND name = 'Sarah')").Scan(&legacyExists)

		if err != nil {
			log.Printf("Error checking for legacy Sarah account: %v", err)
		} else if legacyExists {
			log.Printf("Found legacy Sarah account, will update with Firebase ID")

			// Update the legacy account with the Firebase ID
			_, err = database.DB.Exec(
				"UPDATE users SET id = ?, username = ?, name = ? WHERE id = '1'",
				request.FirebaseID, request.Email, request.Name)

			if err != nil {
				log.Printf("Error updating legacy Sarah account: %v", err)
				http.Error(w, "Failed to update user record: "+err.Error(), http.StatusInternalServerError)
				return
			}

			// Also update any transactions with userId = 1
			_, err = database.DB.Exec(
				"UPDATE transactions SET userId = ? WHERE userId = '1'",
				request.FirebaseID)

			if err != nil {
				log.Printf("Error updating transactions for Sarah: %v", err)
				// Continue anyway as this is not a critical error
			}

			userID = request.FirebaseID
			log.Printf("Successfully migrated Sarah's account to Firebase ID: %s", request.FirebaseID)

			// Continue to return success below
		}
	}

	// Check if this is one of our default admins
	isDefaultAdmin := false
	for _, name := range models.DefaultAdmins {
		if request.Name == name ||
			strings.Contains(request.Name, name) ||
			isSarahEmail ||
			isSarahUID ||
			request.Email == "patrickisgreat@gmail.com" {
			isDefaultAdmin = true
			break
		}
	}

	// Determine role based on default admin status
	role := "user"
	if isDefaultAdmin {
		role = "admin"
	}

	if userID == "" {
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
		log.Printf("Created new user with Firebase ID: %s", request.FirebaseID)
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

		log.Printf("Updated privileges for existing user: %s", request.FirebaseID)
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

	log.Printf("Creating or updating user with Firebase UID %s, email %s", firebaseUID, userRequest.Email)

	// Special handling for Sarah's email to grant proper permissions
	isSpecialUser := userRequest.Email == "sarah.elizabeth.wallis@gmail.com" || firebaseUID == "4fWxBBh9NYhMlwop2SJGt1ZzzI22"

	// Default to approved status
	status := "approved"

	// Check if this is Sarah or another recognized admin by email or UID
	isAdmin := false
	if isSpecialUser || userRequest.Email == "patrickisgreat@gmail.com" || firebaseUID == "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2" {
		isAdmin = true
		log.Printf("Recognized special user %s with email %s", userRequest.Name, userRequest.Email)
	}

	// Check if this user already exists
	var existingID string
	err = database.DB.QueryRow("SELECT id FROM users WHERE id = ?", firebaseUID).Scan(&existingID)
	if err == nil {
		// User exists, update the record
		_, err = database.DB.Exec(
			"UPDATE users SET name = ?, username = ?, isAdmin = ? WHERE id = ?",
			userRequest.Name, userRequest.Username, isAdmin, firebaseUID)

		if err != nil {
			http.Error(w, "Failed to update user: "+err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf("Updated existing user %s", firebaseUID)
	} else {
		// Create a new user record with this Firebase UID
		_, err = database.DB.Exec(
			"INSERT INTO users (id, username, name, status, isAdmin) VALUES (?, ?, ?, ?, ?)",
			firebaseUID, userRequest.Username, userRequest.Name, status, isAdmin)

		if err != nil {
			http.Error(w, "Failed to create user: "+err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf("Created new user with Firebase UID %s, isAdmin: %v", firebaseUID, isAdmin)
	}

	// Return the user info
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.User{
		ID:       firebaseUID,
		Username: userRequest.Username,
		Name:     userRequest.Name,
		Status:   status,
		IsAdmin:  isAdmin,
	})
}
