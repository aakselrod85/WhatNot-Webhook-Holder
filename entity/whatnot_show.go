package entity

import "time"

// WhatnotShow is what a show (a holder stream, a row of mobwebapp's shows) keeps of its bound Whatnot show.
// Id is the show's id; ShowId is the Whatnot show id (shows.whatnot_show_id). Title is the Whatnot show's title
// (shows.whatnot_show_title) when stored, else the show's own title. The status is the show's (Stream.Status).
type WhatnotShow struct {
	Id            int64      `db:"id"`
	ChannelId     int64      `db:"channel_id"`
	ShowId        string     `db:"show_id"`
	Title         string     `db:"title"`
	Description   string     `db:"description"`
	ThumbnailUrl  string     `db:"thumbnail_url"`
	Visibility    string     `db:"visibility"`
	ScheduledDate *time.Time `db:"scheduled_date"`
	StartedAt     *time.Time `db:"started_at"`
	EndedAt       *time.Time `db:"ended_at"`
}
