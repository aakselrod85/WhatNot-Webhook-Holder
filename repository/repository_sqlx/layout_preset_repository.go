package repository_sqlx

import (
	"database/sql"

	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
	"github.com/jmoiron/sqlx"
)

type LayoutPresetRepository struct {
	DB *sqlx.DB
}

func (r *LayoutPresetRepository) ListLayoutPresets(channelId int64) ([]entity.LayoutPreset, error) {
	presets := []entity.LayoutPreset{}
	err := r.DB.Select(&presets, `SELECT * FROM layout_preset WHERE channel_id = $1 ORDER BY name`, channelId)
	if err != nil {
		return nil, err
	}
	return presets, nil
}

func (r *LayoutPresetRepository) GetLayoutPreset(id int64) (*entity.LayoutPreset, error) {
	var p entity.LayoutPreset
	err := r.DB.Get(&p, `SELECT * FROM layout_preset WHERE id = $1`, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *LayoutPresetRepository) CountLayoutPresets(channelId int64) (int, error) {
	var count int
	err := r.DB.Get(&count, `SELECT COUNT(*) FROM layout_preset WHERE channel_id = $1`, channelId)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *LayoutPresetRepository) CreateLayoutPreset(channelId int64, name string, config []byte) (*entity.LayoutPreset, error) {
	var p entity.LayoutPreset
	err := r.DB.Get(&p, `
		INSERT INTO layout_preset (channel_id, name, config)
		VALUES ($1, $2, $3)
		RETURNING *`, channelId, name, string(config))
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *LayoutPresetRepository) UpdateLayoutPreset(id int64, name *string, config []byte) error {
	var cfg interface{}
	if len(config) > 0 {
		cfg = string(config)
	}
	_, err := r.DB.Exec(`
		UPDATE layout_preset SET
			name = COALESCE($2, name),
			config = COALESCE($3, config),
			updated_at = now()
		WHERE id = $1`, id, name, cfg)
	return err
}

func (r *LayoutPresetRepository) DeleteLayoutPreset(id int64) error {
	_, err := r.DB.Exec(`DELETE FROM layout_preset WHERE id = $1`, id)
	return err
}
