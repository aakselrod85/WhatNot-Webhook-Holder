package service

import (
	"errors"
	"fmt"
	"github.com/SaloEater/WhatNot-Webhook-Holder/cache"
	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
)

type GetStreamRequest struct {
	Id int64 `json:"id"`
}

func (s *Service) GetStream(r *GetStreamRequest) (*entity.Stream, error) {
	key := cache.IdToKey(r.Id)

	if !s.StreamCache.Has(key) {
		stream, err := s.StreamRepositorier.Get(r.Id)
		if stream == nil {
			return nil, err
		}
		s.StreamCache.Set(key, stream)
	}

	cached, found := s.StreamCache.Get(key)
	if !found {
		return nil, errors.New(fmt.Sprintf("stream for id %d not found", r.Id))
	}

	stream := *cached
	return &stream, nil
}
