package entity

import (
	"encoding/json"
	"time"
)

type OverlayState struct {
	Id        int64           `json:"id"         db:"id"`
	ChannelId int64           `json:"channel_id" db:"channel_id"`
	Seq       int64           `json:"seq"        db:"seq"`
	State     json.RawMessage `json:"state"      db:"state"`
	UpdatedAt time.Time       `json:"updated_at" db:"updated_at"`
}
