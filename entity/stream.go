package entity

import "time"

// A Stream is a row of mobwebapp's shows table (see CLAUDE.md). WhatnotShowId is shows.whatnot_show_id.
// WhatnotShowName is shows.whatnot_show_title, null when empty.
type Stream struct {
	Id              int64     `db:"id" json:"id"`
	Name            string    `db:"name" json:"name"`
	ChannelId       int64     `json:"channel_id" db:"channel_id"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
	IsDeleted       bool      `json:"is_deleted" db:"is_deleted"`
	IsEnded         bool      `json:"is_ended" db:"is_ended"`
	ActiveBreakId   *int64    `json:"active_break_id" db:"active_break_id"`
	WhatnotShowId   *string   `json:"whatnot_show_id" db:"whatnot_show_id"`
	WhatnotShowName *string   `json:"whatnot_show_name" db:"whatnot_show_name"`
	Status          string    `json:"status" db:"status"`
}

const (
	StreamStatusPlanned  = "planned"
	StreamStatusLive     = "live"
	StreamStatusEnded    = "ended"
	StreamStatusVerified = "verified"
	// StreamStatusCancelled is a show Whatnot cancelled before it started.
	StreamStatusCancelled = "cancelled"
	// StreamStatusFailedToVerify sits between ended and verified: the last real reconciliation was needs_manual or failed.
	StreamStatusFailedToVerify = "failed_to_verify"
)

type StreamStatistic struct {
	Name            string   `db:"name"`
	BreaksAmount    int      `db:"breaks_amount"`
	SoldFor         int      `db:"sold_for"`
	CustomersAmount int      `db:"customers_amount"`
	ChannelName     string   `db:"channel_name"`
	BigCustomers    []string `db:"big_customers"`
	LuckyGoblins    []string `db:"lucky_goblins"`
}

type StreamEnriched struct {
	*Stream
	ChannelName string `db:"channel_name"`
}

// WhatnotShowContent is what a show keeps of its bound Whatnot show.
type WhatnotShowContent struct {
	Title        string // the Whatnot show's own title (shows.whatnot_show_title)
	Description  string
	ThumbnailUrl string
	Visibility   string
	ScheduledAt  *time.Time
	EndedAt      *time.Time
}
