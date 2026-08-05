package repository

import "github.com/SaloEater/WhatNot-Webhook-Holder/entity"

type PhotoRepositorier interface {
	Create(p *entity.Photo) (int64, error)
	GetById(id int64) (*entity.Photo, error)
	GetBySeriesId(seriesId int64) ([]*entity.Photo, error)
	GetUnsoldByChannelActiveStream(channelId int64, withSold bool) ([]*entity.Photo, error)
	Update(id int64, name, team string, price int64) error
	UpdateUrl(id int64, url string) error
	UpdateRotation(id int64, rotation int64) error
	UpdateThumbnail(id int64, thumbnail string) error
	MarkSold(id int64, sold bool) error
	Delete(id int64) error
	Restore(id int64) error
	GetPricesBySeriesId(seriesId int64) ([]*entity.SeriesTeamTotal, error)
	CountUnsoldBySeriesId(seriesId int64) (int64, error)
	CountSoldBySeriesId(seriesId int64) (int64, error)
}
