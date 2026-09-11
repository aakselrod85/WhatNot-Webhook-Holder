package entity

import "time"

type Series struct {
	Id           int64     `json:"id"         db:"id"`
	Name         string    `json:"name"       db:"name"`
	Status       string    `json:"status"     db:"status"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	IsDeleted    bool      `json:"is_deleted"   db:"is_deleted"`
	TotalCards   int64     `json:"total_cards"  db:"total_cards"`
	UsedCards    int64     `json:"used_cards"    db:"used_cards"`
	DefaultPrice string    `json:"default_price" db:"default_price"`
	Kind         string    `json:"kind"          db:"kind"`
}
