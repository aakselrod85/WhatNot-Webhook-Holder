package service

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/SaloEater/WhatNot-Webhook-Holder/cache"
	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
	"github.com/SaloEater/WhatNot-Webhook-Holder/whatnot"
)

func webhookShowId(data json.RawMessage) (string, error) {
	var d struct {
		ShowId string `json:"show_id"`
	}
	if err := json.Unmarshal(data, &d); err != nil {
		return "", err
	}
	if d.ShowId == "" {
		return "", fmt.Errorf("no show_id in event data")
	}
	return d.ShowId, nil
}

func webhookEventTime(ev *entity.WhatnotWebhookEvent) time.Time {
	if ev.EventTime != nil {
		return ev.EventTime.UTC()
	}
	return time.Now().UTC()
}

// whatnotContentFromAPI maps the Whatnot show shape (REST get-a-show and the data of shows.created/updated) to the
// fields a show keeps.
func whatnotContentFromAPI(wn *whatnot.Show) *entity.WhatnotShowContent {
	c := &entity.WhatnotShowContent{
		Title:       wn.Title,
		Description: wn.Description,
		Visibility:  wn.Visibility,
		ScheduledAt: wn.ScheduledStartAt,
		EndedAt:     wn.EndedAt,
	}
	if wn.Thumbnail != nil {
		c.ThumbnailUrl = wn.Thumbnail.Url
	}
	return c
}

// boundStream returns the show of the event's channel bound to the Whatnot show; nil when none is (the event is
// then ignored: the holder never creates a show from Whatnot).
func (s *Service) boundStream(ev *entity.WhatnotWebhookEvent, whatnotShowId string) (*entity.Stream, error) {
	return s.StreamRepositorier.GetByWhatnotShowId(ev.ChannelId, whatnotShowId)
}

// onWhatnotShowUpdated overwrites the content fields of the bound show. shows.created is handled the same way.
func (s *Service) onWhatnotShowUpdated(ev *entity.WhatnotWebhookEvent, data json.RawMessage) error {
	var wn whatnot.Show
	if err := json.Unmarshal(data, &wn); err != nil {
		return err
	}
	if wn.Id == "" {
		return fmt.Errorf("no id in event data")
	}
	stream, err := s.boundStream(ev, wn.Id)
	if err != nil || stream == nil {
		return err
	}
	if err := s.StreamRepositorier.UpdateWhatnotContent(stream.Id, whatnotContentFromAPI(&wn)); err != nil {
		return err
	}
	s.syncDiscordShowAsync(stream.Id)
	return nil
}

func (s *Service) onWhatnotShowStarted(ev *entity.WhatnotWebhookEvent, data json.RawMessage) error {
	showId, err := webhookShowId(data)
	if err != nil {
		return err
	}
	stream, err := s.boundStream(ev, showId)
	if err != nil || stream == nil {
		return err
	}
	startedAt := webhookEventTime(ev)
	if err := s.StreamRepositorier.SetTimes(stream.Id, &startedAt, nil); err != nil {
		return err
	}
	switch stream.Status {
	case entity.StreamStatusLive, entity.StreamStatusEnded, entity.StreamStatusFailedToVerify, entity.StreamStatusVerified:
		s.syncDiscordShowAsync(stream.Id) // the started time changed; SetStreamStatus below syncs otherwise
		return nil
	}
	return s.SetStreamStatus(stream, entity.StreamStatusLive)
}

func (s *Service) onWhatnotShowEnded(ev *entity.WhatnotWebhookEvent, data json.RawMessage) error {
	showId, err := webhookShowId(data)
	if err != nil {
		return err
	}
	stream, err := s.boundStream(ev, showId)
	if err != nil || stream == nil {
		return err
	}
	endedAt := webhookEventTime(ev)
	if err := s.StreamRepositorier.SetTimes(stream.Id, nil, &endedAt); err != nil {
		return err
	}
	s.syncDiscordShowAsync(stream.Id) // EventStreamEnded does not move an already ended stream; the ended time changed
	if err := s.EventStreamEnded(&EventStreamEndedRequest{StreamID: stream.Id}); err != nil {
		fmt.Println(fmt.Sprintf("whatnot webhook: stream %d: event stream ended: %s", stream.Id, err.Error()))
	}
	_, err = s.RunShowReconciliation(stream.Id, "webhook", false)
	return err
}

// onWhatnotShowCancelled marks a show that has not started as cancelled. A started or finished show keeps its status.
func (s *Service) onWhatnotShowCancelled(ev *entity.WhatnotWebhookEvent, data json.RawMessage) error {
	showId, err := webhookShowId(data)
	if err != nil {
		return err
	}
	stream, err := s.boundStream(ev, showId)
	if err != nil || stream == nil {
		return err
	}
	if stream.Status != entity.StreamStatusPlanned {
		return nil
	}
	if err := s.SetStreamStatus(stream, entity.StreamStatusCancelled); err != nil {
		return err
	}
	s.StreamCache.Delete(cache.IdToKey(stream.Id))
	return nil
}
