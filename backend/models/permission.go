package models

import (
	"time"
)

// Permission represents a permission granted to a user
type Permission struct {
	ID             string    `json:"id"`
	GrantedUserID  string    `json:"grantedUserId"`
	OwnerUserID    string    `json:"ownerUserId"`
	ResourceType   string    `json:"resourceType"`
	PermissionType string    `json:"permissionType"`
	CreatedAt      time.Time `json:"createdAt"`
	ExpiresAt      time.Time `json:"expiresAt,omitempty"`
}
