package migrations

import (
	"database/sql"
	"fmt"
	"log"

	"bennwallet/backend/models"
	"bennwallet/backend/security"
)

// FixYNABSchema ensures the YNAB config table has the correct schema
// This is particularly important for fixing schema issues in production
func FixYNABSchema(db *sql.DB) error {
	log.Println("Running YNAB schema fix migration...")

	// First, check if YNAB config table exists
	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'ynab_config'
		)
	`).Scan(&exists)

	if err != nil {
		return fmt.Errorf("error checking if table exists: %w", err)
	}

	if !exists {
		log.Println("ynab_config table doesn't exist, nothing to fix")
		return nil
	}

	// Check column constraints - specifically look for NOT NULL constraints
	log.Println("Checking column constraints...")

	// Get column constraints info for api_token
	var isNotNull bool
	err = db.QueryRow(`
		SELECT is_nullable = 'NO'
		FROM information_schema.columns
		WHERE table_schema = 'public'
		AND table_name = 'ynab_config'
		AND column_name = 'api_token'
	`).Scan(&isNotNull)

	if err != nil {
		log.Printf("Error checking api_token constraint: %v", err)
	} else if isNotNull {
		log.Println("Found NOT NULL constraint on api_token, dropping constraint...")

		// Start a transaction for the constraint changes
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("error starting transaction: %w", err)
		}

		// First check if any data needs to be migrated from unencrypted to encrypted columns
		log.Println("Checking if we need to migrate data to encrypted columns...")

		rows, err := tx.Query(`
			SELECT user_id, api_token, budget_id, account_id
			FROM ynab_config
			WHERE api_token IS NOT NULL 
			AND (encrypted_api_token IS NULL OR encrypted_api_token = '')
		`)

		if err != nil {
			tx.Rollback()
			log.Printf("Error querying for data to migrate: %v", err)
		} else {
			// Migrate any data that needs migration
			for rows.Next() {
				var userID, apiToken, budgetID, accountID string
				if err := rows.Scan(&userID, &apiToken, &budgetID, &accountID); err != nil {
					log.Printf("Error scanning row: %v", err)
					continue
				}

				log.Printf("Migrating data for user %s to encrypted columns", userID)

				// Encrypt the values
				encryptedToken, err := security.Encrypt(apiToken)
				if err != nil {
					log.Printf("Error encrypting token: %v", err)
					continue
				}

				encryptedBudgetID, err := security.Encrypt(budgetID)
				if err != nil {
					log.Printf("Error encrypting budget ID: %v", err)
					continue
				}

				encryptedAccountID, err := security.Encrypt(accountID)
				if err != nil {
					log.Printf("Error encrypting account ID: %v", err)
					continue
				}

				// Update the row with encrypted values
				_, err = tx.Exec(`
					UPDATE ynab_config
					SET encrypted_api_token = $1,
						encrypted_budget_id = $2,
						encrypted_account_id = $3
					WHERE user_id = $4
				`, encryptedToken, encryptedBudgetID, encryptedAccountID, userID)

				if err != nil {
					log.Printf("Error updating encrypted columns for user %s: %v", userID, err)
				} else {
					log.Printf("Successfully migrated data for user %s", userID)
				}
			}
			rows.Close()
		}

		// Now alter the columns to remove NOT NULL constraints
		log.Println("Altering columns to remove NOT NULL constraints...")

		// Drop NOT NULL constraints
		_, err = tx.Exec(`
			ALTER TABLE ynab_config 
			ALTER COLUMN api_token DROP NOT NULL,
			ALTER COLUMN budget_id DROP NOT NULL,
			ALTER COLUMN account_id DROP NOT NULL
		`)

		if err != nil {
			tx.Rollback()
			return fmt.Errorf("error dropping NOT NULL constraints: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("error committing transaction: %w", err)
		}

		log.Println("Successfully removed NOT NULL constraints")
	}

	// Check if we need to handle a foreign key constraint
	var hasForeignKey bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.table_constraints 
			WHERE constraint_name = 'ynab_config_user_id_fkey' 
			AND table_name = 'ynab_config'
		)
	`).Scan(&hasForeignKey)

	if err != nil {
		log.Printf("Error checking for foreign key constraint: %v", err)
	} else if hasForeignKey {
		log.Println("Found foreign key constraint on user_id, checking for orphaned records...")

		// Check if we have any entries that would violate the constraint
		rows, err := db.Query(`
			SELECT y.user_id
			FROM ynab_config y
			LEFT JOIN users u ON y.user_id = u.id
			WHERE u.id IS NULL
		`)

		if err != nil {
			log.Printf("Error checking for orphaned records: %v", err)
		} else {
			// Create users for any orphaned records
			tx, err := db.Begin()
			if err != nil {
				log.Printf("Error starting transaction: %v", err)
			} else {
				orphanedUsers := make([]string, 0)
				for rows.Next() {
					var userID string
					if err := rows.Scan(&userID); err != nil {
						log.Printf("Error scanning user ID: %v", err)
						continue
					}
					orphanedUsers = append(orphanedUsers, userID)
				}
				rows.Close()

				if len(orphanedUsers) > 0 {
					log.Printf("Found %d orphaned records, creating users...", len(orphanedUsers))

					for _, userID := range orphanedUsers {
						_, err = tx.Exec(`
							INSERT INTO users (id, username, name, role)
							VALUES ($1, $2, $3, 'user')
							ON CONFLICT (id) DO NOTHING
						`, userID, fmt.Sprintf("user_%s", userID), fmt.Sprintf("User %s", userID))

						if err != nil {
							log.Printf("Error creating user for orphaned record: %v", err)
						}
					}

					if err := tx.Commit(); err != nil {
						log.Printf("Error committing transaction: %v", err)
						tx.Rollback()
					} else {
						log.Println("Successfully created users for orphaned records")
					}
				} else {
					log.Println("No orphaned records found")
					tx.Rollback()
				}
			}
		}
	}

	// Now run the normal schema fix function
	err = models.FixYNABTableSchema(db)
	if err != nil {
		return fmt.Errorf("failed to fix YNAB schema: %w", err)
	}

	log.Println("Successfully completed YNAB schema fix migration")
	return nil
}
