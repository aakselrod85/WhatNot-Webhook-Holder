package repository

import "github.com/SaloEater/WhatNot-Webhook-Holder/entity"

type ChannelRepositorier interface {
	GetAll() ([]*entity.Channel, error)
	Get(int64) (*entity.Channel, error)
	// SetActiveStream writes only active_show_id.
	SetActiveStream(id int64, streamId *int64) error
	// UpdateSettings writes only name and the default high bid team and floor.
	UpdateSettings(id int64, name string, defaultHighBidTeam string, defaultHighBidFloor int) error
	GetByStream(int64) (*entity.Channel, error)
}
