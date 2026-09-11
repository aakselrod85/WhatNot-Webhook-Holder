package service

import "github.com/SaloEater/WhatNot-Webhook-Holder/entity"

type SeriesPriceRangeListRequest struct {
	SeriesId int64 `json:"series_id"`
}

func (s *Service) SeriesPriceRangeList(r *SeriesPriceRangeListRequest) ([]*entity.SeriesPriceRange, error) {
	return s.SeriesPriceRangeRepositorier.ListBySeries(r.SeriesId)
}
