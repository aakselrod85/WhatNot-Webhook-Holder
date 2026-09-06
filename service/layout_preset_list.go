package service

import "github.com/SaloEater/WhatNot-Webhook-Holder/entity"

type ListLayoutPresetsRequest struct {
	ChannelId int64 `json:"channel_id"`
}

type ListLayoutPresetsResponse struct {
	Presets []entity.LayoutPreset `json:"presets"`
}

func (s *Service) ListLayoutPresets(r *ListLayoutPresetsRequest) (*ListLayoutPresetsResponse, error) {
	presets, err := s.LayoutPresetRepositorier.ListLayoutPresets(r.ChannelId)
	if err != nil {
		return nil, err
	}
	return &ListLayoutPresetsResponse{Presets: presets}, nil
}
