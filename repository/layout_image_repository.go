package repository

import "github.com/SaloEater/WhatNot-Webhook-Holder/entity"

type LayoutImageRepositorier interface {
	Insert(img *entity.LayoutImage) (*entity.LayoutImage, error)
	ListByChannel(channelId int64) ([]entity.LayoutImage, error)
	Delete(id int64) error
}
