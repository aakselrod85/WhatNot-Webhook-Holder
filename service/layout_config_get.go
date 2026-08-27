package service

import "encoding/json"

type GetLayoutConfigRequest struct {
	ChannelId int64 `json:"channel_id"`
}

type GetLayoutConfigResponse struct {
	Config json.RawMessage `json:"config"`
}

func (s *Service) GetLayoutConfig(r *GetLayoutConfigRequest) (*GetLayoutConfigResponse, error) {
	c, err := s.LayoutConfigRepositorier.GetLayoutConfig(r.ChannelId)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return &GetLayoutConfigResponse{Config: nil}, nil
	}
	return &GetLayoutConfigResponse{Config: c.Config}, nil
}
