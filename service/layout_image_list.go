package service

import "github.com/SaloEater/WhatNot-Webhook-Holder/entity"

type LayoutImageListRequest struct {
	ChannelId int64 `json:"channel_id"`
}

type LayoutImageListResponse struct {
	Images []entity.LayoutImage `json:"images"`
}

func (s *Service) LayoutImageList(r *LayoutImageListRequest) (*LayoutImageListResponse, error) {
	images, err := s.LayoutImageRepositorier.ListByChannel(r.ChannelId)
	if err != nil {
		return nil, err
	}
	return &LayoutImageListResponse{Images: images}, nil
}
