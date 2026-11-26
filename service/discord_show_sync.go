package service

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/SaloEater/WhatNot-Webhook-Holder/discord"
	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
)

// discordShowLocks holds one mutex per show id (in-process; the backend runs as a single instance).
var discordShowLocks sync.Map

func (s *Service) discordEnabled() bool {
	return s.Discord.Enabled()
}

func discordLogf(format string, args ...any) {
	fmt.Println("discord: " + fmt.Sprintf(format, args...))
}

// SyncDiscordShow rebuilds the show's Discord message from database state and creates, edits or deletes it.
// It never returns an error: failures are logged so they can not affect the caller's request.
func (s *Service) SyncDiscordShow(showId int64) {
	s.syncDiscordShow(showId, false)
}

// syncDiscordShow is SyncDiscordShow; with alert it also sends the failure ping after the message sync, under the same lock.
// The returned error (already logged) is the message sync's.
func (s *Service) syncDiscordShow(showId int64, alert bool) (err error) {
	defer func() {
		if r := recover(); r != nil {
			discordLogf("sync show %d: panic: %v", showId, r)
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	if !s.discordEnabled() {
		return nil
	}
	lock, _ := discordShowLocks.LoadOrStore(showId, &sync.Mutex{})
	mu := lock.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	ref, err := s.syncDiscordShowLocked(showId)
	if err != nil {
		discordLogf("sync show %d: %s", showId, err.Error())
	}
	if alert && ref != nil {
		// A failed ping is only logged.
		if err := s.sendDiscordAlert(ref); err != nil {
			discordLogf("alert show %d: %s", showId, err.Error())
		}
	}
	return err
}

// discordMessageRef is where the show's message lives after a sync; the alert replies to it.
type discordMessageRef struct {
	ChannelId string
	MessageId string
	Input     discordShowInput
}

// discordAlertUserIds resolves who is pinged for a show. For now: the single configured user.
func (s *Service) discordAlertUserIds(show *entity.WhatnotShow) []string {
	if s.DiscordAlertUserId == "" {
		return nil
	}
	return []string{s.DiscordAlertUserId}
}

func (s *Service) sendDiscordAlert(ref *discordMessageRef) error {
	users := s.discordAlertUserIds(ref.Input.Show)
	if len(users) == 0 || ref.ChannelId == "" || ref.MessageId == "" {
		return nil
	}
	title := ref.Input.Show.Title
	if title == "" {
		title = "Whatnot show"
	}
	var mentions strings.Builder
	for _, u := range users {
		mentions.WriteString("<@" + u + "> ")
	}
	suffix := ""
	if s.FrontendUrl != "" && ref.Input.Stream != nil {
		suffix = fmt.Sprintf("\n<%s/stream/%d/reconciliation>", s.FrontendUrl, ref.Input.Stream.Id)
	}
	content := mentions.String() + "⚠️ Reconciliation of **" + truncateRunes(title, 200) + "** needs attention: " + failureReason(ref.Input.FailedRecon)
	content = truncateRunes(content, max(2000-utf8.RuneCountInString(suffix), 0)) + suffix
	_, err := s.Discord.CreateMessage(ref.ChannelId, discord.Message{
		Embeds:           []discord.Embed{},
		Components:       []discord.ActionRow{},
		AllowedMentions:  discord.AllowedMentions{Users: users},
		Content:          content,
		MessageReference: &discord.MessageReference{MessageId: ref.MessageId, FailIfNotExists: false},
	})
	return err
}

// syncDiscordShowLocked returns the show message's location when one exists after the sync (nil otherwise).
// Only a show with a Whatnot show bound has a message ("message only about real shows"): an unbound, cancelled or
// deleted show has its message removed. A bound show without a message gets one only once it is displayed as live
// or later; a planned show is left alone, and a stored message id that Discord reports gone is cleared for a planned
// show instead of recreated.
func (s *Service) syncDiscordShowLocked(showId int64) (*discordMessageRef, error) {
	stream, err := s.StreamRepositorier.Get(showId)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil // deleted or unknown show; its message (if any) is left as it is
	}
	if err != nil {
		return nil, err
	}
	show, err := s.StreamRepositorier.GetWhatnotShow(showId)
	if err != nil {
		return nil, err
	}
	feed, err := s.DiscordFeedChannelRepositorier.GetByChannel(stream.ChannelId)
	if err != nil || feed == nil {
		return nil, err
	}
	msg, err := s.DiscordShowMessageRepositorier.Get(stream.Id)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		msg = &entity.DiscordShowMessage{ShowId: stream.Id, View: entity.DiscordViewOverview}
	}
	hasMessage := msg.DiscordMessageId != nil && *msg.DiscordMessageId != ""
	postedIn := feed.DiscordChannelId
	if msg.DiscordChannelId != nil && *msg.DiscordChannelId != "" {
		postedIn = *msg.DiscordChannelId
	}

	if show == nil || stream.Status == entity.StreamStatusCancelled {
		if hasMessage {
			if err := s.Discord.DeleteMessage(postedIn, *msg.DiscordMessageId); err != nil && !errors.Is(err, discord.ErrMessageGone) {
				return nil, err
			}
			return nil, s.DiscordShowMessageRepositorier.SetMessage(stream.Id, postedIn, "")
		}
		return nil, nil
	}

	in, err := s.loadDiscordShowInput(stream, show, msg.View)
	if err != nil {
		return nil, err
	}
	payload := renderDiscordShow(in)
	planned := showDisplayState(stream) == showStatePlanned

	if !hasMessage && planned {
		return nil, nil
	}
	if hasMessage {
		err := s.Discord.EditMessage(postedIn, *msg.DiscordMessageId, payload)
		if err == nil {
			s.pinDiscordShowIfLive(stream.Id, postedIn, *msg.DiscordMessageId, in, msg.Pinned)
			return &discordMessageRef{ChannelId: postedIn, MessageId: *msg.DiscordMessageId, Input: in}, nil
		}
		if !errors.Is(err, discord.ErrMessageGone) {
			return &discordMessageRef{ChannelId: postedIn, MessageId: *msg.DiscordMessageId, Input: in}, err
		}
		if planned {
			return nil, s.DiscordShowMessageRepositorier.SetMessage(stream.Id, postedIn, "")
		}
	}
	messageId, err := s.Discord.CreateMessage(feed.DiscordChannelId, payload)
	if err != nil {
		return nil, err
	}
	ref := &discordMessageRef{ChannelId: feed.DiscordChannelId, MessageId: messageId, Input: in}
	if err := s.DiscordShowMessageRepositorier.SetMessage(stream.Id, feed.DiscordChannelId, messageId); err != nil {
		return ref, err
	}
	s.pinDiscordShowIfLive(stream.Id, feed.DiscordChannelId, messageId, in, false)
	return ref, nil
}

