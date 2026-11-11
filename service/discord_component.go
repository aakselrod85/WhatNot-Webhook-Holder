package service

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/SaloEater/WhatNot-Webhook-Holder/discord"
	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
)

// discordLoadingMessages holds the ids of the Discord messages whose page is being loaded after a click.
// A click on a message in this set is ignored (in-process; the backend runs as a single instance).
var discordLoadingMessages sync.Map

const (
	// discordAckWait bounds how long the background work waits for the click response to be written.
	discordAckWait = 3 * time.Second
	// discordAckSettle lets Discord apply the "disabled controls" response before the final edit lands.
	discordAckSettle = 300 * time.Millisecond
)

// HandleDiscordComponent handles a click on a show card control: the view buttons, the ◀ ▶ arrows and the Stats
// page select menu, on a feed message ("show:" ids) or on a /stream-status reply ("stream:" ids).
//
// accepted=false means the click is ignored: unknown control, or the same message is still loading a page.
// For an accepted click, loading is the message's controls all disabled (nil when they could not be read); the
// caller answers the click with it and closes acked once the answer is written. The page itself is loaded in the
// background; if that fails, the message's original controls are put back.
func (s *Service) HandleDiscordComponent(in discord.Interaction, acked <-chan struct{}) (loading []discord.ActionRow, accepted bool) {
	if !s.discordEnabled() {
		return nil, false
	}
	prefix := "show"
	if strings.HasPrefix(in.Data.CustomId, "stream:") {
		prefix = "stream"
	}
	view, showId, ok := parseViewButton(in.Data.CustomId, in.Data.Values, prefix)
	if !ok {
		return nil, false
	}
	key := in.Message.Id
	if key == "" {
		key = in.Data.CustomId
	}
	if _, busy := discordLoadingMessages.LoadOrStore(key, struct{}{}); busy {
		return nil, false
	}
	loading = disabledComponents(in.Message.Components)

	go func() {
		defer discordLoadingMessages.Delete(key)
		defer func() {
			if r := recover(); r != nil {
				discordLogf("component %q: panic: %v", in.Data.CustomId, r)
			}
		}()
		started := time.Now()
		select {
		case <-acked:
		case <-time.After(discordAckWait):
		}
		time.Sleep(discordAckSettle)

		// The click answer already disabled the controls; disabling them again through the API covers the case
		// where Discord did not apply that answer, and logs Discord's reason if it refuses.
		if loading != nil {
			if err := s.Discord.EditInteractionComponents(in.ApplicationId, in.Token, loading); err != nil {
				discordLogf("component %q: disable controls: %s", in.Data.CustomId, err.Error())
			}
		}
		discordLogf("component %q: controls disabled after %s", in.Data.CustomId, time.Since(started).Round(time.Millisecond))

		var err error
		if prefix == "stream" {
			err = s.loadStreamReplyPage(in, showId, view)
		} else {
			err = s.loadFeedPage(showId, view)
		}
		discordLogf("component %q: page shown after %s (error: %v)", in.Data.CustomId, time.Since(started).Round(time.Millisecond), err)
		if err != nil {
			discordLogf("component %q: %s", in.Data.CustomId, err.Error())
			if loading != nil && len(in.Message.Components) > 0 {
				if err := s.Discord.EditInteractionComponents(in.ApplicationId, in.Token, in.Message.Components); err != nil {
					discordLogf("component %q: restore controls: %s", in.Data.CustomId, err.Error())
				}
			}
		}
	}()
	return loading, true
}

// loadFeedPage stores the chosen view of the show's feed message and re-renders that message.
func (s *Service) loadFeedPage(showId int64, view string) error {
	show, err := s.WhatnotShowRepositorier.Get(showId)
	if err != nil {
		return err
	}
	if show == nil || show.Status == entity.WhatnotShowStatusCancelled {
		return errShowUnavailable
	}
	if err := s.DiscordShowMessageRepositorier.SetView(showId, view); err != nil {
		return err
	}
	return s.syncDiscordShow(showId, false)
}

// loadStreamReplyPage re-renders a /stream-status reply in the chosen view; the feed message is left alone.
func (s *Service) loadStreamReplyPage(in discord.Interaction, showId int64, view string) error {
	show, err := s.WhatnotShowRepositorier.Get(showId)
	if err != nil {
		return err
	}
	if show == nil {
		return errShowUnavailable
	}
	msg, err := s.streamShowCard(show, view)
	if err != nil {
		return err
	}
	return s.Discord.EditInteractionResponse(in.ApplicationId, in.Token, msg)
}

type discordError string

func (e discordError) Error() string { return string(e) }

const errShowUnavailable = discordError("show not found or cancelled")

// disabledComponents returns the message's controls with every button (except links) and select menu disabled,
// and the page label showing "Loading…". nil when the components can not be read.
func disabledComponents(raw json.RawMessage) []discord.ActionRow {
	if len(raw) == 0 {
		return nil
	}
	var rows []discord.ActionRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil
	}
	for i := range rows {
		for j := range rows[i].Components {
			c := &rows[i].Components[j]
			if c.Type == discord.ComponentTypeButton && c.Style == discord.ButtonStyleLink {
				continue
			}
			c.Disabled = true
			if strings.Contains(c.CustomId, ":noop:") {
				c.Label = "⏳ Loading…"
			}
		}
	}
	return rows
}
