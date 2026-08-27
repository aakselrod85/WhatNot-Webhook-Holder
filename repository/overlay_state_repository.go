package repository

import "github.com/SaloEater/WhatNot-Webhook-Holder/entity"

type OverlayStateRepositorier interface {
	GetOverlayState(channelId int64) (*entity.OverlayState, error)
	UpsertOverlayState(channelId int64, state []byte) (*entity.OverlayState, error)
}
