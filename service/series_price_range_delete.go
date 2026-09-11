package service

type SeriesPriceRangeDeleteRequest struct {
	Id int64 `json:"id"`
}

type SeriesPriceRangeDeleteResponse struct {
	Success bool `json:"success"`
}

func (s *Service) SeriesPriceRangeDelete(r *SeriesPriceRangeDeleteRequest) (*SeriesPriceRangeDeleteResponse, error) {
	err := s.SeriesPriceRangeRepositorier.Delete(r.Id)
	if err != nil {
		return &SeriesPriceRangeDeleteResponse{}, err
	}
	return &SeriesPriceRangeDeleteResponse{Success: true}, nil
}
