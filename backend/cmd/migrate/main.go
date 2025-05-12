package main

import (
	"bennwallet/backend/database"
	"bennwallet/backend/migrations"
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	log.Println("Starting database migration tool...")

	// Parse command-line flags
	reset := flag.Bool("reset", false, "Reset database before migrations (WARNING: DELETES ALL DATA)")
	dryRun := flag.Bool("dry-run", false, "Check which migrations would be applied without executing them")
	flag.Parse()

	// Check if reset was requested through environment variable (for backward compatibility)
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
