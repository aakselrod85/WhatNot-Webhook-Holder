package repository

import "github.com/SaloEater/WhatNot-Webhook-Holder/entity"

type SeriesPriceRangeRepositorier interface {
	ListBySeries(seriesId int64) ([]*entity.SeriesPriceRange, error)
	Create(r *entity.SeriesPriceRange) (int64, error)
	Update(r *entity.SeriesPriceRange) error
	Delete(id int64) error
}
