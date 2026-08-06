package service

import "github.com/SaloEater/WhatNot-Webhook-Holder/cache"

type GetWidgetSeriesPick2Request struct {
	ChannelId int64 `json:"channel_id"`
}

type GetWidgetSeriesPick2Response struct {
	ChannelId int64 `json:"channel_id"`
	Price     int   `json:"price"`
}

func (s *Service) GetWidgetSeriesPick2(r *GetWidgetSeriesPick2Request) (*GetWidgetSeriesPick2Response, error) {
	key := cache.IdToKey(r.ChannelId)
	if s.WidgetSeriesPick2Cache.Has(key) {
		cached, _ := s.WidgetSeriesPick2Cache.Get(key)
		return &GetWidgetSeriesPick2Response{ChannelId: cached.ChannelId, Price: cached.Price}, nil
	}
	w, err := s.WidgetSeriesPick2Repositorier.GetByChannel(r.ChannelId)
	if err != nil {
		return nil, err
	}
	s.WidgetSeriesPick2Cache.Set(key, w)
	return &GetWidgetSeriesPick2Response{
		ChannelId: w.ChannelId,
		Price:     w.Price,
	}, nil
}
