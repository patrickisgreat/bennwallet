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
		('UgwzWuP8iHNF8nhqDHMwFFcg8Sc2', 'Patrick Bennett', 'Patrick Bennett', 'superadmin', 'approved', true),
		('sarah-wallis-id', 'Sarah Wallis', 'Sarah Wallis', 'admin', 'approved', true),
		('kim-donaldson-id', 'Kim Donaldson', 'Kim Donaldson', 'admin', 'approved', true)
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
		{id: "pb-group-1", name: "Essentials", user_id: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2"},
		{id: "pb-group-2", name: "Lifestyle", user_id: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2"},
		{id: "sw-group-1", name: "Essentials", user_id: "sarah-wallis-id"},
		{id: "sw-group-2", name: "Lifestyle", user_id: "sarah-wallis-id"},
		{id: "kd-group-1", name: "Essentials", user_id: "kim-donaldson-id"},
		{id: "kd-group-2", name: "Monthly Bills", user_id: "kim-donaldson-id"},
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
		{id: "pb-cat-1", name: "Groceries", user_id: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", category_group: "pb-group-1"},
		{id: "pb-cat-2", name: "Housing", user_id: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", category_group: "pb-group-1"},
		{id: "pb-cat-3", name: "Transportation", user_id: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", category_group: "pb-group-1"},
		{id: "pb-cat-4", name: "Entertainment", user_id: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", category_group: "pb-group-2"},
		{id: "pb-cat-5", name: "Travel", user_id: "UgwzWuP8iHNF8nhqDHMwFFcg8Sc2", category_group: "pb-group-2"},
		{id: "sw-cat-1", name: "Groceries", user_id: "sarah-wallis-id", category_group: "sw-group-1"},
		{id: "sw-cat-2", name: "Rent", user_id: "sarah-wallis-id", category_group: "sw-group-1"},
		{id: "sw-cat-3", name: "Entertainment", user_id: "sarah-wallis-id", category_group: "sw-group-2"},
		{id: "sw-cat-4", name: "Dining Out", user_id: "sarah-wallis-id", category_group: "sw-group-2"},
		{id: "kd-cat-1", name: "Internet", user_id: "kim-donaldson-id", category_group: "kd-group-2"},
		{id: "kd-cat-2", name: "Electricity", user_id: "kim-donaldson-id", category_group: "kd-group-2"},
		{id: "kd-cat-3", name: "Groceries", user_id: "kim-donaldson-id", category_group: "kd-group-1"},
		{id: "kd-cat-4", name: "Transportation", user_id: "kim-donaldson-id", category_group: "kd-group-1"},
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
