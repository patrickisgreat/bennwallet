package models

import "time"

// User represents a user in the system
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Status   string `json:"status"` // pending, approved, rejected
	IsAdmin  bool   `json:"isAdmin"`
	Role     string `json:"role"` // admin, user (for future roles)
}

// DefaultUsers who should always be admins
var DefaultAdmins = []string{
	"Sarah",
	"Patrick",
}

// Permission represents a permission granted from one user to another
type Permission struct {
	ID             string    `json:"id"`
	OwnerUserID    string    `json:"ownerUserId"`    // User who owns the data
	GrantedUserID  string    `json:"grantedUserId"`  // User who is granted access
	PermissionType string    `json:"permissionType"` // read, write
	ResourceType   string    `json:"resourceType"`   // transactions, categories, etc.
	CreatedAt      time.Time `json:"createdAt"`
	ExpiresAt      time.Time `json:"expiresAt,omitempty"` // Optional expiration date
}

// PermissionType constants
const (
	PermissionRead  = "read"
	PermissionWrite = "write"
)

// ResourceType constants
const (
	ResourceTransactions = "transactions"
	ResourceCategories   = "categories"
	ResourceReports      = "reports"
	ResourceAll          = "all"
)
