package entity

type WidgetCardsBoardSettings struct {
	Id                     int64  `json:"id"                  db:"id"`
	ChannelId              int64  `json:"channel_id"          db:"channel_id"`
	Orientation            string `json:"orientation"         db:"orientation"`
	ShowHorizontalRow      bool   `json:"show_horizontal_row"       db:"show_horizontal_row"`
	ShowOnlyAvailableTeams bool   `json:"show_only_available_teams" db:"show_only_available_teams"`
}
