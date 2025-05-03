package models

import (
	"testing"
	"time"
)

func TestDefaultAdmins(t *testing.T) {
	// Test that default admins are set correctly
	if len(DefaultAdmins) != 2 {
		t.Errorf("Expected 2 default admins, got %d", len(DefaultAdmins))
	}

	// Test that Sarah and Patrick are in the default admins list
	foundSarah := false
	foundPatrick := false
	for _, admin := range DefaultAdmins {
		if admin == "Sarah" {
			foundSarah = true
		}
		if admin == "Patrick" {
			foundPatrick = true
		}
	}

	if !foundSarah {
		t.Error("Expected 'Sarah' to be a default admin")
	}
	if !foundPatrick {
		t.Error("Expected 'Patrick' to be a default admin")
	}
}

func TestUser(t *testing.T) {
	// Create a test user
	user := User{
		ID:       "test-id",
		Username: "testuser",
		Name:     "Test User",
		Status:   "approved",
		IsAdmin:  true,
		Role:     "admin",
	}

	// Test ID
	if user.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got '%s'", user.ID)
	}

	// Test Username
	if user.Username != "testuser" {
		t.Errorf("Expected Username 'testuser', got '%s'", user.Username)
	}

	// Test Name
	if user.Name != "Test User" {
		t.Errorf("Expected Name 'Test User', got '%s'", user.Name)
	}

	// Test Status
	if user.Status != "approved" {
		t.Errorf("Expected Status 'approved', got '%s'", user.Status)
	}

	// Test IsAdmin
	if !user.IsAdmin {
		t.Error("Expected IsAdmin to be true")
	}

	// Test Role
	if user.Role != "admin" {
		t.Errorf("Expected Role 'admin', got '%s'", user.Role)
	}
}

func TestPermission(t *testing.T) {
	// Create a test permission
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)

	permission := Permission{
		ID:             "perm-id",
		OwnerUserID:    "owner-id",
		GrantedUserID:  "granted-id",
		PermissionType: PermissionWrite,
		ResourceType:   ResourceTransactions,
		CreatedAt:      now,
		ExpiresAt:      expiresAt,
	}

	// Test ID
	if permission.ID != "perm-id" {
		t.Errorf("Expected ID 'perm-id', got '%s'", permission.ID)
	}

	// Test OwnerUserID
	if permission.OwnerUserID != "owner-id" {
		t.Errorf("Expected OwnerUserID 'owner-id', got '%s'", permission.OwnerUserID)
	}

	// Test GrantedUserID
	if permission.GrantedUserID != "granted-id" {
		t.Errorf("Expected GrantedUserID 'granted-id', got '%s'", permission.GrantedUserID)
	}

	// Test PermissionType
	if permission.PermissionType != PermissionWrite {
		t.Errorf("Expected PermissionType '%s', got '%s'", PermissionWrite, permission.PermissionType)
	}

	// Test ResourceType
	if permission.ResourceType != ResourceTransactions {
		t.Errorf("Expected ResourceType '%s', got '%s'", ResourceTransactions, permission.ResourceType)
	}

	// Test CreatedAt
	if !permission.CreatedAt.Equal(now) {
		t.Errorf("Expected CreatedAt '%v', got '%v'", now, permission.CreatedAt)
	}

	// Test ExpiresAt
	if !permission.ExpiresAt.Equal(expiresAt) {
		t.Errorf("Expected ExpiresAt '%v', got '%v'", expiresAt, permission.ExpiresAt)
	}
}

func TestPermissionConstants(t *testing.T) {
	// Test PermissionType constants
	if PermissionRead != "read" {
		t.Errorf("Expected PermissionRead to be 'read', got '%s'", PermissionRead)
	}
	if PermissionWrite != "write" {
		t.Errorf("Expected PermissionWrite to be 'write', got '%s'", PermissionWrite)
	}

	// Test ResourceType constants
	if ResourceTransactions != "transactions" {
		t.Errorf("Expected ResourceTransactions to be 'transactions', got '%s'", ResourceTransactions)
	}
	if ResourceCategories != "categories" {
		t.Errorf("Expected ResourceCategories to be 'categories', got '%s'", ResourceCategories)
	}
	if ResourceReports != "reports" {
		t.Errorf("Expected ResourceReports to be 'reports', got '%s'", ResourceReports)
	}
	if ResourceAll != "all" {
		t.Errorf("Expected ResourceAll to be 'all', got '%s'", ResourceAll)
	}
}
