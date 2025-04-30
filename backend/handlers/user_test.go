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

	// Create users table
	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			name TEXT
		)
	`)
	if err != nil {
		panic(err)
	}

	// Insert test data
	_, err = db.Exec("INSERT INTO users (username, name) VALUES (?, ?)", "testuser", "Test User")
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

	if len(users) != 1 {
		t.Errorf("expected 1 user, got %d", len(users))
	}

	if users[0].Username != "testuser" {
		t.Errorf("unexpected username: got %v want %v",
			users[0].Username, "testuser")
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

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(GetUserByUsername)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusNotFound)
	}
}
