package service

import "github.com/SaloEater/WhatNot-Webhook-Holder/cache"

type SetActiveStreamRequest struct {
	ChannelId      int64  `json:"channel_id"`
	ActiveStreamId *int64 `json:"active_stream_id"`
}

type SetActiveStreamResponse struct {
	Success bool `json:"success"`
}

func (s *Service) SetActiveStream(r *SetActiveStreamRequest) (*SetActiveStreamResponse, error) {
	channel, err := s.ChannelRepositorier.Get(r.ChannelId)
	if err != nil {
		return nil, err
	}
	channel.ActiveStreamId = r.ActiveStreamId
	err = s.ChannelRepositorier.SetActiveStream(channel.Id, r.ActiveStreamId)
	if err != nil {
		return &SetActiveStreamResponse{}, err
	}
	s.ChannelCache.Set(cache.IdToKey(r.ChannelId), channel)
	return &SetActiveStreamResponse{Success: true}, nil
}