// pinDiscordShowIfLive pins the message once while the show is displayed as live, then removes the system notice.
// A pinned message is unpinned once the show is no longer live. Failures are only logged.
func (s *Service) pinDiscordShowIfLive(showId int64, channelId, messageId string, in discordShowInput, pinned bool) {
	live := showDisplayState(in.Stream) == showStateLive
	if pinned && !live {
		if err := s.Discord.UnpinMessage(channelId, messageId); err != nil {
			discordLogf("unpin show %d: %s", showId, err.Error())
			return
		}
		if err := s.DiscordShowMessageRepositorier.SetPinned(showId, false); err != nil {
			discordLogf("unpin show %d: save state: %s", showId, err.Error())
		}
		return
	}
	if pinned || !live {
		return
	}
	if err := s.Discord.PinMessage(channelId, messageId); err != nil {
		discordLogf("pin show %d: %s", showId, err.Error())
		return
	}
	if err := s.DiscordShowMessageRepositorier.SetPinned(showId, true); err != nil {
		discordLogf("pin show %d: save state: %s", showId, err.Error())
	}
	if err := s.Discord.DeletePinNotice(channelId, messageId); err != nil {
		discordLogf("pin show %d: remove notice: %s", showId, err.Error())
	}
}

// loadDiscordShowInput loads everything the card shows. Discord's tables are in the holder database and the show
// in mobwebapp's, so nothing here joins them.
func (s *Service) loadDiscordShowInput(stream *entity.Stream, show *entity.WhatnotShow, view string) (discordShowInput, error) {
	view, selectedBreakId := parseDiscordView(view)
	in := discordShowInput{Show: show, Stream: stream, View: view}
	switch view {
	case entity.DiscordViewStats, entity.DiscordViewReconciliation:
	default:
		in.View = entity.DiscordViewOverview
	}
	if channel, err := s.ChannelRepositorier.Get(stream.ChannelId); err == nil && channel != nil {
		in.ChannelName = channel.Name
	}
	if s.FrontendUrl != "" {
		in.StreamUrl = fmt.Sprintf("%s/stream/%d", s.FrontendUrl, stream.Id)
	}
	// The overview status line needs the reason, so every view loads the latest reconciliation row.
	row, err := s.GetReconciliationByStream(stream.Id, show.ShowId)
	if err != nil {
		return in, err
	}
	in.Reconciliation = row
	if stream.Status == entity.StreamStatusFailedToVerify {
		in.FailedRecon = row
	}

	if in.View == entity.DiscordViewStats {
		report, err := s.buildShowStats(show, stream, selectedBreakId)
		if err != nil {
			return in, err
		}
		in.StatsReport = report
	}
	return in, nil
}

// streamStats returns the stream's sales totals; nil when it has none (or they could not be read).
func (s *Service) streamStats(stream *entity.Stream) *entity.StreamStatistic {
	stats, err := s.StreamRepositorier.GetStats(stream.Id)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			discordLogf("stream %d stats: %s", stream.Id, err.Error())
		}
		return nil
	}
	return stats
}

// syncDiscordShowAsync never blocks the caller; a no-op while Discord is disabled.
func (s *Service) syncDiscordShowAsync(showId int64) {
	s.syncDiscordShowAsyncAlert(showId, false)
}

// syncDiscordShowAsyncAlert with alert pings once when the show has a message, in the same goroutine as the sync.
func (s *Service) syncDiscordShowAsyncAlert(showId int64, alert bool) {
	if !s.discordEnabled() {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				discordLogf("sync show %d: panic: %v", showId, r)
			}
		}()
		s.syncDiscordShow(showId, alert)
	}()
}
