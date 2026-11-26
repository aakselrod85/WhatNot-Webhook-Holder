package service

import (
	"fmt"

	"github.com/SaloEater/WhatNot-Webhook-Holder/cache"
	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
)

type SetStreamStatusRequest struct {
	StreamId int64  `json:"stream_id"`
	Status   string `json:"status"`
}

type SetStreamStatusResponse struct {
	Success bool `json:"success"`
}

// SetStreamStatus writes status, keeps is_ended in sync (ended|failed_to_verify|verified => true), refreshes the cache
// and syncs the show's Discord message.
func (s *Service) SetStreamStatus(stream *entity.Stream, status string) error {
	if err := s.setStreamStatusNoSync(stream, status); err != nil {
		return err
	}
	s.syncDiscordShowAsync(stream.Id)
	return nil
}

func (s *Service) setStreamStatusNoSync(stream *entity.Stream, status string) error {
	stream.Status = status
	stream.IsEnded = status == entity.StreamStatusEnded || status == entity.StreamStatusFailedToVerify || status == entity.StreamStatusVerified
	if err := s.StreamRepositorier.SetStatus(stream.Id, status); err != nil {
		return err
	}
	s.StreamCache.Set(cache.IdToKey(stream.Id), stream)
	return nil
}

// SetStreamStatusManual only accepts "verified" and "ended" (the latter is an alias of EventStreamEnded).
func (s *Service) SetStreamStatusManual(r *SetStreamStatusRequest) (*SetStreamStatusResponse, error) {
	switch r.Status {
	case entity.StreamStatusVerified:
		stream, err := s.StreamRepositorier.Get(r.StreamId)
		if err != nil {
			return nil, err
		}
		if err := s.SetStreamStatus(stream, entity.StreamStatusVerified); err != nil {
			return nil, err
		}
	case entity.StreamStatusEnded:
		if err := s.EventStreamEnded(&EventStreamEndedRequest{StreamID: r.StreamId}); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("status %q can not be set manually", r.Status)
	}
	return &SetStreamStatusResponse{Success: true}, nil
}
