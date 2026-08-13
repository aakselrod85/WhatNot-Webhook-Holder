package service

import "github.com/SaloEater/WhatNot-Webhook-Holder/cache"

type PhotoDeleteRequest struct {
	Id int64 `json:"id"`
}

type PhotoDeleteResponse struct {
	Success bool `json:"success"`
}

func (s *Service) PhotoDelete(r *PhotoDeleteRequest) (*PhotoDeleteResponse, error) {
	response := &PhotoDeleteResponse{Success: false}

	photo, err := s.PhotoRepositorier.GetById(r.Id)
	if err != nil {
		return response, err
	}

	err = s.PhotoRepositorier.Delete(r.Id)
	if err == nil {
		response.Success = true
		s.SeriesPricesCache.Delete(cache.IdToKey(photo.SeriesId))
		s.SeriesWithCountCache.Delete(cache.IdToKey(photo.SeriesId))
	}
	return response, err
}
