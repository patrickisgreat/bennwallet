package models

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Status   string `json:"status"` // pending, approved, rejected
	IsAdmin  bool   `json:"isAdmin"`
}
