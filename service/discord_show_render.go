package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/SaloEater/WhatNot-Webhook-Holder/discord"
	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
)

const (
	discordMaxFields      = 25
	discordMaxFieldValue  = 1024
	discordMaxFieldName   = 256
	discordMaxTitle       = 256
	discordMaxDescription = 4096
	discordMaxFooter      = 2048
	// Discord allows 6000 characters over the whole embed; keep a margin.
	discordMaxEmbedTotal = 5800

	discordColorPlanned  = 0x95A5A6
	discordColorLive     = 0x2ECC71
	discordColorEnded    = 0x3498DB
	discordColorFailed   = 0xE74C3C
	discordColorVerified = 0xF1C40F
)

const (
	showStatePlanned = iota
	showStateLive
	showStateEnded
	showStateFailed
	showStateVerified
)

// whatnotShowURL is the public page of a show on Whatnot. Unverified assumption (plan, open item 1):
// change it here only.
func whatnotShowURL(showId string) string {
	return "https://www.whatnot.com/live/" + showId
}

// discordReconInput is one linked stream with its reconciliation of this show (Row nil = none yet).
type discordReconInput struct {
	Stream *entity.Stream
	Row    *entity.ShowReconciliation
}

// discordShowInput is everything renderDiscordShow needs; the function itself does no I/O.
type discordShowInput struct {
	Show            *entity.WhatnotShow
	View            string
	ChannelName     string
	Streams         []*entity.Stream
	FailedStreamId  int64                      // stream FailedRecon belongs to; 0 if none
	StreamUrl       string                     // ready "Open stream" link; empty = no button
	FailedRecon     *entity.ShowReconciliation // latest row of the failed_to_verify stream; nil if none/not loaded
	Stats           *entity.StreamStatistic // merged over the streams; nil when none could be loaded
	Reconciliations []discordReconInput
	// ButtonPrefix starts the view buttons' custom ids: "show" (default) for the feed message, "stream" for a
	// /stream-status reply, whose buttons switch only that reply.
	ButtonPrefix string
	// StatsReport is the Stats view content (first stream of the show); nil when there is no stream.
	StatsReport *showStatsReport
}

// showDisplayState is the furthest of the show's own status and its linked streams' statuses.
func showDisplayState(show *entity.WhatnotShow, streams []*entity.Stream) int {
	state := showStatePlanned
	switch show.Status {
	case entity.WhatnotShowStatusLive:
		state = showStateLive
	case entity.WhatnotShowStatusEnded:
		state = showStateEnded
	}
	verified := len(streams) > 0
	failed := false
	for _, st := range streams {
		switch st.Status {
		case entity.StreamStatusFailedToVerify:
			failed = true
			verified = false
		case entity.StreamStatusVerified:
			if state < showStateEnded {
				state = showStateEnded
			}
		case entity.StreamStatusEnded:
			if state < showStateEnded {
				state = showStateEnded
			}
			verified = false
		case entity.StreamStatusLive:
			if state < showStateLive {
				state = showStateLive
			}
			verified = false
		default:
			verified = false
		}
	}
	if verified {
		return showStateVerified
	}
	if failed {
		return showStateFailed
	}
	return state
}

// failureReason explains a failed_to_verify stream from its latest reconciliation row.
func failureReason(row *entity.ShowReconciliation) string {
	if row == nil {
		return "verification failed"
	}
	switch row.Status {
	case entity.ReconciliationStatusNeedsManual:
		var rep ReconReport
		if len(row.Report) > 0 && json.Unmarshal(row.Report, &rep) == nil && rep.Blocking > 0 {
			word := "issues"
			if rep.Blocking == 1 {
				word = "issue"
			}
			return fmt.Sprintf("%d blocking %s need manual review", rep.Blocking, word)
		}
		return "blocking issues need manual review"
	case entity.ReconciliationStatusFailed:
		return "reconciliation run failed"
	}
	return "verification failed"
}

