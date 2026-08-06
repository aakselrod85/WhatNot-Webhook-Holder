package service

import "github.com/SaloEater/WhatNot-Webhook-Holder/cache"

type GetWidgetSeriesStashorpassRequest struct {
	ChannelId int64 `json:"channel_id"`
}

type GetWidgetSeriesStashorpassResponse struct {
	ChannelId int64 `json:"channel_id"`
	Price     int   `json:"price"`
}

func (s *Service) GetWidgetSeriesStashorpass(r *GetWidgetSeriesStashorpassRequest) (*GetWidgetSeriesStashorpassResponse, error) {
	key := cache.IdToKey(r.ChannelId)
	if s.WidgetSeriesStashorpassCache.Has(key) {
		cached, _ := s.WidgetSeriesStashorpassCache.Get(key)
		return &GetWidgetSeriesStashorpassResponse{ChannelId: cached.ChannelId, Price: cached.Price}, nil
	}
	w, err := s.WidgetSeriesStashorpassRepositorier.GetByChannel(r.ChannelId)
	if err != nil {
		return nil, err
	}
	s.WidgetSeriesStashorpassCache.Set(key, w)
	return &GetWidgetSeriesStashorpassResponse{
		ChannelId: w.ChannelId,
		Price:     w.Price,
	}, nil
}
