package repository

import "github.com/SaloEater/WhatNot-Webhook-Holder/entity"

type DiscordFeedChannelRepositorier interface {
	// GetByChannel returns nil, nil when the channel has no Discord feed.
	GetByChannel(channelId int64) (*entity.DiscordFeedChannel, error)
	// GetByDiscordChannelId returns nil, nil when no feed uses that Discord channel.
	GetByDiscordChannelId(discordChannelId string) (*entity.DiscordFeedChannel, error)
}

type DiscordShowMessageRepositorier interface {
	// Keyed by the mobwebapp show id (the column is still named whatnot_show_id).
	// Get returns nil, nil when the show has no row yet.
	Get(showId int64) (*entity.DiscordShowMessage, error)
	// SetView creates the row when missing.
	SetView(showId int64, view string) error
	// SetMessage stores where the message was posted; an empty messageId clears it.
	SetMessage(showId int64, discordChannelId string, messageId string) error
	// SetPinned records that the show message is pinned.
	SetPinned(showId int64, pinned bool) error
}
