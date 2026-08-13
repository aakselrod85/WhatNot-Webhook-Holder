package service

import "github.com/SaloEater/WhatNot-Webhook-Holder/cache"

type PhotoRestoreRequest struct {
	Id int64 `json:"id"`
}

type PhotoRestoreResponse struct {
	Success bool `json:"success"`
}

func (s *Service) PhotoRestore(r *PhotoRestoreRequest) (*PhotoRestoreResponse, error) {
	response := &PhotoRestoreResponse{Success: false}

	photo, err := s.PhotoRepositorier.GetById(r.Id)
	if err != nil {
		return response, err
	}

	err = s.PhotoRepositorier.Restore(r.Id)
	if err == nil {
		response.Success = true
		s.SeriesPricesCache.Delete(cache.IdToKey(photo.SeriesId))
		s.SeriesWithCountCache.Delete(cache.IdToKey(photo.SeriesId))
	}
	return response, err
}
