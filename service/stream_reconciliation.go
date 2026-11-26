package service

import (
	"encoding/json"
	"time"
)

type StreamReconciliationRequest struct {
	StreamId int64 `json:"stream_id"`
}

type ReconciliationSummary struct {
	Fixed       int `json:"fixed"`        // customer mismatches fixed (or to fix)
	PriceFixed  int `json:"price_fixed"`  // price-only mismatches fixed (or to fix)
	Added       int `json:"added"`        // giveaway events added (or to add)
	EventsAdded int `json:"events_added"` // new events added for orders with no free event (or to add)
	Blocking    int `json:"blocking"`     // blocking issues (needs_manual)
}

type ReconciliationItem struct {
	Id         int64                 `json:"id"`
	Trigger    string                `json:"trigger"`
	Status     string                `json:"status"`
	StartedAt  time.Time             `json:"started_at"`
	FinishedAt *time.Time            `json:"finished_at"`
	Error      string                `json:"error"`
	Summary    ReconciliationSummary `json:"summary"`
	ManualMap  json.RawMessage       `json:"manual_map"`
}

type StreamReconciliationResponse struct {
	Item *ReconciliationItem `json:"item"`
}

// StreamReconciliation returns the reconciliation of the stream's bound Whatnot show without the
// orders blob; item is null when no show is linked or none ran yet for that show.
func (s *Service) StreamReconciliation(r *StreamReconciliationRequest) (*StreamReconciliationResponse, error) {
	stream, err := s.StreamRepositorier.Get(r.StreamId)
	if err != nil {
		return nil, err
	}
	if stream.WhatnotShowId == nil {
		return &StreamReconciliationResponse{}, nil
	}
	row, err := s.GetReconciliationByStream(r.StreamId, *stream.WhatnotShowId)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return &StreamReconciliationResponse{}, nil
	}
	item := &ReconciliationItem{Id: row.Id, Trigger: row.Trigger, Status: row.Status, StartedAt: row.StartedAt, FinishedAt: row.FinishedAt, Error: row.Error, ManualMap: row.ManualMap}
	var rep ReconReport
	if json.Unmarshal(row.Report, &rep) == nil {
		for _, b := range rep.Breaks {
			item.Summary.Fixed += b.ToFix
			item.Summary.PriceFixed += b.ToFixPrice
			item.Summary.EventsAdded += len(b.ToAdd)
		}
		item.Summary.Added = rep.Giveaways.ToAdd
		item.Summary.Blocking = rep.Blocking
	}
	return &StreamReconciliationResponse{Item: item}, nil
}

// ReconManualMap holds the human decisions from the manual page. Re-runs honour them over auto-matching.
type ReconManualMap struct {
	Breaks    map[string]int64 `json:"breaks"`    // Whatnot break name -> break id, or reconciliation.SkipBreak
	Giveaways map[string]int64 `json:"giveaways"` // order id -> break id
}

type ReconciliationSaveManualMapRequest struct {
	StreamId int64 `json:"stream_id"`
	ReconManualMap
}

type ReconciliationSaveManualMapResponse struct {
	Success bool `json:"success"`
}

// ReconciliationSaveManualMap stores the mapping on the row of the stream's linked show; a no-op when none ran yet.
func (s *Service) ReconciliationSaveManualMap(r *ReconciliationSaveManualMapRequest) (*ReconciliationSaveManualMapResponse, error) {
	encoded, err := json.Marshal(r.ReconManualMap)
	if err != nil {
		return nil, err
	}
	stream, err := s.StreamRepositorier.Get(r.StreamId)
	if err != nil {
		return nil, err
	}
	if stream.WhatnotShowId == nil {
		return &ReconciliationSaveManualMapResponse{Success: true}, nil
	}
	if err := s.SetReconciliationManualMap(r.StreamId, *stream.WhatnotShowId, encoded); err != nil {
		return nil, err
	}
	return &ReconciliationSaveManualMapResponse{Success: true}, nil
}
