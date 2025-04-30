package models

type Category struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color,omitempty"`
	UserID      string `json:"userId"`
}
