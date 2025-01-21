package models

type Record struct {
	ID   string `json:"id" db:"id"`
	URI  string `json:"uri" db:"uri"`
	Date string `json:"date" db:"date"`
}
