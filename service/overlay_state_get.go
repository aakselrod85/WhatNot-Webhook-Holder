package service

import "encoding/json"

type GetOverlayStateRequest struct {
	ChannelId int64 `json:"channel_id"`
}

type GetOverlayStateResponse struct {
	Seq   int64           `json:"seq"`
	State json.RawMessage `json:"state"`
}

func (s *Service) GetOverlayState(r *GetOverlayStateRequest) (*GetOverlayStateResponse, error) {
	st, err := s.OverlayStateRepositorier.GetOverlayState(r.ChannelId)
	if err != nil {
		return nil, err
	}
	if st == nil {
		return &GetOverlayStateResponse{Seq: 0, State: nil}, nil
	}
	return &GetOverlayStateResponse{Seq: st.Seq, State: st.State}, nil
}
