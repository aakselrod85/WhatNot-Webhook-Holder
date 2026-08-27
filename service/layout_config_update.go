package service

import "encoding/json"

type UpdateLayoutConfigRequest struct {
	ChannelId int64           `json:"channel_id"`
	Config    json.RawMessage `json:"config"`
}

type UpdateLayoutConfigResponse struct {
	Success bool `json:"success"`
}

func (s *Service) UpdateLayoutConfig(r *UpdateLayoutConfigRequest) (*UpdateLayoutConfigResponse, error) {
	if err := validateJSONObject(r.Config); err != nil {
		return nil, err
	}
	response := &UpdateLayoutConfigResponse{}
	err := s.LayoutConfigRepositorier.UpsertLayoutConfig(r.ChannelId, r.Config)
	if err == nil {
		response.Success = true
	}
	return response, err
}
