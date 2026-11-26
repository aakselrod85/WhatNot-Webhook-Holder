package entity

import "time"

type Break struct {
	Id int64 `json:"id" db:"id"`

	DayId        int64  `json:"day_id" db:"day_id"`
	Name         string `json:"name" db:"name"`
	HighBidTeam  string `json:"high_bid_team" db:"high_bid_team"`
	GiveawayTeam string `json:"giveaway_team" db:"giveaway_team"`
	HighBidFloor int    `json:"high_bid_floor" db:"high_bid_floor"`

	StartDate time.Time `json:"start_date" db:"start_date"`
	EndDate   time.Time `json:"end_date" db:"end_date"`
	IsDeleted bool      `json:"is_deleted" db:"is_deleted"`
	SeriesId  *int64    `json:"series_id"  db:"series_id"`
	// Number is show_breaks.number, the order of breaks inside a show. Not serialized.
	Number int `json:"-" db:"number"`
}
