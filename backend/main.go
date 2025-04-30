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

	// Create a file server for static files
	fileServer := http.FileServer(http.Dir("dist"))

	// Special handler for SPA routes - serve index.html for any unknown routes
	r.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve API 404 errors as-is
		if r.URL.Path == "/reports/ynab-splits" ||
			r.URL.Path == "/transactions" ||
			r.URL.Path == "/categories" ||
			r.URL.Path == "/users" {
			http.NotFound(w, r)
			return
		}

		// For other routes, serve the SPA's index.html
		log.Printf("Serving index.html for route: %s", r.URL.Path)
		http.ServeFile(w, r, "dist/index.html")
	})

	// Serve static assets directly
	r.PathPrefix("/assets/").Handler(fileServer)

	// Handle root explicitly to serve index.html
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "dist/index.html")
	})

	// Apply middleware
	handler := middleware.EnableCORS(r)

	// Start server
	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", handler))
}
