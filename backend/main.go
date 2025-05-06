package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"bennwallet/backend/database"
	"bennwallet/backend/handlers"
	"bennwallet/backend/middleware"
	"bennwallet/backend/security"
	"bennwallet/backend/services"

	"github.com/gorilla/mux"
)

func main() {
	// Check if we're running in database reset mode
	if os.Getenv("RESET_DB") == "true" {
		log.Println("Running in database reset mode")

		// In PR deployments with RESET_DB=true, completely reset the database
		if os.Getenv("PR_DEPLOYMENT") == "true" {
			log.Println("PR deployment with RESET_DB=true - completely recreating database")
			dbPath := os.Getenv("DB_PATH")
			if dbPath == "" {
				dbPath = "/data/bennwallet.db"
			}

			// Check if file exists
			_, err := os.Stat(dbPath)
			if err == nil {
				// Delete the database file
				log.Printf("Deleting existing database file at %s", dbPath)
				err = os.Remove(dbPath)
				if err != nil {
					log.Printf("Warning: Failed to delete database file: %v", err)
				} else {
					log.Println("Database file deleted successfully")
				}
			} else if !os.IsNotExist(err) {
				log.Printf("Error checking database file: %v", err)
			}
		}
	}

	// Check if this is a PR deployment
	if os.Getenv("PR_DEPLOYMENT") == "true" {
		log.Println("Running in PR deployment mode")
	}

	// Check environment
	if os.Getenv("APP_ENV") != "production" && os.Getenv("NODE_ENV") != "production" {
		log.Println("Running in development environment")
	}

	// Use an encryption key from environment or generate a default one
	encryptionKey := os.Getenv("ENCRYPTION_KEY")
	if encryptionKey == "" {
		log.Println("Warning: ENCRYPTION_KEY not set, using a default key. This is NOT secure for production!")
		encryptionKey = "default-key-for-development-only"
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

	// Run migrations with error handling for RESET_DB mode
	if err := database.RunMigrations(); err != nil {
		// For RESET_DB mode, don't exit with an error if migrations fail
		// This allows the PR deploys to recover from database locks
		if os.Getenv("RESET_DB") == "true" {
			log.Printf("Warning: Migrations failed in RESET_DB mode: %v", err)
			log.Println("Continuing with deployment despite migration errors")
		} else {
			log.Fatal(err)
		}
	}

	// If running in reset mode, exit after database setup is complete
	if os.Getenv("RESET_DB") == "true" {
		log.Println("Database reset attempted. Exiting.")
		return
	}

	// Load environment variables but don't do any database operations
	services.LoadEnvVariables()

	// Initialize Firebase Admin SDK
	log.Println("Initializing Firebase Admin SDK...")
	err = middleware.InitializeFirebase()
	if err != nil {
		log.Printf("Warning: Failed to initialize Firebase: %v", err)
		log.Println("Auth token verification will be disabled!")
	} else {
		log.Println("Firebase Admin SDK initialized (or running in dev mode with auth checks disabled)")
	}

	// Create router
	r := mux.NewRouter()

	// Apply global middleware
	r.Use(middleware.EnableCORS)

	// Register routes with both direct paths and /api prefix to maintain compatibility
	registerRoutes(r)
	apiRouter := r.PathPrefix("/api").Subrouter()
	registerRoutes(apiRouter)

	// Serve static files from the "dist" directory for the frontend
	fs := http.FileServer(http.Dir("./dist"))
	r.PathPrefix("/assets/").Handler(http.StripPrefix("", fs))
	r.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't log asset requests
		if !strings.HasPrefix(r.URL.Path, "/assets/") {
			log.Printf("Serving index.html for path: %s", r.URL.Path)
		}
		http.ServeFile(w, r, "./dist/index.html")
	}).Methods("GET")

	// Configure the server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Handler:      r,
		Addr:         ":" + port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	// Start the server
	log.Printf("Starting server on port %s...", port)
	log.Fatal(srv.ListenAndServe())
}

