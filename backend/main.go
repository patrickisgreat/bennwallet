package main

import (
	"log"
	"net/http"
	"os"

	"bennwallet/backend/api"
	"bennwallet/backend/database"
	"bennwallet/backend/handlers"
	"bennwallet/backend/middleware"
	"bennwallet/backend/security"
	"bennwallet/backend/ynab"

	"github.com/gorilla/mux"
)

func main() {
	// Initialize encryption
	encryptionKey := os.Getenv("ENCRYPTION_KEY")
	if encryptionKey == "" {
		// Generate a default key for development
		log.Println("Warning: ENCRYPTION_KEY not set, using a default key. This is NOT secure for production!")
		encryptionKey = "default-dev-encryption-key-32chars"
	}
	security.InitializeEncryption(encryptionKey)

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

	// Initialize YNAB sync
	err = ynab.InitYNABSync(database.DB)
	if err != nil {
		log.Printf("Warning: Failed to initialize YNAB sync: %v", err)
		// Continue without YNAB sync - it will start when users configure it
	}

	// Create router
	r := mux.NewRouter()

	// Initialize API server
	apiServer := api.NewServer(database.DB)

	// Create YNAB handler
	ynabHandler := handlers.NewYNABHandler(database.DB)

	// Health check
	r.HandleFunc("/health", handlers.HealthCheck).Methods("GET")

	// Transaction routes
	r.HandleFunc("/transactions", handlers.GetTransactions).Methods("GET")
	r.HandleFunc("/transactions", handlers.AddTransaction).Methods("POST")
	r.HandleFunc("/transactions/{id}", handlers.GetTransaction).Methods("GET")
	r.HandleFunc("/transactions/{id}", handlers.UpdateTransaction).Methods("PUT")
	r.HandleFunc("/transactions/{id}", handlers.DeleteTransaction).Methods("DELETE")

	// Category routes
	r.HandleFunc("/categories", handlers.GetCategories).Methods("GET")
	r.HandleFunc("/categories", handlers.AddCategory).Methods("POST")
	r.HandleFunc("/categories/{id}", handlers.UpdateCategory).Methods("PUT")
	r.HandleFunc("/categories/{id}", handlers.DeleteCategory).Methods("DELETE")

	// YNAB routes using the new handler
	r.HandleFunc("/ynab/config", ynabHandler.GetYNABConfig).Methods("GET")
	r.HandleFunc("/ynab/config", ynabHandler.UpdateYNABConfig).Methods("POST", "PUT")
	r.HandleFunc("/ynab/sync-categories", ynabHandler.SyncYNABCategories).Methods("POST")

	// API routes
	r.PathPrefix("/api/").Handler(http.StripPrefix("/api", apiServer.Handler()))

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
