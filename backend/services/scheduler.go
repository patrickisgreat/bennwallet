package services

import (
	"log"
	"time"
)

// StartScheduler starts the task scheduler for periodic tasks
func StartScheduler() {
	log.Println("Starting task scheduler...")

	// Schedule YNAB sync to run daily at midnight
	go startYNABSyncScheduler()
}

// startYNABSyncScheduler runs YNAB sync on a daily schedule
func startYNABSyncScheduler() {
	for {
		// Calculate time until midnight
		now := time.Now()
		midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		timeUntilMidnight := midnight.Sub(now)

		log.Printf("Next YNAB sync scheduled in %v", timeUntilMidnight)

		// Sleep until midnight
		time.Sleep(timeUntilMidnight)

		// Run YNAB sync for all users
		log.Println("Running scheduled YNAB sync...")
		SyncAllUsersYNABCategories()

		// Small delay to ensure we don't run multiple times if execution is very quick
		time.Sleep(time.Second)
	}
}