// registerRoutes sets up all API routes
func registerRoutes(r *mux.Router) {
	// Public routes (no auth required)
	r.HandleFunc("/health", handlers.HealthCheck).Methods("GET", "OPTIONS")

	// Create a subrouter for authenticated routes
	protectedRouter := r.PathPrefix("").Subrouter()
	protectedRouter.Use(middleware.AuthMiddleware)

	// Protected transaction routes
	protectedRouter.HandleFunc("/transactions", handlers.GetTransactions).Methods("GET")
	protectedRouter.HandleFunc("/transactions", handlers.AddTransaction).Methods("POST")
	protectedRouter.HandleFunc("/transactions/unique-fields", handlers.GetUniqueTransactionFields).Methods("GET")
	protectedRouter.HandleFunc("/transactions/{id}", handlers.GetTransaction).Methods("GET")
	protectedRouter.HandleFunc("/transactions/{id}", handlers.UpdateTransaction).Methods("PUT")
	protectedRouter.HandleFunc("/transactions/{id}", handlers.DeleteTransaction).Methods("DELETE")

	// Protected Category routes
	protectedRouter.HandleFunc("/categories", handlers.GetCategories).Methods("GET")
	protectedRouter.HandleFunc("/categories", handlers.AddCategory).Methods("POST")
	protectedRouter.HandleFunc("/categories/{id}", handlers.UpdateCategory).Methods("PUT")
	protectedRouter.HandleFunc("/categories/{id}", handlers.DeleteCategory).Methods("DELETE")

	// Protected User routes
	protectedRouter.HandleFunc("/users", handlers.GetUsers).Methods("GET")
	protectedRouter.HandleFunc("/users/sync", handlers.SyncFirebaseUser).Methods("POST")
	protectedRouter.HandleFunc("/users/{username}", handlers.GetUserByUsername).Methods("GET")

	// Protected YNAB routes
	protectedRouter.HandleFunc("/ynab/categories", handlers.GetYNABCategories).Methods("GET")
	protectedRouter.HandleFunc("/ynab/sync", handlers.SyncYNABTransaction).Methods("POST")
	protectedRouter.HandleFunc("/reports/ynab-splits", handlers.GetYNABSplits).Methods("POST")

	// YNAB Config routes (add these to match frontend expectations)
	protectedRouter.HandleFunc("/ynab/config", handlers.GetYNABConfig).Methods("GET")
	protectedRouter.HandleFunc("/ynab/config", handlers.UpdateYNABConfig).Methods("PUT")
	protectedRouter.HandleFunc("/ynab/sync/categories", handlers.SyncYNABCategories).Methods("POST")

	// Permission management routes
	protectedRouter.HandleFunc("/permissions", handlers.GetUserPermissions).Methods("GET")
	protectedRouter.HandleFunc("/permissions", handlers.GrantPermission).Methods("POST")
	protectedRouter.HandleFunc("/permissions", handlers.RevokePermission).Methods("DELETE")
	protectedRouter.HandleFunc("/roles", handlers.SetUserRole).Methods("POST")
	protectedRouter.HandleFunc("/roles/{userId}", handlers.GetUserRole).Methods("GET")

	// Saved filters routes
	protectedRouter.HandleFunc("/filters", handlers.GetSavedFilters).Methods("GET")
	protectedRouter.HandleFunc("/filters", handlers.CreateSavedFilter).Methods("POST")
	protectedRouter.HandleFunc("/filters/{id}", handlers.GetSavedFilter).Methods("GET")
	protectedRouter.HandleFunc("/filters/{id}", handlers.UpdateSavedFilter).Methods("PUT")
	protectedRouter.HandleFunc("/filters/{id}", handlers.DeleteSavedFilter).Methods("DELETE")

	// Custom reports routes
	protectedRouter.HandleFunc("/reports/custom", handlers.GetCustomReports).Methods("GET")
	protectedRouter.HandleFunc("/reports/custom", handlers.CreateCustomReport).Methods("POST")
	protectedRouter.HandleFunc("/reports/custom/{id}", handlers.GetCustomReport).Methods("GET")
	protectedRouter.HandleFunc("/reports/custom/{id}", handlers.UpdateCustomReport).Methods("PUT")
	protectedRouter.HandleFunc("/reports/custom/{id}", handlers.DeleteCustomReport).Methods("DELETE")
	protectedRouter.HandleFunc("/reports/custom/{id}/run", handlers.RunCustomReport).Methods("POST")
}
