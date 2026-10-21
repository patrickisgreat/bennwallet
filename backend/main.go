package main

import (
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

const defaultDevEncryptionKey = "default-key-for-development-only"

var (
	port    = flag.String("port", ":8080", "Address to listen on")
	resetDB = flag.Bool("reset-db", false, "Drop and recreate the database before starting")
	noExit  = flag.Bool("no-exit", false, "Keep serving after a database reset instead of exiting")
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		runMigrations(os.Args[2:])
		return
	}
	flag.Parse()

	services.LoadEnvVariables()
	security.InitializeEncryption(encryptionKey())

	if err := database.InitDB(); err != nil {
		log.Fatal(err)
	}

	if err := models.FixYNABTableSchema(database.DB); err != nil {
		log.Printf("Warning: error fixing YNAB table schema: %v", err)
	}

	isResetDB := os.Getenv("RESET_DB") == "true" || *resetDB
	if err := migrations.RunMigrations(database.DB, isResetDB); err != nil {
		log.Printf("Warning: failed to run migrations: %v", err)
	}
	if isResetDB && !*noExit {
		log.Println("Database reset completed. Exiting.")
		return
	}

	if err := middleware.InitializeFirebase(); err != nil {
		log.Printf("Warning: failed to initialize Firebase, token verification disabled: %v", err)
	}

	srv := &http.Server{
		Handler:      newRouter(),
		Addr:         *port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	log.Printf("Starting server on %s", *port)
	log.Fatal(srv.ListenAndServe())
}

// encryptionKey returns ENCRYPTION_KEY from the environment, falling back to an insecure development key.
func encryptionKey() string {
	if key := os.Getenv("ENCRYPTION_KEY"); key != "" {
		return key
	}
	log.Println("Warning: ENCRYPTION_KEY not set, using a default key. This is NOT secure for production!")
	return defaultDevEncryptionKey
}

// newRouter builds the HTTP router: API routes at both / and /api, with the built frontend served from ./dist.
func newRouter() *mux.Router {
	r := mux.NewRouter()
	r.Use(middleware.EnableCORS)

	registerRoutes(r)
	registerRoutes(r.PathPrefix("/api").Subrouter())

	r.PathPrefix("/assets/").Handler(http.FileServer(http.Dir("./dist")))
	r.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/assets/") {
			log.Printf("Serving index.html for path: %s", r.URL.Path)
		}
		http.ServeFile(w, r, "./dist/index.html")
	}).Methods("GET")

	return r
}

