package repository

import (
	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
)

type BreakRepositorier interface {
	Get(int64) (*entity.Break, error)
	Delete(int64) error
	// Update writes the dates, high bid team, giveaway team and floor; never the title.
	Update(p *entity.Break) error
	// Rename writes only the title.
	Rename(id int64, name string) error
	GetBreaksByStreamId(int64) ([]*entity.Break, error)
	SetSeries(breakID int64, seriesID *int64) error
	// GetActiveSeriesIdByChannel returns the series of the channel's active break; nil, nil when there is none.
	GetActiveSeriesIdByChannel(channelId int64) (*int64, error)
}
