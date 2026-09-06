package service

type LayoutImageDeleteRequest struct {
	Id int64 `json:"id"`
}

type LayoutImageDeleteResponse struct {
	Success bool `json:"success"`
}

func (s *Service) LayoutImageDelete(r *LayoutImageDeleteRequest) (*LayoutImageDeleteResponse, error) {
	response := &LayoutImageDeleteResponse{Success: false}

	err := s.LayoutImageRepositorier.Delete(r.Id)
	if err == nil {
		response.Success = true
	}
	return response, err
}
