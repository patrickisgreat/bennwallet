package testutil

import (
	"database/sql"
)

// SeedTestData seeds all test data into the database
func SeedTestData(db *sql.DB) error {
	// Insert test users
	_, err := db.Exec(`
		INSERT INTO users (id, username, name, role, status, is_admin) 
		VALUES 
		('test-user-id', 'testuser', 'Test User', 'admin', 'approved', true),
		('1', 'sarah', 'Sarah', 'admin', 'approved', true),
		('2', 'patrick', 'Patrick', 'admin', 'approved', true),
		('admin1', 'admin1', 'Admin One', 'admin', 'approved', true)
		ON CONFLICT (id) DO NOTHING
	`)
	if err != nil {
		return err
	}

	// Create YNAB category groups table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ynab_category_groups (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			category_group_id TEXT,
			user_id TEXT NOT NULL,
			hidden BOOLEAN DEFAULT false,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Create YNAB categories table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ynab_categories (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			group_id TEXT,
			category_group_id TEXT,
			user_id TEXT NOT NULL,
			hidden BOOLEAN DEFAULT false,
			budget_amount DECIMAL(15,2),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Define test category groups
	categoryGroups := []struct {
		id      string
		name    string
		user_id string
	}{
		{id: "test-group-1", name: "Essentials", user_id: TestUserID},
		{id: "test-group-2", name: "Lifestyle", user_id: TestUserID},
		{id: "test-group-3", name: "Monthly Bills", user_id: TestUserID},
	}

	// Insert category groups
	for _, group := range categoryGroups {
		_, err := db.Exec(`
			INSERT INTO ynab_category_groups (id, name, category_group_id, user_id, hidden)
			VALUES ($1, $2, $1, $3, $4)
			ON CONFLICT (id) DO UPDATE SET
			name = $2,
			category_group_id = $1
		`, group.id, group.name, group.user_id, false)

		if err != nil {
			return err
		}
	}

	// Define test categories
	categories := []struct {
		id             string
		name           string
		user_id        string
		category_group string
	}{
		{id: "test-cat-1", name: "Groceries", user_id: TestUserID, category_group: "test-group-1"},
		{id: "test-cat-2", name: "Rent", user_id: TestUserID, category_group: "test-group-1"},
		{id: "test-cat-3", name: "Entertainment", user_id: TestUserID, category_group: "test-group-2"},
		{id: "test-cat-4", name: "Dining Out", user_id: TestUserID, category_group: "test-group-2"},
		{id: "test-cat-5", name: "Internet", user_id: TestUserID, category_group: "test-group-3"},
		{id: "test-cat-6", name: "Electricity", user_id: TestUserID, category_group: "test-group-3"},
	}

	// Insert categories with both group_id and category_group_id set
	for _, cat := range categories {
		_, err := db.Exec(`
			INSERT INTO ynab_categories (id, name, group_id, category_group_id, user_id, hidden, budget_amount)
			VALUES ($1, $2, $3, $3, $4, $5, $6)
			ON CONFLICT (id) DO UPDATE SET
			name = $2,
			group_id = $3,
			category_group_id = $3
		`, cat.id, cat.name, cat.category_group, cat.user_id, false, 0.0)

		if err != nil {
			return err
		}
	}

	return nil
}
