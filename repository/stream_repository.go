package repository

import "github.com/SaloEater/WhatNot-Webhook-Holder/entity"

type StreamRepositorier interface {
	GetAll() ([]*entity.Stream, error)
	Get(int64) (*entity.Stream, error)
	Delete(int64) error
	Update(*entity.Stream) error
	Create(*entity.Stream) (int64, error)
	GetUsernames(int64) ([]string, error)
	GetAllByChannelId(int64) ([]*entity.Stream, error)
	GetStats(int64) (*entity.StreamStatistic, error)
	GetEnriched(int64) (*entity.StreamEnriched, error)
	GetByChannelIdPaginated(channelId int64, limit int64, offset int64) ([]*entity.Stream, error)
	CountByChannelId(channelId int64) (int64, error)
}
