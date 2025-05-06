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
	// Check if we're running in reset mode
	resetDb := os.Getenv("RESET_DB") == "true"
	isPR := os.Getenv("PR_DEPLOYMENT") == "true"
	isDevEnv := os.Getenv("APP_ENV") == "development"

	if resetDb {
		log.Println("Running in database reset mode")
	}

	if isPR {
		log.Println("Running in PR deployment mode")
	}

	if isDevEnv {
		log.Println("Running in development environment")
	}

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

	// If running in reset mode, exit after database setup is complete
	if resetDb {
		log.Println("Database reset and seeded successfully. Exiting.")
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
}
