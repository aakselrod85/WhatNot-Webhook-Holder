package service

import (
	"encoding/json"
	"fmt"

	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
)

// whatnotWebhookHandlers is the topic dispatcher: adding a topic = one entry here + one handler.
var whatnotWebhookHandlers = map[string]func(s *Service, ev *entity.WhatnotWebhookEvent, data json.RawMessage) error{
	"shows.created":   (*Service).onWhatnotShowUpdated,
	"shows.updated":   (*Service).onWhatnotShowUpdated,
	"shows.started":   (*Service).onWhatnotShowStarted,
	"shows.ended":     (*Service).onWhatnotShowEnded,
	"shows.cancelled": (*Service).onWhatnotShowCancelled,
}

// ProcessWhatnotWebhook runs the handler of a stored event and records the outcome on the row.
func (s *Service) ProcessWhatnotWebhook(eventId int64) {
	status, errMsg := entity.WebhookEventStatusFailed, ""
	defer func() {
		if r := recover(); r != nil {
			status, errMsg = entity.WebhookEventStatusFailed, fmt.Sprintf("panic: %v", r)
		}
		if err := s.SetWebhookEventStatus(eventId, status, errMsg); err != nil {
			fmt.Println(fmt.Sprintf("whatnot webhook event %d: set status: %s", eventId, err.Error()))
		}
		fmt.Println(fmt.Sprintf("whatnot webhook event %d: %s %s", eventId, status, errMsg))
	}()

	ev, err := s.GetWebhookEvent(eventId)
	if err != nil {
		errMsg = err.Error()
		return
	}
	handler, ok := whatnotWebhookHandlers[ev.Type]
	if !ok {
		status, errMsg = entity.WebhookEventStatusIgnored, "no handler"
		return
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(ev.Payload, &envelope); err != nil {
		errMsg = err.Error()
		return
	}
	if err := handler(s, ev, envelope.Data); err != nil {
		errMsg = err.Error()
		return
	}
	status = entity.WebhookEventStatusProcessed
}
