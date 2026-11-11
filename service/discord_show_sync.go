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

// discordShowLocks holds one mutex per whatnot_show id (in-process; the backend runs as a single instance).
var discordShowLocks sync.Map

func (s *Service) discordEnabled() bool {
	return s.Discord.Enabled()
}

func discordLogf(format string, args ...any) {
	fmt.Println("discord: " + fmt.Sprintf(format, args...))
}

// SyncDiscordShow rebuilds the show's Discord message from database state and creates, edits or deletes it.
// It never returns an error: failures are logged so they can not affect the caller's request.
func (s *Service) SyncDiscordShow(whatnotShowId int64) {
	s.syncDiscordShow(whatnotShowId, false)
}

// syncDiscordShow is SyncDiscordShow; with alert it also sends the failure ping after the message sync, under the same lock.
// The returned error (already logged) is the message sync's.
func (s *Service) syncDiscordShow(whatnotShowId int64, alert bool) (err error) {
	defer func() {
		if r := recover(); r != nil {
			discordLogf("sync show %d: panic: %v", whatnotShowId, r)
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	if !s.discordEnabled() {
		return nil
	}
	lock, _ := discordShowLocks.LoadOrStore(whatnotShowId, &sync.Mutex{})
	mu := lock.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	ref, err := s.syncDiscordShowLocked(whatnotShowId)
	if err != nil {
		discordLogf("sync show %d: %s", whatnotShowId, err.Error())
	}
	if alert && ref != nil {
		// A failed ping is only logged.
		if err := s.sendDiscordAlert(ref); err != nil {
			discordLogf("alert show %d: %s", whatnotShowId, err.Error())
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
	if s.FrontendUrl != "" && ref.Input.FailedStreamId != 0 {
		suffix = fmt.Sprintf("\n<%s/stream/%d/reconciliation>", s.FrontendUrl, ref.Input.FailedStreamId)
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
// A show without a message gets one only once it is displayed as live or later; a planned show is left alone,
// and a stored message id that Discord reports gone is cleared for a planned show instead of recreated.
func (s *Service) syncDiscordShowLocked(whatnotShowId int64) (*discordMessageRef, error) {
	show, err := s.WhatnotShowRepositorier.Get(whatnotShowId)
	if err != nil || show == nil {
		return nil, err
	}
	feed, err := s.DiscordFeedChannelRepositorier.GetByChannel(show.ChannelId)
	if err != nil || feed == nil {
		return nil, err
	}
	msg, err := s.DiscordShowMessageRepositorier.Get(show.Id)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		msg = &entity.DiscordShowMessage{WhatnotShowId: show.Id, View: entity.DiscordViewOverview}
	}
	hasMessage := msg.DiscordMessageId != nil && *msg.DiscordMessageId != ""
	postedIn := feed.DiscordChannelId
	if msg.DiscordChannelId != nil && *msg.DiscordChannelId != "" {
		postedIn = *msg.DiscordChannelId
	}

	if show.Status == entity.WhatnotShowStatusCancelled {
		if hasMessage {
			if err := s.Discord.DeleteMessage(postedIn, *msg.DiscordMessageId); err != nil {
				return nil, err
			}
			return nil, s.DiscordShowMessageRepositorier.SetMessage(show.Id, postedIn, "")
		}
		return nil, nil
	}

	in, err := s.loadDiscordShowInput(show, msg.View)
	if err != nil {
		return nil, err
	}
	payload := renderDiscordShow(in)
	planned := showDisplayState(in.Show, in.Streams) == showStatePlanned

	if !hasMessage && planned {
		return nil, nil
	}
	if hasMessage {
		err := s.Discord.EditMessage(postedIn, *msg.DiscordMessageId, payload)
		if err == nil {
			s.pinDiscordShowIfLive(show.Id, postedIn, *msg.DiscordMessageId, in, msg.Pinned)
			return &discordMessageRef{ChannelId: postedIn, MessageId: *msg.DiscordMessageId, Input: in}, nil
		}
		if !errors.Is(err, discord.ErrMessageGone) {
			return &discordMessageRef{ChannelId: postedIn, MessageId: *msg.DiscordMessageId, Input: in}, err
		}
		if planned {
			return nil, s.DiscordShowMessageRepositorier.SetMessage(show.Id, postedIn, "")
		}
	}
	messageId, err := s.Discord.CreateMessage(feed.DiscordChannelId, payload)
	if err != nil {
		return nil, err
	}
	ref := &discordMessageRef{ChannelId: feed.DiscordChannelId, MessageId: messageId, Input: in}
	if err := s.DiscordShowMessageRepositorier.SetMessage(show.Id, feed.DiscordChannelId, messageId); err != nil {
		return ref, err
	}
	s.pinDiscordShowIfLive(show.Id, feed.DiscordChannelId, messageId, in, false)
	return ref, nil
}

// pinDiscordShowIfLive pins the message once while the show is displayed as live, then removes the system notice.
// A pinned message is unpinned once the show is no longer live. Failures are only logged.
func (s *Service) pinDiscordShowIfLive(showId int64, channelId, messageId string, in discordShowInput, pinned bool) {
	live := showDisplayState(in.Show, in.Streams) == showStateLive
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

func (s *Service) loadDiscordShowInput(show *entity.WhatnotShow, view string) (discordShowInput, error) {
	view, selectedBreakId := parseDiscordView(view)
	in := discordShowInput{Show: show, View: view}
	switch view {
	case entity.DiscordViewStats, entity.DiscordViewReconciliation:
	default:
		in.View = entity.DiscordViewOverview
	}
	if channel, err := s.ChannelRepositorier.Get(show.ChannelId); err == nil && channel != nil {
		in.ChannelName = channel.Name
	}
	streams, err := s.WhatnotShowRepositorier.GetStreams(show.Id)
	if err != nil {
		return in, err
	}
	in.Streams = streams
	if s.FrontendUrl != "" {
		latest, err := s.WhatnotShowRepositorier.GetLatestStream(show.Id)
		if err != nil {
			return in, err
		}
		if latest != nil {
			in.StreamUrl = fmt.Sprintf("%s/stream/%d", s.FrontendUrl, latest.Id)
		}
	}
	// The overview status line needs the reason, so every view loads the failing stream's latest row.
	for _, st := range streams {
		if st.Status == entity.StreamStatusFailedToVerify {
			row, err := s.GetReconciliationByStream(st.Id, show.ShowId)
			if err != nil {
				return in, err
			}
			in.FailedRecon = row
			in.FailedStreamId = st.Id
			break
		}
	}

	switch in.View {
	case entity.DiscordViewStats:
		report, err := s.buildShowStats(show, streams, selectedBreakId)
		if err != nil {
			return in, err
		}
		in.StatsReport = report
	case entity.DiscordViewReconciliation:
		for _, st := range streams {
			row, err := s.GetReconciliationByStream(st.Id, show.ShowId)
			if err != nil {
				return in, err
			}
			in.Reconciliations = append(in.Reconciliations, discordReconInput{Stream: st, Row: row})
		}
	}
	return in, nil
}

// mergedStreamStats sums the stats of the streams and merges their name lists; nil when none has sales.
func (s *Service) mergedStreamStats(streams []*entity.Stream) *entity.StreamStatistic {
	var merged *entity.StreamStatistic
	seenBig, seenLucky := map[string]bool{}, map[string]bool{}
	for _, st := range streams {
		stats, err := s.StreamRepositorier.GetStats(st.Id)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				discordLogf("stream %d stats: %s", st.Id, err.Error())
			}
			continue
		}
		if stats == nil {
			continue
		}
		if merged == nil {
			merged = &entity.StreamStatistic{Name: stats.Name, ChannelName: stats.ChannelName}
		}
		merged.BreaksAmount += stats.BreaksAmount
		merged.SoldFor += stats.SoldFor
		merged.CustomersAmount += stats.CustomersAmount
		for _, c := range stats.BigCustomers {
			if !seenBig[c] {
				seenBig[c] = true
				merged.BigCustomers = append(merged.BigCustomers, c)
			}
		}
		for _, c := range stats.LuckyGoblins {
			if !seenLucky[c] {
				seenLucky[c] = true
				merged.LuckyGoblins = append(merged.LuckyGoblins, c)
			}
		}
	}
	return merged
}

// syncDiscordShowAsync never blocks the caller; a no-op while Discord is disabled.
func (s *Service) syncDiscordShowAsync(whatnotShowId int64) {
	if !s.discordEnabled() {
		return
	}
	go s.SyncDiscordShow(whatnotShowId)
}

// SyncDiscordShowsOfStreamAsync syncs every show linked to the stream. No linked show is a silent no-op.
func (s *Service) SyncDiscordShowsOfStreamAsync(streamId int64) {
	s.syncDiscordShowsOfStreamAsync(streamId, false)
}

// syncDiscordShowsOfStreamAsync with alert pings once per show that has a message, in the same goroutine as the sync.
func (s *Service) syncDiscordShowsOfStreamAsync(streamId int64, alert bool) {
	if !s.discordEnabled() {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				discordLogf("sync shows of stream %d: panic: %v", streamId, r)
			}
		}()
		shows, err := s.WhatnotShowRepositorier.GetByStream(streamId)
		if err != nil {
			discordLogf("sync shows of stream %d: %s", streamId, err.Error())
			return
		}
		for _, show := range shows {
			s.syncDiscordShow(show.Id, alert)
		}
	}()
}
