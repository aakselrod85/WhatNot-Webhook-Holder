package service

import "github.com/SaloEater/WhatNot-Webhook-Holder/cache"

type SetActiveBreakRequest struct {
	StreamId      int64  `json:"stream_id"`
	ActiveBreakId *int64 `json:"active_break_id"`
}

type SetActiveBreakResponse struct {
	Success bool `json:"success"`
}

func (s *Service) SetActiveBreak(r *SetActiveBreakRequest) (*SetActiveBreakResponse, error) {
	stream, err := s.StreamRepositorier.Get(r.StreamId)
	if err != nil {
		return nil, err
	}
	stream.ActiveBreakId = r.ActiveBreakId
	err = s.StreamRepositorier.SetActiveBreak(stream.Id, r.ActiveBreakId)
	if err != nil {
		return &SetActiveBreakResponse{}, err
	}
	s.StreamCache.Set(cache.IdToKey(r.StreamId), stream)

	channel, err := s.ChannelRepositorier.Get(stream.ChannelId)
	if err != nil {
		return nil, err
	}
	channel.ActiveStreamId = &r.StreamId
	err = s.ChannelRepositorier.SetActiveStream(channel.Id, &r.StreamId)
	if err != nil {
		return &SetActiveBreakResponse{}, err
	}
	s.ChannelCache.Set(cache.IdToKey(stream.ChannelId), channel)

	return &SetActiveBreakResponse{Success: true}, nil
}
