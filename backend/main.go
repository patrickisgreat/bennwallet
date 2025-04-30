package main

import (
	"log"
	"net/http"

	"bennwallet/backend/database"
	"bennwallet/backend/handlers"
	"bennwallet/backend/middleware"

	"github.com/gorilla/mux"
)

func main() {
	// Initialize database
	err := database.InitDB()
	if err != nil {
		log.Fatal(err)
	}

	// Seed default users
	err = database.SeedDefaultUsers()
	if err != nil {
		log.Fatal(err)
	}

	// Create router
	r := mux.NewRouter()

	// Health check
	r.HandleFunc("/health", handlers.HealthCheck).Methods("GET")

	// Transaction routes
	r.HandleFunc("/transactions", handlers.GetTransactions).Methods("GET")
	r.HandleFunc("/transactions", handlers.AddTransaction).Methods("POST")
	r.HandleFunc("/transactions/{id}", handlers.UpdateTransaction).Methods("PUT")
	r.HandleFunc("/transactions/{id}", handlers.DeleteTransaction).Methods("DELETE")

	// Category routes
	r.HandleFunc("/categories", handlers.GetCategories).Methods("GET")
	r.HandleFunc("/categories", handlers.AddCategory).Methods("POST")
	r.HandleFunc("/categories/{id}", handlers.UpdateCategory).Methods("PUT")
	r.HandleFunc("/categories/{id}", handlers.DeleteCategory).Methods("DELETE")

	// User routes
	r.HandleFunc("/users", handlers.GetUsers).Methods("GET")
	r.HandleFunc("/users/{username}", handlers.GetUserByUsername).Methods("GET")

	// Report routes
	r.HandleFunc("/reports/ynab-splits", handlers.GetYNABSplits).Methods("GET", "POST")

	// Serve static files
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("dist")))

	// Apply middleware
	handler := middleware.EnableCORS(r)

	// Start server
	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", handler))
}
