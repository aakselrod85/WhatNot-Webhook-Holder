package repository_sqlx

import (
	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
	"github.com/jmoiron/sqlx"
)

// ChannelRepository works on mobwebapp's channels table (the second connection pool).
type ChannelRepository struct {
	DB *sqlx.DB
}

// channelCols maps channels onto entity.Channel. Channels have no soft delete. key, manual_hits and sort are never touched.
const channelCols = `c.id, c.name, false AS is_deleted, c.active_show_id AS active_stream_id,
	c.default_high_bid_team, c.default_high_bid_floor, c.whatnot_api_key, c.whatnot_webhook_secret`

func (r *ChannelRepository) GetAll() ([]*entity.Channel, error) {
	channels := []*entity.Channel{}
	err := r.DB.Unsafe().Select(&channels, `SELECT `+channelCols+` FROM channels c ORDER BY c.id`)
	return channels, err
}

func (r *ChannelRepository) Get(id int64) (*entity.Channel, error) {
	var channel entity.Channel
	err := r.DB.Unsafe().Get(&channel, `SELECT `+channelCols+` FROM channels c WHERE c.id = $1`, id)
	return &channel, err
}

func (r *ChannelRepository) GetByStream(streamId int64) (*entity.Channel, error) {
	var channel entity.Channel
	err := r.DB.Unsafe().Get(&channel, `SELECT `+channelCols+` FROM channels c INNER JOIN shows s ON c.id = s.channel_id WHERE s.id = $1`, streamId)
	return &channel, err
}

func (r *ChannelRepository) SetActiveStream(id int64, streamId *int64) error {
	_, err := r.DB.Exec(`UPDATE channels SET active_show_id = $2 WHERE id = $1`, id, streamId)
	return err
}

func (r *ChannelRepository) UpdateSettings(id int64, name string, defaultHighBidTeam string, defaultHighBidFloor int) error {
	_, err := r.DB.Exec(`UPDATE channels SET name = $2, default_high_bid_team = $3, default_high_bid_floor = $4 WHERE id = $1`,
		id, name, defaultHighBidTeam, defaultHighBidFloor)
	return err
}
