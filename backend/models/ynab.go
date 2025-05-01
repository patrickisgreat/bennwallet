package models

type YNABCategoryGroup struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Hidden  bool   `json:"hidden"`
	Deleted bool   `json:"deleted"`
}

type YNABCategory struct {
	ID                string `json:"id"`
	CategoryGroupID   string `json:"category_group_id"`
	CategoryGroupName string `json:"category_group_name"`
	Name              string `json:"name"`
	Hidden            bool   `json:"hidden"`
	Deleted           bool   `json:"deleted"`
}

type YNABCategoryResponse struct {
	Data struct {
		CategoryGroups []struct {
			ID         string         `json:"id"`
			Name       string         `json:"name"`
			Hidden     bool           `json:"hidden"`
			Deleted    bool           `json:"deleted"`
			Categories []YNABCategory `json:"categories"`
		} `json:"category_groups"`
	} `json:"data"`
}
