package entity

import "time"

const (
	DiscordViewOverview       = "overview"
	DiscordViewStats          = "stats"
	DiscordViewReconciliation = "reconciliation"
)

type DiscordShowMessage struct {
	ShowId           int64     `json:"show_id" db:"whatnot_show_id"` // the mobwebapp show id (column name kept)
	View             string    `json:"view" db:"view"`
	DiscordChannelId *string   `json:"discord_channel_id" db:"discord_channel_id"`
	DiscordMessageId *string   `json:"discord_message_id" db:"discord_message_id"`
	Pinned           bool      `json:"pinned" db:"pinned"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

type DiscordFeedChannel struct {
	ChannelId        int64     `json:"channel_id" db:"channel_id"`
	DiscordChannelId string    `json:"discord_channel_id" db:"discord_channel_id"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
}