func renderDiscordShow(in discordShowInput) discord.Message {
	show := in.Show
	state := showDisplayState(show, in.Streams)

	color, statusLine := discordColorPlanned, "🗓️ Scheduled"
	switch state {
	case showStatePlanned:
		if show.ScheduledDate != nil {
			statusLine += fmt.Sprintf(" for <t:%d:F> (<t:%d:R>)", show.ScheduledDate.Unix(), show.ScheduledDate.Unix())
		}
	case showStateLive:
		color, statusLine = discordColorLive, "🔴 Live"
		if show.StartedAt != nil {
			statusLine += fmt.Sprintf(" since <t:%d:R>", show.StartedAt.Unix())
		}
	case showStateEnded:
		color, statusLine = discordColorEnded, "🏁 Ended"
		if show.StartedAt != nil && show.EndedAt != nil && show.EndedAt.After(*show.StartedAt) {
			statusLine += " (" + formatDiscordDuration(show.EndedAt.Sub(*show.StartedAt)) + ")"
		}
	case showStateFailed:
		color, statusLine = discordColorFailed, "⚠️ Verification failed"
		if reason := failureReason(in.FailedRecon); reason != "" {
			statusLine += ": " + reason
		}
	case showStateVerified:
		color, statusLine = discordColorVerified, "✅ Verified"
	}

	description := statusLine
	if show.Description != "" {
		description += "\n\n" + show.Description
	}
	title := show.Title
	if title == "" {
		title = "Whatnot show"
	}
	link := whatnotShowURL(show.ShowId)

	embed := discord.Embed{
		Title:       truncateRunes(title, discordMaxTitle),
		URL:         link,
		Description: truncateRunes(description, discordMaxDescription),
		Color:       color,
	}
	if show.ThumbnailUrl != "" {
		embed.Image = &discord.EmbedImage{URL: show.ThumbnailUrl}
	}
	if in.ChannelName != "" {
		embed.Footer = &discord.EmbedFooter{Text: truncateRunes(in.ChannelName, discordMaxFooter)}
	}

	switch in.View {
	case entity.DiscordViewStats:
		embed.Fields = renderStatsFields(in)
	case entity.DiscordViewReconciliation:
		embed.Fields = renderReconFields(in)
	}
	fitEmbed(&embed)

	components := renderButtons(show, in.View, len(in.Streams) > 0, link, in.StreamUrl, in.ButtonPrefix)
	if in.View == entity.DiscordViewStats {
		prefix := in.ButtonPrefix
		if prefix == "" {
			prefix = "show"
		}
		components = append(components, renderStatsNav(in.StatsReport, prefix, show.Id)...)
	}
	return discord.Message{
		Embeds:          []discord.Embed{embed},
		Components:      components,
		AllowedMentions: discord.AllowedMentions{Parse: []string{}},
	}
}

func renderButtons(show *entity.WhatnotShow, view string, hasStreams bool, link string, streamUrl string, prefix string) []discord.ActionRow {
	if prefix == "" {
		prefix = "show"
	}
	viewRow := discord.ActionRow{Type: discord.ComponentTypeActionRow}
	views := []struct{ view, label string }{
		{entity.DiscordViewOverview, "Overview"},
		{entity.DiscordViewStats, "Stats"},
		{entity.DiscordViewReconciliation, "Reconciliation"},
	}
	for _, v := range views {
		btn := discord.Component{
			Type:     discord.ComponentTypeButton,
			Style:    discord.ButtonStyleSecondary,
			Label:    v.label,
			CustomId: fmt.Sprintf("%s:view:%s:%d", prefix, v.view, show.Id),
		}
		if v.view == view {
			btn.Style, btn.Disabled = discord.ButtonStylePrimary, true
		} else if v.view != entity.DiscordViewOverview && !hasStreams {
			btn.Disabled = true
		}
		viewRow.Components = append(viewRow.Components, btn)
	}
	linkRow := discord.ActionRow{Type: discord.ComponentTypeActionRow, Components: []discord.Component{
		{Type: discord.ComponentTypeButton, Style: discord.ButtonStyleLink, Label: "Open show", URL: link},
	}}
	if streamUrl != "" {
		linkRow.Components = append(linkRow.Components,
			discord.Component{Type: discord.ComponentTypeButton, Style: discord.ButtonStyleLink, Label: "Open stream", URL: streamUrl})
	}
	return []discord.ActionRow{viewRow, linkRow}
}

