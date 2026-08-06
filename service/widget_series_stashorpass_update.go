package service

import (
	"github.com/SaloEater/WhatNot-Webhook-Holder/cache"
	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
)

type UpdateWidgetSeriesStashorpassRequest struct {
	ChannelId int64 `json:"channel_id"`
	Price     int   `json:"price"`
}

type UpdateWidgetSeriesStashorpassResponse struct {
	Success bool `json:"success"`
}

func (s *Service) UpdateWidgetSeriesStashorpass(r *UpdateWidgetSeriesStashorpassRequest) (*UpdateWidgetSeriesStashorpassResponse, error) {
	response := &UpdateWidgetSeriesStashorpassResponse{}
	err := s.WidgetSeriesStashorpassRepositorier.Upsert(&entity.WidgetSeriesStashorpass{
		ChannelId: r.ChannelId,
		Price:     r.Price,
	})
	if err == nil {
		response.Success = true
		s.WidgetSeriesStashorpassCache.Delete(cache.IdToKey(r.ChannelId))
	}
	return response, err
}
