package service

import (
	"fmt"

	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
)

type PhotoUploadResponse struct {
	Id int64 `json:"id"`
}

func (s *Service) PhotoUpload(seriesID int64, data []byte, name, team string, price int64, rotation int64, filename string, thumbData []byte, thumbFilename string) (*PhotoUploadResponse, error) {
	if rotation != 0 && rotation != 90 && rotation != 180 && rotation != 270 {
		return nil, fmt.Errorf("invalid rotation: %d", rotation)
	}

	url, err := s.DigitalOceaner.SaveCardPhoto(data, seriesID, filename)
	if err != nil {
		return nil, err
	}

	thumbnail := ""
	if len(thumbData) > 0 {
		thumbnail, err = s.DigitalOceaner.SaveCardThumbnail(thumbData, seriesID, thumbFilename)
		if err != nil {
			return nil, err
		}
	}

	id, err := s.PhotoRepositorier.Create(&entity.Photo{
		SeriesId:  seriesID,
		Name:      name,
		Team:      team,
		Price:     price,
		Rotation:  rotation,
		Url:       url,
		Thumbnail: thumbnail,
	})
	if err != nil {
		//TODO: remove photo from DO
		return nil, err
	}
	return &PhotoUploadResponse{Id: id}, nil
}
