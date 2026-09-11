package service

import "github.com/SaloEater/WhatNot-Webhook-Holder/entity"

type SeriesPriceRangeUpdateRequest struct {
	Id        int64 `json:"id"`
	PriceFrom int   `json:"price_from"`
	PriceTo   *int  `json:"price_to"`
	Count     int   `json:"count"`
}

type SeriesPriceRangeUpdateResponse struct {
	Success bool `json:"success"`
}

func (s *Service) SeriesPriceRangeUpdate(r *SeriesPriceRangeUpdateRequest) (*SeriesPriceRangeUpdateResponse, error) {
	if err := validateSeriesPriceRange(r.PriceFrom, r.PriceTo, r.Count); err != nil {
		return nil, err
	}
	err := s.SeriesPriceRangeRepositorier.Update(&entity.SeriesPriceRange{
		Id:        r.Id,
		PriceFrom: r.PriceFrom,
		PriceTo:   r.PriceTo,
		Count:     r.Count,
	})
	if err != nil {
		return nil, err
	}
	return &SeriesPriceRangeUpdateResponse{Success: true}, nil
}
