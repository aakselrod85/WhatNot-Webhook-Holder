package service

import (
	"fmt"
)

type PhotoThumbnailResponse struct {
	Thumbnail string `json:"thumbnail"`
}

func (s *Service) PhotoThumbnail(id int64, thumbData []byte, filename string) (*PhotoThumbnailResponse, error) {
	photo, err := s.PhotoRepositorier.GetById(id)
	if err != nil {
		return nil, err
	}
	if photo == nil {
		return nil, fmt.Errorf("photo %d not found", id)
	}

	url, err := s.DigitalOceaner.SaveCardThumbnail(thumbData, photo.SeriesId, filename)
	if err != nil {
		return nil, err
	}

	err = s.PhotoRepositorier.UpdateThumbnail(id, url)
	if err != nil {
		return nil, err
	}

	return &PhotoThumbnailResponse{Thumbnail: url}, nil
}
