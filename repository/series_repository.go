package repository

import "github.com/SaloEater/WhatNot-Webhook-Holder/entity"

type SeriesRepositorier interface {
	Create(name string, totalCards int64, defaultPrice string, kind string) (int64, error)
	Get(id int64) (*entity.Series, error)
	GetList() ([]*entity.Series, error)
	GetListPaginated(limit int64, offset int64) ([]*entity.Series, error)
	Update(id int64, name string, usedCards int64, defaultPrice string, totalCards int64) error
	Close(id int64) error
	Delete(id int64) error
	CountOpen() (int, error)
	CountActive() (int64, error)
}
