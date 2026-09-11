package service

import (
	"errors"

	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
)

type SeriesPriceRangeCreateRequest struct {
	SeriesId  int64 `json:"series_id"`
	PriceFrom int   `json:"price_from"`
	PriceTo   *int  `json:"price_to"`
	Count     int   `json:"count"`
}

type SeriesPriceRangeCreateResponse struct {
	Id int64 `json:"id"`
}

func (s *Service) SeriesPriceRangeCreate(r *SeriesPriceRangeCreateRequest) (*SeriesPriceRangeCreateResponse, error) {
	if err := validateSeriesPriceRange(r.PriceFrom, r.PriceTo, r.Count); err != nil {
		return nil, err
	}
	id, err := s.SeriesPriceRangeRepositorier.Create(&entity.SeriesPriceRange{
		SeriesId:  r.SeriesId,
		PriceFrom: r.PriceFrom,
		PriceTo:   r.PriceTo,
		Count:     r.Count,
	})
	if err != nil {
		return nil, err
	}
	return &SeriesPriceRangeCreateResponse{Id: id}, nil
}

func validateSeriesPriceRange(priceFrom int, priceTo *int, count int) error {
	if priceFrom < 0 {
		return errors.New("price_from must be >= 0")
	}
	if priceTo != nil && *priceTo <= priceFrom {
		return errors.New("price_to must be greater than price_from")
	}
	if count < 0 {
		return errors.New("count must be >= 0")
	}
	return nil
}
