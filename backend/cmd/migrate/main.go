package main

import (
	"fmt"
	"log"
	"os"

	"bennwallet/backend/database"
	"bennwallet/backend/migrations"
)

func main() {
	// Initialize database connection
	err := database.InitDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Set RESET_DB environment variable to force recreating test data
	// This will ensure all test users have categories
	os.Setenv("RESET_DB", "true")

	// Run migrations with resetDB=true to force dropping tables and recreating
	err = migrations.RunMigrations(database.DB, true)
	if err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	fmt.Println("Migrations completed successfully!")
	os.Exit(0)
}
