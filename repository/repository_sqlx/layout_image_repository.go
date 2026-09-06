package repository_sqlx

import (
	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
	"github.com/jmoiron/sqlx"
)

type LayoutImageRepository struct {
	DB *sqlx.DB
}

func (r *LayoutImageRepository) Insert(img *entity.LayoutImage) (*entity.LayoutImage, error) {
	var i entity.LayoutImage
	err := r.DB.Get(&i, `
		INSERT INTO layout_image (channel_id, name, url, width, height, size_bytes)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING *`, img.ChannelId, img.Name, img.Url, img.Width, img.Height, img.SizeBytes)
	if err != nil {
		return nil, err
	}
	return &i, nil
}

func (r *LayoutImageRepository) ListByChannel(channelId int64) ([]entity.LayoutImage, error) {
	images := []entity.LayoutImage{}
	err := r.DB.Select(&images, `
		SELECT * FROM layout_image
		WHERE channel_id = $1
		ORDER BY created_at DESC, id DESC`, channelId)
	if err != nil {
		return nil, err
	}
	return images, nil
}

func (r *LayoutImageRepository) Delete(id int64) error {
	_, err := r.DB.Exec(`DELETE FROM layout_image WHERE id = $1`, id)
	return err
}
