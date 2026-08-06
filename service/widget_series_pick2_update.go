package service

import (
	"github.com/SaloEater/WhatNot-Webhook-Holder/cache"
	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
)

type UpdateWidgetSeriesPick2Request struct {
	ChannelId int64 `json:"channel_id"`
	Price     int   `json:"price"`
}

type UpdateWidgetSeriesPick2Response struct {
	Success bool `json:"success"`
}

func (s *Service) UpdateWidgetSeriesPick2(r *UpdateWidgetSeriesPick2Request) (*UpdateWidgetSeriesPick2Response, error) {
	response := &UpdateWidgetSeriesPick2Response{}
	err := s.WidgetSeriesPick2Repositorier.Upsert(&entity.WidgetSeriesPick2{
		ChannelId: r.ChannelId,
		Price:     r.Price,
	})
	if err == nil {
		response.Success = true
		s.WidgetSeriesPick2Cache.Delete(cache.IdToKey(r.ChannelId))
	}
	return response, err
}
