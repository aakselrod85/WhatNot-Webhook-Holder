package service

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/SaloEater/WhatNot-Webhook-Holder/discord"
	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
)

// /stream-status <id> posts the current status of one stream (by its id in this system) to the channel.
const streamStatusCommand = "stream-status"

// RegisterDiscordCommands registers the slash commands as server commands in every server the bot is in
// (replacing whatever was registered before). Silent no-op without the bot token or application id; failures are
// only logged. Meant to run in a goroutine at startup.
func (s *Service) RegisterDiscordCommands(applicationId string) {
	defer func() {
		if r := recover(); r != nil {
			discordLogf("register commands: panic: %v", r)
		}
	}()
	if !s.discordEnabled() || applicationId == "" {
		return
	}
	guilds, err := s.Discord.ListGuildIds()
	if err != nil {
		discordLogf("register commands: %s", err.Error())
		return
	}
	commands := []map[string]any{{
		"name":        streamStatusCommand,
		"description": "Show the status of a stream",
		"type":        1,
		"options": []map[string]any{{
			"type":        4, // integer
			"name":        "id",
			"description": "Stream id (as in the admin UI)",
			"required":    true,
			"min_value":   1,
		}},
	}}
	for _, g := range guilds {
		if err := s.Discord.SetGuildCommands(applicationId, g, commands); err != nil {
			discordLogf("register commands in guild %s: %s", g, err.Error())
		}
	}
}

// HandleDiscordCommand runs an application command in the background and always ends by replacing the deferred
// reply with a final message. The caller answers the interaction itself.
func (s *Service) HandleDiscordCommand(in discord.Interaction) {
	go func() {
		result := discordTextMessage("Something went wrong")
		defer func() {
			if r := recover(); r != nil {
				discordLogf("command %q: panic: %v", in.Data.Name, r)
				result = discordTextMessage("Something went wrong")
			}
			if err := s.Discord.EditInteractionResponse(in.ApplicationId, in.Token, result); err != nil {
				discordLogf("command %q: reply: %s", in.Data.Name, err.Error())
			}
		}()
		if in.Data.Name != streamStatusCommand {
			result = discordTextMessage("Unknown command")
			return
		}
		var streamId int64
		for _, o := range in.Data.Options {
			if o.Name == "id" {
				streamId = parseDiscordIntOption(o.Value)
			}
		}
		if streamId <= 0 {
			result = discordTextMessage("Please pass a stream id, e.g. `/stream-status id:123`")
			return
		}
		result = s.streamStatusMessage(streamId)
	}()
}

func parseDiscordIntOption(raw json.RawMessage) int64 {
	var n json.Number
	if json.Unmarshal(raw, &n) == nil {
		if v, err := n.Int64(); err == nil {
			return v
		}
	}
	var str string
	if json.Unmarshal(raw, &str) == nil {
		if v, err := strconv.ParseInt(strings.TrimSpace(str), 10, 64); err == nil {
			return v
		}
	}
	return 0
}

func discordTextMessage(text string) discord.Message {
	return discord.Message{
		Content:         truncateRunes(text, 2000),
		Embeds:          []discord.Embed{},
		Components:      []discord.ActionRow{},
		AllowedMentions: discord.AllowedMentions{Parse: []string{}},
	}
}

// streamStatusMessage loads the stream and everything shown about it and renders the reply.
func (s *Service) streamStatusMessage(streamId int64) discord.Message {
	stream, err := s.StreamRepositorier.Get(streamId)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (stream == nil || stream.Id == 0)) {
		return discordTextMessage(fmt.Sprintf("Stream #%d not found", streamId))
	}
	if err != nil {
		discordLogf("stream status %d: %s", streamId, err.Error())
		return discordTextMessage("Could not load the stream, try again later")
	}

	shows, err := s.WhatnotShowRepositorier.GetByStream(stream.Id)
	if err != nil {
		discordLogf("stream status %d: shows: %s", streamId, err.Error())
	}
	// A stream linked to a Whatnot show gets the same card as the show's feed message.
	if len(shows) > 0 {
		if msg, err := s.streamShowCard(shows[0], entity.DiscordViewOverview); err == nil {
			return msg
		} else {
			discordLogf("stream status %d: show card: %s", streamId, err.Error())
		}
	}

	in := streamStatusInput{Stream: stream}
	if channel, err := s.ChannelRepositorier.Get(stream.ChannelId); err == nil && channel != nil {
		in.ChannelName = channel.Name
	}
	in.Shows = shows
	if stream.Status == entity.StreamStatusFailedToVerify && len(shows) > 0 {
		row, err := s.GetReconciliationByStream(stream.Id, shows[0].ShowId)
		if err != nil {
			discordLogf("stream status %d: reconciliation: %s", streamId, err.Error())
		}
		in.FailedRecon = row
	}
	in.Stats = s.mergedStreamStats([]*entity.Stream{stream})
	if s.FrontendUrl != "" {
		in.StreamUrl = fmt.Sprintf("%s/stream/%d", s.FrontendUrl, stream.Id)
	}
	return renderStreamStatus(in)
}

