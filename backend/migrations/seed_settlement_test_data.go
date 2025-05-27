package migrations

import (
	"database/sql"
	"log"
)

// AddSettlementTestData adds test transactions that create realistic debt scenarios for testing settlements
func AddSettlementTestData(db *sql.DB) error {
	// Only run in development/test environments
	if isProduction() {
		log.Println("⛔ REFUSING to seed settlement test data in production environment")
		return nil
	}

	log.Println("🧪 Settlement test data is now included in UpdateSeedDataForDebtTracking")
	log.Println("No additional settlement test data needed - using simplified 3-user system")

	return nil
}
