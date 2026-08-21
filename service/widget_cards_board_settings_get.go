package service

import "github.com/SaloEater/WhatNot-Webhook-Holder/cache"

type GetWidgetCardsBoardSettingsRequest struct {
	ChannelId int64 `json:"channel_id"`
}

type GetWidgetCardsBoardSettingsResponse struct {
	ChannelId              int64  `json:"channel_id"`
	Orientation            string `json:"orientation"`
	ShowHorizontalRow      bool   `json:"show_horizontal_row"`
	ShowOnlyAvailableTeams bool   `json:"show_only_available_teams"`
}

func (s *Service) GetWidgetCardsBoardSettings(r *GetWidgetCardsBoardSettingsRequest) (*GetWidgetCardsBoardSettingsResponse, error) {
	key := cache.IdToKey(r.ChannelId)
	if s.CardsBoardSettingsCache.Has(key) {
		cached, _ := s.CardsBoardSettingsCache.Get(key)
		return &GetWidgetCardsBoardSettingsResponse{ChannelId: cached.ChannelId, Orientation: cached.Orientation, ShowHorizontalRow: cached.ShowHorizontalRow, ShowOnlyAvailableTeams: cached.ShowOnlyAvailableTeams}, nil
	}
	w, err := s.WidgetCardsBoardSettingsRepositorier.GetByChannel(r.ChannelId)
	if err != nil {
		return nil, err
	}
	s.CardsBoardSettingsCache.Set(key, w)
	return &GetWidgetCardsBoardSettingsResponse{ChannelId: w.ChannelId, Orientation: w.Orientation, ShowHorizontalRow: w.ShowHorizontalRow, ShowOnlyAvailableTeams: w.ShowOnlyAvailableTeams}, nil
}