type streamStatusInput struct {
	Stream      *entity.Stream
	ChannelName string
	Shows       []*entity.WhatnotShow // most recently linked first
	FailedRecon *entity.ShowReconciliation
	Stats       *entity.StreamStatistic // nil = no sales yet
	StreamUrl   string                  // empty = no link
}

func renderStreamStatus(in streamStatusInput) discord.Message {
	st := in.Stream
	color, statusLine := discordColorPlanned, "🗓️ Planned"
	switch st.Status {
	case entity.StreamStatusLive:
		color, statusLine = discordColorLive, "🔴 Live"
	case entity.StreamStatusEnded:
		color, statusLine = discordColorEnded, "🏁 Ended"
	case entity.StreamStatusFailedToVerify:
		color, statusLine = discordColorFailed, "⚠️ Verification failed: "+failureReason(in.FailedRecon)
	case entity.StreamStatusVerified:
		color, statusLine = discordColorVerified, "✅ Verified"
	}

	title := st.Name
	if title == "" {
		title = fmt.Sprintf("Stream #%d", st.Id)
	}
	embed := discord.Embed{
		Title:       truncateRunes(title, discordMaxTitle),
		URL:         in.StreamUrl,
		Description: statusLine,
		Color:       color,
		Footer:      &discord.EmbedFooter{Text: fmt.Sprintf("Stream #%d", st.Id)},
	}
	if in.ChannelName != "" {
		embed.Fields = append(embed.Fields, discord.EmbedField{Name: "Channel", Value: in.ChannelName, Inline: true})
	}
	embed.Fields = append(embed.Fields, discord.EmbedField{Name: "Created", Value: fmt.Sprintf("<t:%d:f>", st.CreatedAt.Unix()), Inline: true})

	if len(in.Shows) > 0 {
		lines := make([]string, 0, len(in.Shows))
		for _, sh := range in.Shows {
			name := sh.Title
			if name == "" {
				name = "Whatnot show"
			}
			lines = append(lines, fmt.Sprintf("[%s](%s)", strings.ReplaceAll(name, "]", ")"), whatnotShowURL(sh.ShowId)))
		}
		embed.Fields = append(embed.Fields, discord.EmbedField{Name: "Whatnot show", Value: strings.Join(lines, "\n")})
		if sh := in.Shows[0]; sh.ThumbnailUrl != "" {
			embed.Image = &discord.EmbedImage{URL: sh.ThumbnailUrl}
		}
	}

	if in.Stats == nil {
		embed.Fields = append(embed.Fields, discord.EmbedField{Name: "Sales", Value: "No sales recorded yet"})
	} else {
		embed.Fields = append(embed.Fields,
			discord.EmbedField{Name: "Breaks", Value: fmt.Sprint(in.Stats.BreaksAmount), Inline: true},
			discord.EmbedField{Name: "Earned", Value: fmt.Sprintf("%d$", in.Stats.SoldFor), Inline: true},
			discord.EmbedField{Name: "Unique customers", Value: fmt.Sprint(in.Stats.CustomersAmount), Inline: true},
		)
	}
	fitEmbed(&embed)

	components := []discord.ActionRow{}
	if in.StreamUrl != "" {
		components = append(components, discord.ActionRow{Type: discord.ComponentTypeActionRow, Components: []discord.Component{
			{Type: discord.ComponentTypeButton, Style: discord.ButtonStyleLink, Label: "Open stream", URL: in.StreamUrl},
		}})
	}
	return discord.Message{
		Embeds:          []discord.Embed{embed},
		Components:      components,
		AllowedMentions: discord.AllowedMentions{Parse: []string{}},
	}
}

// streamShowCard renders the show's feed card for a /stream-status reply; its view buttons use the "stream" prefix.
func (s *Service) streamShowCard(show *entity.WhatnotShow, view string) (discord.Message, error) {
	in, err := s.loadDiscordShowInput(show, view)
	if err != nil {
		return discord.Message{}, err
	}
	in.ButtonPrefix = "stream"
	return renderDiscordShow(in), nil
}
