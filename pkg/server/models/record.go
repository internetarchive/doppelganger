package models

type Record struct {
	ID   string `json:"id" db:"id"`
	URI  string `json:"uri" db:"uri"`
	Date int64  `json:"date" db:"date"`
	SHA1 string `json:"sha1" db:"sha1"`
	Size int64  `json:"size" db:"size"`
}
