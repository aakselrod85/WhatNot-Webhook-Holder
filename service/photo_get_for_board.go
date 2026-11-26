package service

import "github.com/SaloEater/WhatNot-Webhook-Holder/entity"

type PhotoGetForBoardRequest struct {
	ChannelId int64 `json:"channel_id"`
	WithSold  bool  `json:"with_sold"`
}

// PhotoGetForBoard runs in two steps because the break lives in mobwebapp's database and the photos in the holder's:
// the series of the channel's active break first, then the photos of that series.
func (s *Service) PhotoGetForBoard(r *PhotoGetForBoardRequest) ([]*entity.Photo, error) {
	seriesId, err := s.BreakRepositorier.GetActiveSeriesIdByChannel(r.ChannelId)
	if err != nil {
		return nil, err
	}
	if seriesId == nil {
		return []*entity.Photo{}, nil
	}
	return s.PhotoRepositorier.GetUnsoldBySeriesId(*seriesId, r.WithSold)
}