func renderStatsFields(in discordShowInput) []discord.EmbedField {
	if len(in.Streams) == 0 || in.StatsReport == nil {
		return []discord.EmbedField{{Name: "Stats", Value: "No stream linked yet"}}
	}
	return renderStatsReport(in.StatsReport)
}

func renderReconFields(in discordShowInput) []discord.EmbedField {
	if len(in.Reconciliations) == 0 {
		return []discord.EmbedField{{Name: "Reconciliation", Value: "No stream linked yet"}}
	}
	fields := make([]discord.EmbedField, 0, len(in.Reconciliations))
	for _, rc := range in.Reconciliations {
		name := "Reconciliation"
		if rc.Stream != nil && (len(in.Reconciliations) > 1 || rc.Stream.Name != "") {
			name = "Reconciliation: " + rc.Stream.Name
		}
		fields = append(fields, discord.EmbedField{Name: name, Value: renderReconValue(rc.Row)})
	}
	return fields
}

func renderReconValue(row *entity.ShowReconciliation) string {
	if row == nil {
		return "Not reconciled yet"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Status: **%s**", row.Status)
	switch row.Status {
	case entity.ReconciliationStatusNeedsManual:
		b.WriteString(" - " + failureReason(row))
	case entity.ReconciliationStatusFailed:
		b.WriteString(" - reconciliation run failed")
	}
	if row.FinishedAt != nil {
		fmt.Fprintf(&b, " (<t:%d:R>)", row.FinishedAt.Unix())
	}
	if row.Error != "" {
		fmt.Fprintf(&b, "\nError: %s", truncateRunes(row.Error, 200))
	}
	var rep ReconReport
	if len(row.Report) == 0 || json.Unmarshal(row.Report, &rep) != nil || rep.OrdersScanned == 0 {
		return b.String()
	}
	var matched, toFix, toAdd, extra int
	for _, br := range rep.Breaks {
		matched += br.Matched
		toFix += br.ToFix + br.ToFixPrice
		toAdd += len(br.ToAdd)
		extra += len(br.Extra)
	}
	fmt.Fprintf(&b, "\nOrders scanned: %d", rep.OrdersScanned)
	fmt.Fprintf(&b, "\nBlocking: %d", rep.Blocking)
	fmt.Fprintf(&b, "\nTeams: %d matched, %d to fix, %d to add, %d extra", matched, toFix, toAdd, extra)
	fmt.Fprintf(&b, "\nGiveaways: %d to add, %d unplaced, %d extra", rep.Giveaways.ToAdd, len(rep.Giveaways.Unplaced), len(rep.Giveaways.Extra))
	fmt.Fprintf(&b, "\nUnparsed: %d", len(rep.Unparsed))
	return b.String()
}

// fitEmbed truncates fields to Discord's limits and drops trailing fields until the embed fits the total.
func fitEmbed(e *discord.Embed) {
	if len(e.Fields) > discordMaxFields {
		e.Fields = e.Fields[:discordMaxFields]
	}
	for i := range e.Fields {
		e.Fields[i].Name = truncateRunes(e.Fields[i].Name, discordMaxFieldName)
		e.Fields[i].Value = truncateRunes(e.Fields[i].Value, discordMaxFieldValue)
		if e.Fields[i].Value == "" {
			e.Fields[i].Value = "-"
		}
	}
	for len(e.Fields) > 0 && embedLength(e) > discordMaxEmbedTotal {
		e.Fields = e.Fields[:len(e.Fields)-1]
	}
	if over := embedLength(e) - discordMaxEmbedTotal; over > 0 {
		e.Description = truncateRunes(e.Description, max(utf8.RuneCountInString(e.Description)-over, 0))
	}
}

func embedLength(e *discord.Embed) int {
	n := utf8.RuneCountInString(e.Title) + utf8.RuneCountInString(e.Description)
	if e.Footer != nil {
		n += utf8.RuneCountInString(e.Footer.Text)
	}
	for _, f := range e.Fields {
		n += utf8.RuneCountInString(f.Name) + utf8.RuneCountInString(f.Value)
	}
	return n
}

func truncateRunes(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	if limit <= 1 {
		return string([]rune(s)[:max(limit, 0)])
	}
	return string([]rune(s)[:limit-1]) + "…"
}

func formatDiscordDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h, m := int(d.Hours()), int(d.Minutes())%60
	if h == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}
