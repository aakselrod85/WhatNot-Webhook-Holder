package service

import (
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
)

const maxLayoutPresetsPerChannel = 30

type CreateLayoutPresetRequest struct {
	ChannelId int64           `json:"channel_id"`
	Name      string          `json:"name"`
	Config    json.RawMessage `json:"config"`
}

type CreateLayoutPresetResponse struct {
	Preset *entity.LayoutPreset `json:"preset"`
}

func (s *Service) CreateLayoutPreset(r *CreateLayoutPresetRequest) (*CreateLayoutPresetResponse, error) {
	name := strings.TrimSpace(r.Name)
	if l := utf8.RuneCountInString(name); l < 1 || l > 60 {
		return nil, errors.New("name must be between 1 and 60 characters")
	}
	if err := validateJSONObject(r.Config); err != nil {
		return nil, err
	}

	count, err := s.LayoutPresetRepositorier.CountLayoutPresets(r.ChannelId)
	if err != nil {
		return nil, err
	}
	if count >= maxLayoutPresetsPerChannel {
		return nil, errors.New("this channel already has the maximum of 30 layout presets")
	}

	existing, err := s.LayoutPresetRepositorier.ListLayoutPresets(r.ChannelId)
	if err != nil {
		return nil, err
	}
	for _, p := range existing {
		if p.Name == name {
			return nil, errors.New("a layout preset with this name already exists for this channel")
		}
	}

	preset, err := s.LayoutPresetRepositorier.CreateLayoutPreset(r.ChannelId, name, r.Config)
	if err != nil {
		return nil, err
	}
	return &CreateLayoutPresetResponse{Preset: preset}, nil
}
