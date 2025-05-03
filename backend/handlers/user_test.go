package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"bennwallet/backend/database"
	"bennwallet/backend/models"

	"github.com/gorilla/mux"
)

func setupTestDB() {
	// Create a test database connection
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		panic(err)
	}
	database.DB = db

	// Create users table with all fields
	_, err = db.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			name TEXT,
			status TEXT DEFAULT 'approved',
			isAdmin BOOLEAN DEFAULT 0,
			role TEXT DEFAULT 'user'
		)
	`)
	if err != nil {
		panic(err)
	}

	// Insert test data including admin user
	_, err = db.Exec("INSERT INTO users (id, username, name, status, isAdmin, role) VALUES (?, ?, ?, ?, ?, ?)",
		"test1", "testuser", "Test User", "approved", 0, "user")
	if err != nil {
		panic(err)
	}

	_, err = db.Exec("INSERT INTO users (id, username, name, status, isAdmin, role) VALUES (?, ?, ?, ?, ?, ?)",
		"admin1", "Sarah", "Sarah Admin", "approved", 1, "admin")
	if err != nil {
		panic(err)
	}
}

func TestGetUsers(t *testing.T) {
	setupTestDB()
	defer database.DB.Close()

	req, err := http.NewRequest("GET", "/users", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Add mock authentication as admin
	req = MockAuthContext(req, "admin1")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(GetUsers)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	var users []models.User
	if err := json.NewDecoder(rr.Body).Decode(&users); err != nil {
		t.Fatal(err)
	}

	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}

	// Find the non-admin user
	var foundTestUser bool
	for _, user := range users {
		if user.Username == "testuser" {
			foundTestUser = true
			if user.Role != "user" {
				t.Errorf("expected role 'user', got '%s'", user.Role)
			}
			if user.IsAdmin {
				t.Errorf("expected IsAdmin to be false")
			}
		}
	}

	// Find the admin user
	var foundAdminUser bool
	for _, user := range users {
		if user.Username == "Sarah" {
			foundAdminUser = true
			if user.Role != "admin" {
				t.Errorf("expected role 'admin', got '%s'", user.Role)
			}
			if !user.IsAdmin {
				t.Errorf("expected IsAdmin to be true")
			}
		}
	}

	if !foundTestUser {
		t.Errorf("did not find the test user in the results")
	}

	if !foundAdminUser {
		t.Errorf("did not find the admin user in the results")
	}
}

func TestGetUserByUsername(t *testing.T) {
	setupTestDB()
	defer database.DB.Close()

	req, err := http.NewRequest("GET", "/users/testuser", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Add username to request context
	req = mux.SetURLVars(req, map[string]string{"username": "testuser"})

	// Add mock authentication
	req = MockAuthContext(req, "test1")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(GetUserByUsername)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	var user models.User
	if err := json.NewDecoder(rr.Body).Decode(&user); err != nil {
		t.Fatal(err)
	}

	if user.Username != "testuser" {
		t.Errorf("handler returned unexpected username: got %v want %v",
			user.Username, "testuser")
	}

	if user.Status != "approved" {
		t.Errorf("expected Status 'approved', got '%s'", user.Status)
	}

	if user.Role != "user" {
		t.Errorf("expected Role 'user', got '%s'", user.Role)
	}
}

func TestGetUserByUsername_NotFound(t *testing.T) {
	setupTestDB()
	defer database.DB.Close()

	req, err := http.NewRequest("GET", "/users/nonexistent", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Add username to request context
	req = mux.SetURLVars(req, map[string]string{"username": "nonexistent"})

	// Add mock authentication
	req = MockAuthContext(req, "test1")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(GetUserByUsername)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusNotFound)
	}
}
