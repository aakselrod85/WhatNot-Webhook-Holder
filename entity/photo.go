package entity

import "time"

type Photo struct {
	Id        int64     `json:"id"         db:"id"`
	SeriesId  int64     `json:"series_id"  db:"series_id"`
	Name      string    `json:"name"       db:"name"`
	Team      string    `json:"team"       db:"team"`
	Url       string    `json:"url"        db:"url"`
	Thumbnail string    `json:"thumbnail"  db:"thumbnail"`
	IsSold    bool      `json:"is_sold"    db:"is_sold"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	IsDeleted bool      `json:"is_deleted" db:"is_deleted"`
	Price     int64     `json:"price"      db:"price"`
	Rotation  int64     `json:"rotation"   db:"rotation"`
}
