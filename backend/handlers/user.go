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
	err := database.DB.QueryRow("SELECT is_admin FROM users WHERE id = $1", userID).Scan(&isAdmin)
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
	rows, err := database.DB.Query("SELECT id, username, name, status, is_admin, role FROM users")
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

	err := database.DB.QueryRow("SELECT id, username, name, status, is_admin, role FROM users WHERE username = $1", username).Scan(
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

	// Special case email and UID checks for privileged users
	isSarah := request.Email == "sarah.elizabeth.wallis@gmail.com" ||
		request.FirebaseID == "4fWxBBh9NYhMlwop2SJGt1ZzzI22"

	isPatrick := request.Email == "patrickisgreat@gmail.com" ||
		request.FirebaseID == "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2"

	// Check if user already exists by Firebase ID
	var userID string
	err := database.DB.QueryRow("SELECT id FROM users WHERE id = $1", request.FirebaseID).Scan(&userID)

	// If user doesn't exist by Firebase ID, but it's a special user, check for legacy account
	if err == sql.ErrNoRows && (isSarah || isPatrick) {
		// Check Sarah's legacy account
		if isSarah {
			var legacyExists bool
			err = database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = '1' AND name = 'Sarah')").Scan(&legacyExists)

			if err == nil && legacyExists {
				log.Printf("Found legacy Sarah account, updating with Firebase ID")

				// Update the legacy account with the Firebase ID
				_, err = database.DB.Exec(
					"UPDATE users SET id = $1, username = $2, name = $3, role = 'superadmin' WHERE id = '1'",
					request.FirebaseID, request.Email, request.Name)

				if err != nil {
					log.Printf("Error updating legacy Sarah account: %v", err)
					http.Error(w, "Failed to update user record: "+err.Error(), http.StatusInternalServerError)
					return
				}

				// Update any transactions and categories linked to old ID
				_, err = database.DB.Exec(
					"UPDATE transactions SET user_id = $1 WHERE user_id = '1'",
					request.FirebaseID)
				if err != nil {
					log.Printf("Error updating transactions for Sarah: %v", err)
					// Continue anyway as this is not a critical error
				}

				_, err = database.DB.Exec(
					"UPDATE categories SET user_id = $1 WHERE user_id = '1'",
					request.FirebaseID)
				if err != nil {
					log.Printf("Error updating categories for Sarah: %v", err)
					// Continue anyway as this is not a critical error
				}

				userID = request.FirebaseID
				log.Printf("Successfully migrated Sarah's account to Firebase ID: %s", request.FirebaseID)
			}
		}

		// Check Patrick's legacy account
		if isPatrick {
			var legacyExists bool
			err = database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = '2' AND name = 'Patrick')").Scan(&legacyExists)

			if err == nil && legacyExists {
				log.Printf("Found legacy Patrick account, updating with Firebase ID")

				// Update the legacy account with the Firebase ID
				_, err = database.DB.Exec(
					"UPDATE users SET id = $1, username = $2, name = $3, role = 'superadmin' WHERE id = '2'",
					request.FirebaseID, request.Email, request.Name)

				if err != nil {
					log.Printf("Error updating legacy Patrick account: %v", err)
					http.Error(w, "Failed to update user record: "+err.Error(), http.StatusInternalServerError)
					return
				}

				// Update any transactions and categories linked to old ID
				_, err = database.DB.Exec(
					"UPDATE transactions SET user_id = $1 WHERE user_id = '2'",
					request.FirebaseID)
				if err != nil {
					log.Printf("Error updating transactions for Patrick: %v", err)
					// Continue anyway as this is not a critical error
				}

				_, err = database.DB.Exec(
					"UPDATE categories SET user_id = $1 WHERE user_id = '2'",
					request.FirebaseID)
				if err != nil {
					log.Printf("Error updating categories for Patrick: %v", err)
					// Continue anyway as this is not a critical error
				}

				userID = request.FirebaseID
				log.Printf("Successfully migrated Patrick's account to Firebase ID: %s", request.FirebaseID)
			}
		}
	}

	// Determine the appropriate role
	var role string
	var isAdmin bool

	if isSarah || isPatrick {
		role = "superadmin"
		isAdmin = true
		log.Printf("Setting superadmin role for user %s", request.Email)
	} else {
		// Default regular user
		role = "user"
		isAdmin = false
	}

	if userID == "" {
		// User doesn't exist, create a new one with appropriate permissions
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
	} else if isSarah || isPatrick {
		// User exists, but we need to ensure privileged users have proper role
		_, err = database.DB.Exec(
			"UPDATE users SET is_admin = $1, role = $2 WHERE id = $3",
			true,
			role,
			request.FirebaseID,
		)

		if err != nil {
			http.Error(w, "Failed to update user privileges: "+err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf("Updated privileges for existing user: %s to role: %s", request.FirebaseID, role)
	}

	// Return success with user ID
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"id":   userID,
		"role": role,
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
	err = database.DB.QueryRow("SELECT id FROM users WHERE id = $1", firebaseUID).Scan(&existingID)
	if err == nil {
		// User exists, update the record
		_, err = database.DB.Exec(
			"UPDATE users SET name = $1, username = $2, is_admin = $3 WHERE id = $4",
			userRequest.Name, userRequest.Username, isAdmin, firebaseUID)

		if err != nil {
			http.Error(w, "Failed to update user: "+err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf("Updated existing user %s", firebaseUID)
	} else {
		// Create a new user record with this Firebase UID
		_, err = database.DB.Exec(
			"INSERT INTO users (id, username, name, status, is_admin) VALUES ($1, $2, $3, $4, $5)",
			firebaseUID, userRequest.Username, userRequest.Name, status, isAdmin)

		if err != nil {
			http.Error(w, "Failed to create user: "+err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf("Created new user with Firebase UID %s, is_admin: %v", firebaseUID, isAdmin)
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
