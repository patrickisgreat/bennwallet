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

	// Run migrations
	err = migrations.RunMigrations(database.DB)
	if err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	fmt.Println("Migrations completed successfully!")
	os.Exit(0)
}
