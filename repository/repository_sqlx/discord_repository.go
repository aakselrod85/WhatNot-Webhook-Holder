package repository_sqlx

import (
	"database/sql"
	"errors"

	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
	"github.com/jmoiron/sqlx"
)

type DiscordFeedChannelRepository struct {
	DB *sqlx.DB
}

func (r *DiscordFeedChannelRepository) GetByChannel(channelId int64) (*entity.DiscordFeedChannel, error) {
	var v entity.DiscordFeedChannel
	err := r.DB.Unsafe().Get(&v, `SELECT * FROM discord_feed_channel WHERE channel_id = $1`, channelId)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &v, err
}

func (r *DiscordFeedChannelRepository) GetByDiscordChannelId(discordChannelId string) (*entity.DiscordFeedChannel, error) {
	var v entity.DiscordFeedChannel
	err := r.DB.Unsafe().Get(&v, `SELECT * FROM discord_feed_channel WHERE discord_channel_id = $1 LIMIT 1`, discordChannelId)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &v, err
}

type DiscordShowMessageRepository struct {
	DB *sqlx.DB
}

func (r *DiscordShowMessageRepository) Get(showId int64) (*entity.DiscordShowMessage, error) {
	var v entity.DiscordShowMessage
	err := r.DB.Unsafe().Get(&v, `SELECT * FROM discord_show_message WHERE whatnot_show_id = $1`, showId)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &v, err
}

func (r *DiscordShowMessageRepository) SetView(showId int64, view string) error {
	_, err := r.DB.Exec(`INSERT INTO discord_show_message (whatnot_show_id, view) VALUES ($1, $2)
ON CONFLICT (whatnot_show_id) DO UPDATE SET view = EXCLUDED.view, updated_at = now()`, showId, view)
	return err
}

func (r *DiscordShowMessageRepository) SetMessage(showId int64, discordChannelId string, messageId string) error {
	_, err := r.DB.Exec(`INSERT INTO discord_show_message (whatnot_show_id, discord_channel_id, discord_message_id)
VALUES ($1, NULLIF($2, ''), NULLIF($3, ''))
ON CONFLICT (whatnot_show_id) DO UPDATE SET
	discord_channel_id = EXCLUDED.discord_channel_id,
	discord_message_id = EXCLUDED.discord_message_id,
	pinned = false,
	updated_at = now()`, showId, discordChannelId, messageId)
	return err
}

func (r *DiscordShowMessageRepository) SetPinned(showId int64, pinned bool) error {
	_, err := r.DB.Exec(`UPDATE discord_show_message SET pinned = $2, updated_at = now() WHERE whatnot_show_id = $1`, showId, pinned)
	return err
}
