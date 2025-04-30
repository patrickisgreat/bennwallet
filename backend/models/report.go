package models

type ReportFilter struct {
	StartDate string `json:"startDate,omitempty"`
	EndDate   string `json:"endDate,omitempty"`
	Category  string `json:"category,omitempty"`
	PayTo     string `json:"payTo,omitempty"`
	EnteredBy string `json:"enteredBy,omitempty"`
	Paid      *bool  `json:"paid,omitempty"`
	UserId    string `json:"userId,omitempty"`
}

type CategoryTotal struct {
	Category string  `json:"category"`
	Total    float64 `json:"total"`
}
