package models

// User represents a user in the system
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Status   string `json:"status"` // pending, approved, rejected
	IsAdmin  bool   `json:"isAdmin"`
	Role     string `json:"role"` // admin, user (for future roles)
}

// UserPermission was removed as it's now declared in permission.go