// registerRoutes sets up all API routes
func registerRoutes(r *mux.Router) {
	// Public routes (no auth required)
	r.HandleFunc("/health", handlers.HealthCheck).Methods("GET", "OPTIONS")

	protected := r.PathPrefix("").Subrouter()
	protected.Use(middleware.AuthMiddleware)

	// Transactions
	protected.HandleFunc("/transactions", handlers.GetTransactions).Methods("GET")
	protected.HandleFunc("/transactions", handlers.AddTransaction).Methods("POST")
	protected.HandleFunc("/transactions/unique-fields", handlers.GetUniqueTransactionFields).Methods("GET")
	protected.HandleFunc("/transactions/{id}", handlers.GetTransaction).Methods("GET")
	protected.HandleFunc("/transactions/{id}", handlers.UpdateTransaction).Methods("PUT")
	protected.HandleFunc("/transactions/{id}", handlers.DeleteTransaction).Methods("DELETE")

	// Categories
	protected.HandleFunc("/categories", handlers.GetCategories).Methods("GET")
	protected.HandleFunc("/categories", handlers.CreateCategory).Methods("POST")
	protected.HandleFunc("/categories/{id}", handlers.UpdateCategory).Methods("PUT")
	protected.HandleFunc("/categories/{id}", handlers.DeleteCategory).Methods("DELETE")

	// Users
	protected.HandleFunc("/users", handlers.GetUsers).Methods("GET")
	protected.HandleFunc("/users/sync", handlers.SyncFirebaseUser).Methods("POST")
	protected.HandleFunc("/users/{username}", handlers.GetUserByUsername).Methods("GET")
	protected.HandleFunc("/user/me", handlers.GetCurrentUser).Methods("GET")

	// YNAB
	protected.HandleFunc("/ynab/categories", handlers.GetYNABCategories).Methods("GET")
	protected.HandleFunc("/ynab/sync", handlers.SyncYNABTransaction).Methods("POST")
	protected.HandleFunc("/ynab/config", handlers.GetYNABConfig).Methods("GET")
	protected.HandleFunc("/ynab/config", handlers.UpdateYNABConfig).Methods("PUT")
	protected.HandleFunc("/ynab/sync/categories", handlers.SyncYNABCategories).Methods("POST")
	protected.HandleFunc("/reports/ynab-splits", handlers.GetYNABSplits).Methods("POST")

	// Diagnostics
	protected.HandleFunc("/diagnostic/db-check", handlers.CheckDatabaseHandler).Methods("GET")
	protected.HandleFunc("/diagnostic/transaction-categories", handlers.CheckTransactionCategories).Methods("GET")

	// Permissions and roles
	protected.HandleFunc("/permissions", handlers.GetUserPermissions).Methods("GET")
	protected.HandleFunc("/permissions", handlers.GrantPermission).Methods("POST")
	protected.HandleFunc("/permissions", handlers.RevokePermission).Methods("DELETE")
	protected.HandleFunc("/permissions/all", handlers.GetAllPermissions).Methods("GET")
	protected.HandleFunc("/roles", handlers.SetUserRole).Methods("POST")
	protected.HandleFunc("/roles/{userId}", handlers.GetUserRole).Methods("GET")

	// Saved filters
	protected.HandleFunc("/filters", handlers.GetSavedFilters).Methods("GET")
	protected.HandleFunc("/filters", handlers.CreateSavedFilter).Methods("POST")
	protected.HandleFunc("/filters/{id}", handlers.GetSavedFilter).Methods("GET")
	protected.HandleFunc("/filters/{id}", handlers.UpdateSavedFilter).Methods("PUT")
	protected.HandleFunc("/filters/{id}", handlers.DeleteSavedFilter).Methods("DELETE")

	// Custom reports
	protected.HandleFunc("/reports/custom", handlers.GetCustomReports).Methods("GET")
	protected.HandleFunc("/reports/custom", handlers.CreateCustomReport).Methods("POST")
	protected.HandleFunc("/reports/custom/{id}", handlers.GetCustomReport).Methods("GET")
	protected.HandleFunc("/reports/custom/{id}", handlers.UpdateCustomReport).Methods("PUT")
	protected.HandleFunc("/reports/custom/{id}", handlers.DeleteCustomReport).Methods("DELETE")
	protected.HandleFunc("/reports/custom/{id}/run", handlers.RunCustomReport).Methods("POST")

	// Settlements
	settlements := handlers.NewSettlementHandler(database.DB)
	protected.HandleFunc("/settlements", settlements.CreateSettlement).Methods("POST")
	protected.HandleFunc("/settlements", settlements.GetUserSettlements).Methods("GET")
	protected.HandleFunc("/settlements/monthly", settlements.GetSettlementsGroupedByMonth).Methods("GET")
	protected.HandleFunc("/settlements/report", settlements.GetSettlementReportData).Methods("GET")
	protected.HandleFunc("/settlements/apply-payment", settlements.ApplyOthersTransactionToDebt).Methods("POST")
	protected.HandleFunc("/settlements/{id}", settlements.GetSettlement).Methods("GET")
	protected.HandleFunc("/settlements/{id}/status", settlements.UpdateSettlementStatus).Methods("PUT")
	protected.HandleFunc("/settlements/{id}/apply", settlements.ApplyTransaction).Methods("POST")
	protected.HandleFunc("/settlements/{id}/transactions/{transactionId}", settlements.RemoveTransaction).Methods("DELETE")
	protected.HandleFunc("/settlements/{id}/available-transactions", settlements.GetAvailableSettlementTransactions).Methods("GET")
	protected.HandleFunc("/transactions/{transactionId}/settlements", settlements.GetTransactionSettlements).Methods("GET")
}

// runMigrations implements the `bennwallet migrate [--reset] [--dry-run]` subcommand.
func runMigrations(args []string) {
	cmd := flag.NewFlagSet("migrate", flag.ExitOnError)
	reset := cmd.Bool("reset", false, "Reset database before migrations (WARNING: DELETES ALL DATA)")
	dryRun := cmd.Bool("dry-run", false, "List migrations that would be applied without executing them")
	if err := cmd.Parse(args); err != nil {
		log.Fatalf("Failed to parse migration flags: %v", err)
	}
	if os.Getenv("RESET_DB") == "true" {
		*reset = true
	}

	services.LoadEnvVariables()

	db, err := database.CreatePostgresDB()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	inProduction := os.Getenv("APP_ENV") == "production" || os.Getenv("NODE_ENV") == "production"
	if *reset && inProduction && os.Getenv("CONFIRM_PROD_RESET") != "yes" {
		log.Fatal("Refusing to reset the PRODUCTION database. Set CONFIRM_PROD_RESET=yes to proceed.")
	}

	if *dryRun {
		pending, err := migrations.GetPendingMigrations(db)
		if err != nil {
			log.Fatalf("Failed to check pending migrations: %v", err)
		}
		if len(pending) == 0 {
			fmt.Println("Database schema is up to date. No migrations needed.")
			return
		}
		fmt.Println("Pending migrations:")
		for _, name := range pending {
			fmt.Printf("  - %s\n", name)
		}
		return
	}

	if err := migrations.RunMigrations(db, *reset); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	log.Println("Database migrations completed successfully")
}
