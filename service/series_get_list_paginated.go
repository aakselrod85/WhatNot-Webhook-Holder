package service

import "github.com/SaloEater/WhatNot-Webhook-Holder/entity"

type SeriesGetListPaginatedRequest struct {
	Page     int64 `json:"page"`
	PageSize int64 `json:"page_size"`
}

type SeriesGetListPaginatedResponse struct {
	Items []*entity.Series `json:"items"`
	Total int64            `json:"total"`
}

func (s *Service) SeriesGetListPaginated(request *SeriesGetListPaginatedRequest) (*SeriesGetListPaginatedResponse, error) {
	pageSize := request.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	page := request.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	items, err := s.SeriesRepositorier.GetListPaginated(pageSize, offset)
	if err != nil {
		return nil, err
	}
	total, err := s.SeriesRepositorier.CountActive()
	if err != nil {
		return nil, err
	}
	return &SeriesGetListPaginatedResponse{
		Items: items,
		Total: total,
	}, nil
}
