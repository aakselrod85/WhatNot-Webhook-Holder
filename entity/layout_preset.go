package entity

import (
	"encoding/json"
	"time"
)

type LayoutPreset struct {
	Id        int64           `json:"id"         db:"id"`
	ChannelId int64           `json:"channel_id" db:"channel_id"`
	Name      string          `json:"name"       db:"name"`
	Config    json.RawMessage `json:"config"     db:"config"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt time.Time       `json:"updated_at" db:"updated_at"`
}
