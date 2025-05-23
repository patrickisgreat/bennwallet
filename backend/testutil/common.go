package testutil

import (
	"os"
)

// TestUserID is the ID used for testing
const TestUserID = "test-user-id"

// getEnvOrDefault gets environment variable or returns default
func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
