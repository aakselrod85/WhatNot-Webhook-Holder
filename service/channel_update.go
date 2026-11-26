package service

import "github.com/SaloEater/WhatNot-Webhook-Holder/cache"

type UpdateChannelRequest struct {
	Id                  int64  `json:"id"`
	Name                string `json:"name"`
	DefaultHighBidTeam  string `json:"default_high_bid_team"`
	DefaultHighBidFloor int    `json:"default_high_bid_floor"`
}

type UpdateChannelResponse struct {
	Success bool `json:"success"`
}

func (s *Service) UpdateChannel(r *UpdateChannelRequest) (*UpdateChannelResponse, error) {
	response := &UpdateChannelResponse{}
	channel, err := s.ChannelRepositorier.Get(r.Id)
	if err != nil {
		return response, err
	}

	channel.Name = r.Name
	channel.DefaultHighBidTeam = r.DefaultHighBidTeam
	channel.DefaultHighBidFloor = r.DefaultHighBidFloor
	err = s.ChannelRepositorier.UpdateSettings(channel.Id, channel.Name, channel.DefaultHighBidTeam, channel.DefaultHighBidFloor)
	if err == nil {
		response.Success = true
	}
	key := cache.IdToKey(r.Id)
	s.ChannelCache.Set(key, channel)

	return response, err
}
