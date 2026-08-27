package repository

import "github.com/SaloEater/WhatNot-Webhook-Holder/entity"

type LayoutConfigRepositorier interface {
	GetLayoutConfig(channelId int64) (*entity.LayoutConfig, error)
	UpsertLayoutConfig(channelId int64, config []byte) error
}
