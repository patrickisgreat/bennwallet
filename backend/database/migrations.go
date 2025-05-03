package database

import (
	"log"

	"bennwallet/backend/migrations"
)

// RunMigrations runs all database migrations
func RunMigrations() error {
	log.Println("Running database migrations...")

	// Run all migrations from the migrations package
	if err := migrations.RunMigrations(DB); err != nil {
		log.Printf("Error running migrations: %v", err)
		return err
	}

	log.Println("Database migrations completed successfully")
	return nil
}
