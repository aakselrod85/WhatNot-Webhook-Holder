package entity

import "time"

type LayoutImage struct {
	Id        int64     `json:"id"         db:"id"`
	ChannelId int64     `json:"channel_id" db:"channel_id"`
	Name      string    `json:"name"       db:"name"`
	Url       string    `json:"url"        db:"url"`
	Width     int       `json:"width"      db:"width"`
	Height    int       `json:"height"     db:"height"`
	SizeBytes int64     `json:"size_bytes" db:"size_bytes"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
