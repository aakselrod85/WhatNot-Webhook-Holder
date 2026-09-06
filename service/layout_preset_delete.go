package service

type DeleteLayoutPresetRequest struct {
	Id int64 `json:"id"`
}

type DeleteLayoutPresetResponse struct {
	Success bool `json:"success"`
}

func (s *Service) DeleteLayoutPreset(r *DeleteLayoutPresetRequest) (*DeleteLayoutPresetResponse, error) {
	err := s.LayoutPresetRepositorier.DeleteLayoutPreset(r.Id)
	if err != nil {
		return nil, err
	}
	return &DeleteLayoutPresetResponse{Success: true}, nil
}
