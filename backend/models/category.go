package models

type Category struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	Color           string `json:"color,omitempty"`
	UserID          string `json:"userId"`
	CategoryGroupID string `json:"categoryGroupId,omitempty"`
	Hidden          bool   `json:"hidden,omitempty"`
}
