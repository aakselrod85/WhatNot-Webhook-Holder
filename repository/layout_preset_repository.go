package repository

import "github.com/SaloEater/WhatNot-Webhook-Holder/entity"

type LayoutPresetRepositorier interface {
	ListLayoutPresets(channelId int64) ([]entity.LayoutPreset, error)
	GetLayoutPreset(id int64) (*entity.LayoutPreset, error)
	CountLayoutPresets(channelId int64) (int, error)
	CreateLayoutPreset(channelId int64, name string, config []byte) (*entity.LayoutPreset, error)
	UpdateLayoutPreset(id int64, name *string, config []byte) error
	DeleteLayoutPreset(id int64) error
}
