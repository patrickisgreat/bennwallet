package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"bennwallet/backend/database"
	"bennwallet/backend/handlers"
	"bennwallet/backend/middleware"
	"bennwallet/backend/migrations"
	"bennwallet/backend/models"
	"bennwallet/backend/security"
	"bennwallet/backend/services"

	"github.com/gorilla/mux"
)

var (
	port       = flag.String("port", ":8080", "Port to listen on")
	resetDB    = flag.Bool("reset-db", false, "Reset database")
	devMode    = flag.Bool("dev", true, "Run in development mode with test auth")
	testUserID = flag.String("test-user", "admin-user-1", "Test user ID for dev mode")
	noExit     = flag.Bool("no-exit", false, "Don't exit after migrations")
)

func main() {
	// Check if running in migration mode
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		runMigrations()
		return
	}

	// Parse command line flags
	flag.Parse()

	// Check if we're running in database reset mode
	isResetDB := os.Getenv("RESET_DB") == "true" || *resetDB

	// Debug - print RESET_DB value
	log.Printf("RESET_DB environment variable value: '%s'", os.Getenv("RESET_DB"))
	log.Printf("Is reset DB flag: %v", isResetDB)

	// Check if this is a PR deployment
	isPRDeployment := os.Getenv("PR_DEPLOYMENT") == "true"

	// Check environment
	isDevelopment := os.Getenv("APP_ENV") != "production" &&
		os.Getenv("NODE_ENV") != "production" &&
		os.Getenv("ENVIRONMENT") != "production" &&
		os.Getenv("ENV") != "production"

	// In development mode, we don't automatically reset the database
	// User must explicitly set RESET_DB=true or use the --reset-db flag
	// This ensures we don't lose data every time the server starts

	if isResetDB {
		log.Println("Running in database reset mode")
	}

	if isPRDeployment {
		log.Println("Running in PR deployment mode")
	}

	if isDevelopment {
		log.Println("Running in development environment")
	}

	// Use an encryption key from environment or generate a default one
	encryptionKey := os.Getenv("ENCRYPTION_KEY")
	if encryptionKey == "" {
		log.Println("Warning: ENCRYPTION_KEY not set in environment, checking .env file...")

		// Try to load from .env.local first, then fall back to .env
		if data, err := os.ReadFile(".env.local"); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "ENCRYPTION_KEY=") {
					encryptionKey = strings.TrimPrefix(line, "ENCRYPTION_KEY=")
					log.Println("Successfully loaded ENCRYPTION_KEY from .env.local file")
					break
				}
			}
		}

		// If still not found, try .env
		if encryptionKey == "" {
			if data, err := os.ReadFile(".env"); err == nil {
				for _, line := range strings.Split(string(data), "\n") {
					if strings.HasPrefix(line, "ENCRYPTION_KEY=") {
						encryptionKey = strings.TrimPrefix(line, "ENCRYPTION_KEY=")
						log.Println("Successfully loaded ENCRYPTION_KEY from .env file")
						break
					}
				}
			}
		}

		// Last resort, use a default key in development
		if encryptionKey == "" {
			log.Println("Warning: ENCRYPTION_KEY not found in any file, using a default key. This is NOT secure for production!")
			encryptionKey = "default-key-for-development-only"
		}
	} else {
		log.Println("Using ENCRYPTION_KEY from environment variables")
	}

	log.Printf("Initializing encryption with key (length: %d)", len(encryptionKey))
	security.InitializeEncryption(encryptionKey)

	// Initialize database
	err := database.InitDB()
	if err != nil {
		log.Fatal(err)
	}

	// Fix YNAB config table schema if needed
	log.Println("Checking and fixing YNAB table schema...")
	err = models.FixYNABTableSchema(database.DB)
	if err != nil {
		log.Printf("Warning: Error fixing YNAB table schema: %v", err)
	} else {
		log.Println("YNAB table schema check completed")
	}

	// Run migrations (including test data seeding if in dev/PR environment)
	log.Println("Running migrations...")
	err = migrations.RunMigrations(database.DB, isResetDB)
	if err != nil {
		log.Printf("Warning: Failed to run migrations: %v", err)
	}

	// If running in reset mode, exit after database setup is complete
	// unless --no-exit flag is provided
	if isResetDB && !*noExit {
		log.Println("Database reset completed successfully. Exiting.")
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

	// Dev-only middleware that sets a test user ID for authentication
	testAuthMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only apply in dev mode
			if *devMode {
				log.Printf("Dev mode: setting test user ID: %s", *testUserID)
				ctx := context.WithValue(r.Context(), middleware.UserIDKey, *testUserID)
				next.ServeHTTP(w, r.WithContext(ctx))
			} else {
				next.ServeHTTP(w, r)
			}
		})
	}

	// Apply our middleware chain
	// CORS first, then auth
	r.Use(middleware.EnableCORS)

	if *devMode {
		// In dev mode, use test auth middleware
		r.Use(testAuthMiddleware)
		log.Println("Running in dev mode with test authentication")
	} else {
		// In production, use real auth middleware
		r.Use(middleware.AuthMiddleware)
	}

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
	srv := &http.Server{
		Handler:      r,
		Addr:         *port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	// Start the server
	log.Printf("Starting server on port %s...", *port)
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
	protectedRouter.HandleFunc("/user/me", handlers.GetCurrentUser).Methods("GET")

	// Protected YNAB routes
	protectedRouter.HandleFunc("/ynab/categories", handlers.GetYNABCategories).Methods("GET")
	protectedRouter.HandleFunc("/ynab/sync", handlers.SyncYNABTransaction).Methods("POST")
	protectedRouter.HandleFunc("/reports/ynab-splits", handlers.GetYNABSplits).Methods("POST")

	// YNAB Config routes (add these to match frontend expectations)
	protectedRouter.HandleFunc("/ynab/config", handlers.GetYNABConfig).Methods("GET")
	protectedRouter.HandleFunc("/ynab/config", handlers.UpdateYNABConfig).Methods("PUT")
	protectedRouter.HandleFunc("/ynab/sync/categories", handlers.SyncYNABCategories).Methods("POST")

	// Diagnostic routes
	protectedRouter.HandleFunc("/diagnostic/db-check", handlers.CheckDatabaseHandler).Methods("GET")

	// Permission management routes
	protectedRouter.HandleFunc("/permissions", handlers.GetUserPermissions).Methods("GET")
	protectedRouter.HandleFunc("/permissions", handlers.GrantPermission).Methods("POST")
	protectedRouter.HandleFunc("/permissions", handlers.RevokePermission).Methods("DELETE")
	protectedRouter.HandleFunc("/permissions/all", handlers.GetAllPermissions).Methods("GET")
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

// runMigrations handles the migrate command-line functionality
func runMigrations() {
	// Parse flags for migration command
	migrateCmd := flag.NewFlagSet("migrate", flag.ExitOnError)
	reset := migrateCmd.Bool("reset", false, "Reset database before migrations (WARNING: DELETES ALL DATA)")
	dryRun := migrateCmd.Bool("dry-run", false, "Check which migrations would be applied without executing them")

	// Parse the remaining command-line arguments
	args := os.Args[2:] // Skip "migrate" command
	if err := migrateCmd.Parse(args); err != nil {
		log.Fatalf("Failed to parse migration flags: %v", err)
	}

	// Check if reset was requested through environment variable
	resetEnv := os.Getenv("RESET_DB") == "true"
	if resetEnv {
		*reset = true
	}

	// Connect to database
	log.Println("Connecting to database...")
	db, err := database.CreatePostgresDB()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Confirm dangerous operations in production
	if *reset && (os.Getenv("APP_ENV") == "production" || os.Getenv("NODE_ENV") == "production") {
		log.Println("⚠️ WARNING: You are about to RESET the PRODUCTION database! ⚠️")
		log.Println("This will DELETE ALL DATA in the database.")
		log.Println("To proceed, set CONFIRM_PROD_RESET=yes in environment.")

		if os.Getenv("CONFIRM_PROD_RESET") != "yes" {
			log.Fatalf("Production database reset was not confirmed. Aborting.")
		}
	}

	// Check which migrations need to be applied (dry run)
	if *dryRun {
		pendingMigrations, err := migrations.GetPendingMigrations(db)
		if err != nil {
			log.Fatalf("Failed to check pending migrations: %v", err)
		}

		if len(pendingMigrations) == 0 {
			fmt.Println("✅ Database schema is up to date. No migrations needed.")
		} else {
			fmt.Println("🔍 Pending migrations that would be applied:")
			for _, migration := range pendingMigrations {
				fmt.Printf("  - %s\n", migration)
			}
		}
		return
	}

	// Run the migrations
	log.Println("Running migrations...")
	err = migrations.RunMigrations(db, *reset)
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	log.Println("✅ Database migrations completed successfully!")
}
