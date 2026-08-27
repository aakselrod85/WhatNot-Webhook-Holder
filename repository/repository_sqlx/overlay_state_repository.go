package repository_sqlx

import (
	"database/sql"

	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
	"github.com/jmoiron/sqlx"
)

type OverlayStateRepository struct {
	DB *sqlx.DB
}

func (r *OverlayStateRepository) GetOverlayState(channelId int64) (*entity.OverlayState, error) {
	var s entity.OverlayState
	err := r.DB.Get(&s, `SELECT * FROM overlay_state WHERE channel_id = $1`, channelId)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *OverlayStateRepository) UpsertOverlayState(channelId int64, state []byte) (*entity.OverlayState, error) {
	var s entity.OverlayState
	err := r.DB.Get(&s, `
		INSERT INTO overlay_state (channel_id, seq, state, updated_at)
		VALUES ($1, 1, $2, now())
		ON CONFLICT (channel_id) DO UPDATE SET
			state = EXCLUDED.state,
			seq = overlay_state.seq + 1,
			updated_at = now()
		RETURNING *`, channelId, string(state))
	if err != nil {
		return nil, err
	}
	return &s, nil
}
