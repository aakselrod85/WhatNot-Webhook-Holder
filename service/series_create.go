package service

import (
	"errors"

	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
)

type SeriesCreateRequest struct {
	Name         string `json:"name"`
	TotalCards   int64  `json:"total_cards"`
	DefaultPrice string `json:"default_price"`
	Kind         string `json:"kind"`
}

type SeriesCreateResponse struct {
	Id           int64  `json:"id"`
	Name         string `json:"name"`
	TotalCards   int64  `json:"total_cards"`
	DefaultPrice string `json:"default_price"`
	Kind         string `json:"kind"`
}

func (s *Service) SeriesCreate(r *SeriesCreateRequest) (*SeriesCreateResponse, error) {
	kind := r.Kind
	if kind == "" {
		kind = entity.SeriesKindCards
	}
	if !entity.IsValidSeriesKind(kind) {
		return nil, errors.New("invalid series kind: " + kind)
	}
	id, err := s.SeriesRepositorier.Create(r.Name, r.TotalCards, r.DefaultPrice, kind)
	if err != nil {
		return nil, err
	}
	return &SeriesCreateResponse{Id: id, Name: r.Name, TotalCards: r.TotalCards, DefaultPrice: r.DefaultPrice, Kind: kind}, nil
}
