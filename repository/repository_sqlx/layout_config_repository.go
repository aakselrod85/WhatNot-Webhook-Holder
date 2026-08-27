package repository_sqlx

import (
	"database/sql"

	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
	"github.com/jmoiron/sqlx"
)

type LayoutConfigRepository struct {
	DB *sqlx.DB
}

func (r *LayoutConfigRepository) GetLayoutConfig(channelId int64) (*entity.LayoutConfig, error) {
	var c entity.LayoutConfig
	err := r.DB.Get(&c, `SELECT * FROM layout_config WHERE channel_id = $1`, channelId)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *LayoutConfigRepository) UpsertLayoutConfig(channelId int64, config []byte) error {
	_, err := r.DB.Exec(`
		INSERT INTO layout_config (channel_id, config, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (channel_id) DO UPDATE SET
			config = EXCLUDED.config,
			updated_at = now()`, channelId, string(config))
	return err
}
