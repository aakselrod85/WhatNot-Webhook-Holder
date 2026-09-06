package service

import (
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"
)

type UpdateLayoutPresetRequest struct {
	Id     int64           `json:"id"`
	Name   *string         `json:"name"`
	Config json.RawMessage `json:"config"`
}

type UpdateLayoutPresetResponse struct {
	Success bool `json:"success"`
}

func (s *Service) UpdateLayoutPreset(r *UpdateLayoutPresetRequest) (*UpdateLayoutPresetResponse, error) {
	if r.Name == nil && len(r.Config) == 0 {
		return nil, errors.New("nothing to update")
	}

	var name *string
	if r.Name != nil {
		trimmed := strings.TrimSpace(*r.Name)
		if l := utf8.RuneCountInString(trimmed); l < 1 || l > 60 {
			return nil, errors.New("name must be between 1 and 60 characters")
		}
		name = &trimmed
	}

	if len(r.Config) > 0 {
		if err := validateJSONObject(r.Config); err != nil {
			return nil, err
		}
	}

	// A rename gets the same duplicate guard as create: names are unique per channel, and without
	// this the raw "duplicate key value violates unique constraint" text is what the operator sees.
	if name != nil {
		current, err := s.LayoutPresetRepositorier.GetLayoutPreset(r.Id)
		if err != nil {
			return nil, err
		}
		if current == nil {
			return nil, errors.New("layout preset not found")
		}
		existing, err := s.LayoutPresetRepositorier.ListLayoutPresets(current.ChannelId)
		if err != nil {
			return nil, err
		}
		for _, p := range existing {
			if p.Id != r.Id && p.Name == *name {
				return nil, errors.New("a layout preset with this name already exists for this channel")
			}
		}
	}

	err := s.LayoutPresetRepositorier.UpdateLayoutPreset(r.Id, name, r.Config)
	if err != nil {
		return nil, err
	}
	return &UpdateLayoutPresetResponse{Success: true}, nil
}
