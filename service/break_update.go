package service

import (
	"fmt"
	"github.com/SaloEater/WhatNot-Webhook-Holder/cache"
	"time"
)

type UpdateBreakRequest struct {
	Id           int64  `json:"id"`
	Name         string `json:"name"`
	StartDate    string `json:"start_date"`
	EndDate      string `json:"end_date"`
	HighBidTeam  string `json:"high_bid_team"`
	GiveawayTeam string `json:"giveaway_team"`
	HighBidFloor int    `json:"high_bid_floor"`
}

type UpdateBreakResponse struct {
	Success bool `json:"success"`
}

const DateLayout = "2006-01-02T15:04:05Z"

func (s *Service) UpdateBreak(r *UpdateBreakRequest) (*UpdateBreakResponse, error) {
	startDate, err := time.Parse(DateLayout, r.StartDate)
	if err != nil {
		fmt.Println("Error parsing start date string:", err)
		return nil, err
	}

	endDate, err := time.Parse(DateLayout, r.EndDate)
	if err != nil {
		fmt.Println("Error parsing end date string:", err)
		return nil, err
	}

	response := &UpdateBreakResponse{}
	oldBreak, err := s.BreakRepositorier.Get(r.Id)
	if err != nil {
		return response, err
	}

	renamed := oldBreak.Name != r.Name
	oldBreak.Name = r.Name
	oldBreak.StartDate = startDate
	oldBreak.EndDate = endDate
	oldBreak.HighBidTeam = r.HighBidTeam
	oldBreak.GiveawayTeam = r.GiveawayTeam
	oldBreak.HighBidFloor = r.HighBidFloor
	err = s.BreakRepositorier.Update(oldBreak)
	if err == nil && renamed {
		err = s.BreakRepositorier.Rename(oldBreak.Id, r.Name)
	}
	if err == nil {
		response.Success = true
	}

	s.BreakCache.Set(cache.IdToKey(r.Id), oldBreak)

	return response, err

}
