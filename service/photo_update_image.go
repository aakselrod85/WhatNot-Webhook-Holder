package service

import "fmt"

type PhotoUpdateImageResponse struct {
	Url string `json:"url"`
}

func (s *Service) PhotoUpdateImage(id int64, imageData []byte, filename string) (*PhotoUpdateImageResponse, error) {
	photo, err := s.PhotoRepositorier.GetById(id)
	if err != nil {
		return nil, err
	}
	if photo == nil {
		return nil, fmt.Errorf("photo %d not found", id)
	}

	url, err := s.DigitalOceaner.SaveCardPhoto(imageData, photo.SeriesId, filename)
	if err != nil {
		return nil, err
	}

	err = s.PhotoRepositorier.UpdateUrl(id, url)
	if err != nil {
		return nil, err
	}

	return &PhotoUpdateImageResponse{Url: url}, nil
}
