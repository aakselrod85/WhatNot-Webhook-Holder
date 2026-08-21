package service

import (
	"github.com/SaloEater/WhatNot-Webhook-Holder/cache"
	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
)

type UpdateWidgetCardsBoardSettingsRequest struct {
	ChannelId              int64  `json:"channel_id"`
	Orientation            string `json:"orientation"`
	ShowHorizontalRow      bool   `json:"show_horizontal_row"`
	ShowOnlyAvailableTeams bool   `json:"show_only_available_teams"`
}

type UpdateWidgetCardsBoardSettingsResponse struct {
	Success bool `json:"success"`
}

func (s *Service) UpdateWidgetCardsBoardSettings(r *UpdateWidgetCardsBoardSettingsRequest) (*UpdateWidgetCardsBoardSettingsResponse, error) {
	response := &UpdateWidgetCardsBoardSettingsResponse{}
	err := s.WidgetCardsBoardSettingsRepositorier.Upsert(&entity.WidgetCardsBoardSettings{
		ChannelId:              r.ChannelId,
		Orientation:            r.Orientation,
		ShowHorizontalRow:      r.ShowHorizontalRow,
		ShowOnlyAvailableTeams: r.ShowOnlyAvailableTeams,
	})
	if err == nil {
		response.Success = true
		s.CardsBoardSettingsCache.Delete(cache.IdToKey(r.ChannelId))
	}
	return response, err
}
