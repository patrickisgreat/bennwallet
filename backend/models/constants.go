package models

// Resource types
const (
	ResourceTransactions = "transactions"
	ResourceCategories   = "categories"
	ResourceYnabConfig   = "ynab_config"
	ResourceUsers        = "users"
	ResourceReports      = "reports"
	ResourceAll          = "all"
)

// Permission types
const (
	PermissionRead  = "read"
	PermissionWrite = "write"
	PermissionAll   = "all"
)

// User roles
const (
	RoleUser       = "user"
	RoleAdmin      = "admin"
	RoleSuperAdmin = "superadmin"
)
