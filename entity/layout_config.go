package entity

import (
	"encoding/json"
	"time"
)

type LayoutConfig struct {
	Id        int64           `json:"id"         db:"id"`
	ChannelId int64           `json:"channel_id" db:"channel_id"`
	Config    json.RawMessage `json:"config"     db:"config"`
	UpdatedAt time.Time       `json:"updated_at" db:"updated_at"`
}
