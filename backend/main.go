package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"bennwallet/backend/api"
	"bennwallet/backend/database"
	"bennwallet/backend/handlers"
	"bennwallet/backend/middleware"
	"bennwallet/backend/models"
	"bennwallet/backend/security"
	"bennwallet/backend/services"
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

	// Seed default users but don't start syncing yet
	err = database.SeedDefaultUsers()
	if err != nil {
		log.Fatal(err)
	}

	// Load environment variables but don't do any database operations
	services.LoadEnvVariables()

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

	// Category routes - consider deprecating these as we're now using YNAB categories
	r.HandleFunc("/categories", handlers.GetCategories).Methods("GET")
	r.HandleFunc("/categories", handlers.AddCategory).Methods("POST")
	r.HandleFunc("/categories/{id}", handlers.UpdateCategory).Methods("PUT")
	r.HandleFunc("/categories/{id}", handlers.DeleteCategory).Methods("DELETE")

	// YNAB routes
	r.HandleFunc("/ynab/categories", handlers.GetYNABCategories).Methods("GET")
	r.HandleFunc("/ynab/sync", handlers.SyncYNABTransaction).Methods("POST")
	r.HandleFunc("/ynab/config", ynabHandler.GetYNABConfig).Methods("GET")
	r.HandleFunc("/ynab/config", ynabHandler.UpdateYNABConfig).Methods("POST", "PUT")
	r.HandleFunc("/ynab/sync-categories", ynabHandler.SyncYNABCategories).Methods("POST")
	r.HandleFunc("/ynab/force-sync", func(w http.ResponseWriter, r *http.Request) {
		userId := r.URL.Query().Get("userId")
		if userId == "" {
			http.Error(w, "userId is required", http.StatusBadRequest)
			return
		}

		// Get the user's YNAB config
		config, err := models.GetYNABConfig(database.DB, userId)
		if err != nil {
			log.Printf("Error retrieving YNAB config: %v", err)
			http.Error(w, "Error retrieving YNAB configuration", http.StatusInternalServerError)
			return
		}

		if !config.HasCredentials {
			// Try to configure from environment variables as a fallback
			services.SetupYNABForUser(userId)
		}

		// Get budget ID for the user (after potential setup)
		var budgetId string
		err = database.DB.QueryRow("SELECT budget_id FROM user_ynab_settings WHERE user_id = ?", userId).Scan(&budgetId)
		if err != nil {
			log.Printf("Error getting budget ID for user %s: %v", userId, err)
			http.Error(w, "User not found or YNAB not configured", http.StatusBadRequest)
			return
		}

		// Force sync for this user
		err = services.SyncYNABCategoriesNew(userId, budgetId)
		if err != nil {
			log.Printf("Error syncing YNAB categories for user %s: %v", userId, err)
			http.Error(w, fmt.Sprintf("Error syncing: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"success","message":"YNAB categories synced successfully"}`))
	}).Methods("GET")

	// API routes
	r.PathPrefix("/api/").Handler(http.StripPrefix("/api", apiServer.Handler()))

	// User routes
	r.HandleFunc("/users", handlers.GetUsers).Methods("GET")
	r.HandleFunc("/users/{username}", handlers.GetUserByUsername).Methods("GET")
	r.HandleFunc("/users/sync", handlers.SyncFirebaseUser).Methods("POST")

	// Report routes
	r.HandleFunc("/reports/ynab-splits", handlers.GetYNABSplits).Methods("GET", "POST")

	// Create a file server for static files
	fileServer := http.FileServer(http.Dir("dist"))

	// Special handler for SPA routes - serve index.html for any unknown routes
	r.PathPrefix("/").Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if the requested path is an API route - if so, return 404
		if len(r.URL.Path) >= 4 && r.URL.Path[0:4] == "/api" {
			http.NotFound(w, r)
			return
		}

		// Otherwise serve the static files
		fileServer.ServeHTTP(w, r)
	}))

	// Apply middleware
	handler := middleware.EnableCORS(r)

	// Start server
	log.Println("Server starting on :8080")

	// Start YNAB sync in a separate goroutine after server starts
	go func() {
		log.Println("Starting background YNAB initialization...")
		time.Sleep(5 * time.Second)

		// Setup YNAB from environment variables if available
		services.SetupYNABFromEnv()

		// Initialize YNAB sync system - this will only start background sync if there are configured users
		if err := ynab.InitYNABSync(database.DB); err != nil {
			log.Printf("Error initializing YNAB sync: %v", err)
		}

		// Trigger initial sync for any configured users
		services.InitialSync()

		log.Println("YNAB initialization completed")
	}()

	log.Fatal(http.ListenAndServe(":8080", handler))
}
