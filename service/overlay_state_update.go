package service

import "encoding/json"

type UpdateOverlayStateRequest struct {
	ChannelId int64           `json:"channel_id"`
	State     json.RawMessage `json:"state"`
}

type UpdateOverlayStateResponse struct {
	Seq   int64           `json:"seq"`
	State json.RawMessage `json:"state"`
}

func (s *Service) UpdateOverlayState(r *UpdateOverlayStateRequest) (*UpdateOverlayStateResponse, error) {
	if err := validateJSONObject(r.State); err != nil {
		return nil, err
	}
	st, err := s.OverlayStateRepositorier.UpsertOverlayState(r.ChannelId, r.State)
	if err != nil {
		return nil, err
	}
	return &UpdateOverlayStateResponse{Seq: st.Seq, State: st.State}, nil
}
